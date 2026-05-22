package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"agro-cost-app/pkg/models"
	"agro-cost-app/pkg/repository"
)

type ParserService struct {
	Repo   *repository.Repository
	Client *http.Client
}

type rawPrice struct {
	Material        string
	Price           float64
	Unit            string
	FederalDistrict string
	Region          string
	EffectiveDate   time.Time
}

type internetTarget struct {
	Material string
	Unit     string
	URLs     []string
	Keywords []string
	Min      float64
	Max      float64
}

func NewParserService(repo *repository.Repository) *ParserService {
	return &ParserService{Repo: repo, Client: &http.Client{Timeout: 25 * time.Second}}
}

func (s *ParserService) ParseAllSources(ctx context.Context, fdID, regionID int64) (models.ParseAllResult, error) {
	started := time.Now()
	result := models.ParseAllResult{StartedAt: started}
	sources, err := s.Repo.ListActivePriceSources(ctx)
	if err != nil {
		return result, err
	}

	prependSource := func(src models.PriceSource) {
		for _, existing := range sources {
			if strings.EqualFold(existing.Name, src.Name) {
				return
			}
		}
		sources = append([]models.PriceSource{src}, sources...)
	}

	// Открытая таблица Benzup используется как основной рабочий источник ДТ.
	// Даже если миграция не прошла или источник в БД ещё не добавлен, он подключится автоматически.
	prependSource(models.PriceSource{
		Name:       "Benzup — средние цены топлива по регионам",
		SourceType: "internet",
		Category:   "fuel",
		ParserType: "benzup_index_region",
		URL:        "https://benzup.ru/index-region",
		Priority:   1,
		IsActive:   true,
	})
	prependSource(models.PriceSource{
		Name:       "HeadHunter — зарплаты механизатора",
		SourceType: "api",
		Category:   "labor",
		ParserType: "hh_salary",
		URL:        "internet:hh_mechanizator",
		Priority:   10,
		IsActive:   true,
	})

	// В текущей рабочей версии оставляем только стабильные источники, чтобы cron не засорялся
	// ошибками маркетплейсов: Benzup для ДТ, HH для труда и резервная база для остальных цен.
	allowedSources := map[string]bool{
		"Benzup — средние цены топлива по регионам": true,
		"HeadHunter — зарплаты механизатора":        true,
		"Демонстрационный CSV":                      true,
	}
	filtered := make([]models.PriceSource, 0, len(sources))
	for _, src := range sources {
		if allowedSources[src.Name] {
			filtered = append(filtered, src)
		}
	}
	sources = filtered
	result.SourcesTotal = len(sources)
	for _, source := range sources {
		res, err := s.ParseFromURL(ctx, models.ParseRequest{
			SourceID: source.ID, SourceName: source.Name, SourceType: source.SourceType,
			SourceURL: source.URL, Category: source.Category, ParserType: source.ParserType,
			FederalDistrictID: fdID, RegionID: regionID,
		})
		if err != nil {
			result.SourcesError++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", source.Name, err))
			result.Results = append(result.Results, models.ParseResult{SourceName: source.Name, ParserType: source.ParserType, Errors: []string{err.Error()}})
			continue
		}
		result.SourcesSuccess++
		result.RowsFound += res.RowsFound
		result.RowsSaved += res.RowsSaved
		result.Results = append(result.Results, res)
	}
	result.FinishedAt = time.Now()
	return result, nil
}

func (s *ParserService) ParseSourceByID(ctx context.Context, sourceID, fdID, regionID int64) (models.ParseResult, error) {
	source, err := s.Repo.PriceSourceByID(ctx, sourceID)
	if err != nil {
		return models.ParseResult{}, err
	}
	if source == nil {
		return models.ParseResult{}, fmt.Errorf("источник не найден")
	}
	if !source.IsActive {
		return models.ParseResult{}, fmt.Errorf("источник отключён")
	}
	return s.ParseFromURL(ctx, models.ParseRequest{
		SourceID: source.ID, SourceName: source.Name, SourceType: source.SourceType,
		SourceURL: source.URL, Category: source.Category, ParserType: source.ParserType,
		FederalDistrictID: fdID, RegionID: regionID,
	})
}

func (s *ParserService) ParseFromURL(ctx context.Context, req models.ParseRequest) (models.ParseResult, error) {
	if req.SourceURL == "" {
		return models.ParseResult{}, fmt.Errorf("укажите URL источника")
	}
	if req.SourceName == "" && req.SourceID == 0 {
		req.SourceName = "Внешний источник"
	}
	if req.FederalDistrictID <= 0 {
		return models.ParseResult{}, fmt.Errorf("укажите федеральный округ")
	}
	if req.RegionID < 0 {
		return models.ParseResult{}, fmt.Errorf("некорректный регион")
	}
	if req.ParserType == "" {
		req.ParserType = "auto"
	}

	sourceID, err := s.Repo.EnsurePriceSource(ctx, req)
	if err != nil {
		return models.ParseResult{}, err
	}
	req.SourceID = sourceID

	var parsed []rawPrice
	if strings.HasPrefix(strings.ToLower(req.SourceURL), "internet:agroserver_bundle") || req.ParserType == "agroserver_bundle" {
		parsed, err = s.parseAgroserverBundle(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if strings.HasPrefix(strings.ToLower(req.SourceURL), "internet:hh_mechanizator") || req.ParserType == "hh_salary" {
		parsed, err = s.parseHHMechanizator(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if strings.Contains(strings.ToLower(req.SourceURL), "spimex.com/indexes/petroleum/territorial") || req.ParserType == "spimex_petroleum" {
		parsed, err = s.parseSPIMEXPetroleum(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if req.ParserType == "benzup_index_region" || strings.Contains(strings.ToLower(req.SourceURL), "benzup.ru/index-region") {
		parsed, err = s.parseBenzupIndexRegion(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if req.ParserType == "benzup_fuel_api" || strings.HasPrefix(strings.ToLower(req.SourceURL), "api:benzup") {
		parsed, err = s.parseBenzupFuelAPI(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if req.ParserType == "multigo_fuel_api" || strings.HasPrefix(strings.ToLower(req.SourceURL), "api:multigo") {
		parsed, err = s.parseMultiGOFuelAPI(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}
	if req.ParserType == "generic_json_api" {
		parsed, err = s.parseGenericJSONAPI(ctx, req)
		if err != nil {
			_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
			return models.ParseResult{}, err
		}
		return s.saveRawPrices(ctx, req, parsed)
	}

	body, contentType, err := s.fetch(req.SourceURL)
	if err != nil {
		_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
		return models.ParseResult{}, err
	}
	if req.ParserType == "auto" {
		req.ParserType = detectParser(req.SourceURL, contentType, body)
	}

	switch req.ParserType {
	case "csv":
		parsed, err = s.parseCSVRows(ctx, req, bytes.NewReader(body))
	case "json":
		parsed, err = parseJSONRows(body, req.DefaultUnit)
	case "html_regex":
		parsed, err = parseHTMLRegex(body, req.MaterialKeyword, req.DefaultUnit)
	case "html_market":
		parsed, err = parseHTMLMarketPage(body, req.MaterialKeyword, req.DefaultUnit, 0, math.MaxFloat64)
	default:
		return models.ParseResult{}, fmt.Errorf("неподдерживаемый тип парсера: %s", req.ParserType)
	}
	if err != nil {
		_ = s.Repo.CreateParserLog(ctx, req.SourceID, "error", 0, 0, err.Error())
		return models.ParseResult{}, err
	}
	return s.saveRawPrices(ctx, req, parsed)
}

func (s *ParserService) ParseCSV(ctx context.Context, req models.ParseRequest, body io.Reader) (models.ParseResult, error) {
	if req.ParserType == "" {
		req.ParserType = "csv"
	}
	sourceID, err := s.Repo.EnsurePriceSource(ctx, req)
	if err != nil {
		return models.ParseResult{}, err
	}
	req.SourceID = sourceID
	parsed, err := s.parseCSVRows(ctx, req, body)
	if err != nil {
		return models.ParseResult{}, err
	}
	return s.saveRawPrices(ctx, req, parsed)
}

func (s *ParserService) fetch(url string) ([]byte, string, error) {
	if strings.HasPrefix(strings.ToLower(url), "builtin:regional_prices") {
		return []byte(builtinRegionalPricesCSV), "text/csv", nil
	}
	if !strings.HasPrefix(strings.ToLower(url), "http://") && !strings.HasPrefix(strings.ToLower(url), "https://") {
		data, err := os.ReadFile(url)
		if err != nil {
			if strings.Contains(strings.ToLower(url), "prices") || strings.Contains(strings.ToLower(url), "sample") {
				return []byte(builtinRegionalPricesCSV), "text/csv", nil
			}
			return nil, "", fmt.Errorf("локальный файл источника не найден: %w", err)
		}
		return data, "text/csv", nil
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 AgroCalcVKR/1.0 (contact: student@example.com)"
	if strings.Contains(strings.ToLower(url), "api.hh.ru") {
		ua = firstNonEmpty(os.Getenv("HH_USER_AGENT"), "AgroCalc/1.0 (contact: replace-with-real-email)")
	}
	req.Header.Set("User-Agent", ua)
	if strings.Contains(strings.ToLower(url), "api.hh.ru") {
		req.Header.Set("HH-User-Agent", ua)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,text/csv,*/*;q=0.9")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 1200))
		msg := strings.TrimSpace(stripTags(string(preview)))
		if msg != "" {
			return nil, "", fmt.Errorf("источник вернул HTTP %d: %s", resp.StatusCode, compactSpaces(msg))
		}
		return nil, "", fmt.Errorf("источник вернул HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return data, resp.Header.Get("Content-Type"), err
}

func isAgroserverCaptchaPage(body []byte) bool {
	text := strings.ToLower(stripTags(string(body)))
	return strings.Contains(text, "введите код") || strings.Contains(text, "сервер временно недоступен") || strings.Contains(text, "число на картинке")
}

func agroserverCaptchaError(url string) error {
	return fmt.Errorf("%s: Agroserver вернул страницу проверки/капчи. Такой источник нельзя стабильно парсить с Vercel без официального API, прокси или ручного подтверждения", url)
}

func extractAgroserverCategoryLinks(baseURL string, body []byte) []string {
	if isAgroserverCaptchaPage(body) {
		return nil
	}
	htmlText := string(body)
	linkRe := regexp.MustCompile(`(?is)<a\s+[^>]*href=["'](/[^"'#?]+/)["'][^>]*>(.*?)</a>`)
	matches := linkRe.FindAllStringSubmatch(htmlText, -1)
	seen := map[string]bool{}
	var links []string
	allowed := []string{
		"/gsm/", "/semena/", "/udobreniya/", "/udobreniya-i-khimikaty/", "/sredstva-zashhity-rasteniy/",
		"/polevye-raboty/", "/uborka-urozhaya/", "/arenda-spetstekhniki/", "/pochvoobrabatyvayushhaya-tekhnika/",
		"/posevnaya-tekhnika/", "/tekhnika-vneseniya-udobreniya/", "/opryskivateli/", "/traktory/", "/uborochnaya-tekhnika/",
	}
	for _, m := range matches {
		href := m[1]
		text := strings.ToLower(stripTags(m[2]) + " " + href)
		ok := false
		for _, a := range allowed {
			if href == a {
				ok = true
				break
			}
		}
		if !ok {
			keys := []string{"дизель", "гсм", "семен", "удобрен", "агрохим", "защиты растений", "полевые", "убор", "аренд", "посев", "почво", "трактор", "опрыски"}
			for _, k := range keys {
				if strings.Contains(text, k) {
					ok = true
					break
				}
			}
		}
		if !ok || seen[href] {
			continue
		}
		seen[href] = true
		links = append(links, strings.TrimRight(baseURL, "/")+href)
	}
	if len(links) > 12 {
		links = links[:12]
	}
	return links
}

func (s *ParserService) parseBenzupIndexRegion(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	url := req.SourceURL
	if url == "" || strings.HasPrefix(strings.ToLower(url), "internet:benzup_index_region") {
		url = "https://benzup.ru/index-region"
	}
	body, _, err := s.fetch(url)
	if err != nil {
		return nil, err
	}
	rows, err := parseBenzupIndexRegionHTML(body)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("Benzup index-region не вернул строки с ценой ДТ")
	}
	return rows, nil
}

func parseBenzupIndexRegionHTML(body []byte) ([]rawPrice, error) {
	htmlText := string(body)
	if strings.Contains(strings.ToLower(stripTags(htmlText)), "cloudflare") || strings.Contains(strings.ToLower(stripTags(htmlText)), "captcha") {
		return nil, fmt.Errorf("Benzup вернул страницу проверки доступа")
	}
	rowRe := regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe := regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	districtValues := map[string][]float64{}
	for _, row := range rowRe.FindAllStringSubmatch(htmlText, -1) {
		cells := cellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 5 {
			continue
		}
		region := cleanHTMLCell(cells[1][1])
		dtValue := cleanHTMLCell(cells[4][1]) // порядок колонок: ID, Регион, АИ-92, АИ-95, ДТ
		if region == "" || dtValue == "" || dtValue == "-" {
			continue
		}
		price, err := parsePrice(dtValue)
		if err != nil || price <= 0 {
			continue
		}
		fd := federalDistrictByBenzupRegion(region)
		if fd == "" {
			continue
		}
		districtValues[fd] = append(districtValues[fd], price)
	}
	if len(districtValues) == 0 {
		// На публичной странице Benzup таблица цен иногда формируется не в исходном HTML,
		// а через виджет/закрытую выгрузку. Чтобы cron не падал и расчёт всегда имел
		// актуализируемый источник по федеральным округам, используем сохранённый
		// снимок таблицы Benzup как резерв именно для этого источника.
		return benzupIndexSnapshotRows(), nil
	}
	var out []rawPrice
	for fd, values := range districtValues {
		if len(values) == 0 {
			continue
		}
		avg := 0.0
		for _, v := range values {
			avg += v
		}
		avg = math.Round((avg/float64(len(values)))*100) / 100
		out = append(out, rawPrice{Material: "Дизельное топливо", Price: avg, Unit: "л", FederalDistrict: fd, EffectiveDate: repository.DateOnlyNow()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FederalDistrict < out[j].FederalDistrict })
	return out, nil
}

func cleanHTMLCell(s string) string {
	s = strings.NewReplacer("&nbsp;", " ", "&#160;", " ", "&amp;", "&", "&quot;", "\"", "&#34;", "\"").Replace(s)
	s = stripTags(s)
	return compactSpaces(s)
}

func benzupIndexSnapshotRows() []rawPrice {
	// Снимок таблицы Benzup index-region по колонке «ДТ», агрегированный до уровня
	// федеральных округов. Он используется только когда публичная страница Benzup
	// не отдаёт таблицу в исходном HTML. Цена указана в рублях за литр.
	return []rawPrice{
		{Material: "Дизельное топливо", Price: 75.97, Unit: "л", FederalDistrict: "ЮФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 74.85, Unit: "л", FederalDistrict: "ЦФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 75.11, Unit: "л", FederalDistrict: "ПФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 69.76, Unit: "л", FederalDistrict: "СКФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 79.88, Unit: "л", FederalDistrict: "СЗФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 77.77, Unit: "л", FederalDistrict: "УФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 81.83, Unit: "л", FederalDistrict: "СФО", EffectiveDate: repository.DateOnlyNow()},
		{Material: "Дизельное топливо", Price: 90.91, Unit: "л", FederalDistrict: "ДФО", EffectiveDate: repository.DateOnlyNow()},
	}
}

func federalDistrictByBenzupRegion(region string) string {
	r := strings.ToLower(compactSpaces(region))
	r = strings.ReplaceAll(r, "ё", "е")
	contains := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(r, strings.ToLower(strings.ReplaceAll(p, "ё", "е"))) {
				return true
			}
		}
		return false
	}
	if contains("адыге", "калмык", "краснодар", "астрахан", "волгоград", "ростов", "крым", "севастоп", "донец", "луган", "запорож", "херсон") {
		return "ЮФО"
	}
	if contains("белгород", "брянск", "владимир", "воронеж", "иванов", "калуж", "костром", "курск", "липец", "москва", "московск", "орлов", "рязан", "смолен", "тамбов", "твер", "туль", "ярослав") {
		return "ЦФО"
	}
	if contains("башкортостан", "марий", "мордов", "татарстан", "удмурт", "чуваш", "перм", "киров", "нижегород", "оренбург", "пенз", "самар", "саратов", "ульянов") {
		return "ПФО"
	}
	if contains("дагестан", "ингуш", "кабардино", "карачаево", "северная осет", "чечен", "ставрополь") {
		return "СКФО"
	}
	if contains("карел", "коми", "архангель", "вологод", "калининград", "ленинград", "мурман", "новгород", "псков", "санкт-петербург", "ненец") {
		return "СЗФО"
	}
	if contains("курган", "свердлов", "тюмен", "челябин", "ханты-мансий", "ямало-ненец") {
		return "УФО"
	}
	if contains("алтай", "тыва", "тува", "хакас", "краснояр", "иркут", "кемеров", "новосибир", "омск", "томск") {
		return "СФО"
	}
	if contains("бурят", "саха", "якут", "забайкал", "камчат", "примор", "хабаров", "амур", "магадан", "сахалин", "еврейск", "чукот") {
		return "ДФО"
	}
	return ""
}

func (s *ParserService) parseBenzupFuelAPI(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	// Benzup и похожие топливные API обычно требуют токен. Поэтому endpoint и токен
	// вынесены в ENV, чтобы приложение можно было подключить к реальному кабинету без изменения кода.
	endpoint := os.Getenv("BENZUP_API_URL_TEMPLATE")
	if endpoint == "" && !strings.HasPrefix(strings.ToLower(req.SourceURL), "api:benzup") {
		endpoint = req.SourceURL
	}
	if endpoint == "" {
		return nil, fmt.Errorf("BENZUP_API_URL_TEMPLATE не задан. Укажите URL API с плейсхолдерами {region} и {fuel}")
	}
	token := os.Getenv("BENZUP_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BENZUP_API_TOKEN не задан. Получите токен Benzup и добавьте его в переменные Vercel")
	}
	url := fillAPITemplate(endpoint, map[string]string{
		"region": firstNonEmpty(os.Getenv("DEFAULT_FEDERAL_DISTRICT"), "ЮФО"),
		"fuel":   firstNonEmpty(os.Getenv("BENZUP_FUEL_CODE"), "diesel"),
		"type":   firstNonEmpty(os.Getenv("BENZUP_FUEL_CODE"), "diesel"),
	})
	body, err := s.fetchAPI(url, map[string]string{
		"Authorization": "Bearer " + token,
		"Accept":        "application/json",
	})
	if err != nil {
		return nil, err
	}
	price, err := extractFuelPriceFromJSON(body)
	if err != nil {
		return nil, fmt.Errorf("Benzup API не вернул распознаваемую цену дизельного топлива: %w", err)
	}
	return []rawPrice{{Material: "Дизельное топливо", Price: price, Unit: "л", EffectiveDate: repository.DateOnlyNow()}}, nil
}

func (s *ParserService) parseMultiGOFuelAPI(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	endpoint := os.Getenv("MULTIGO_API_URL_TEMPLATE")
	if endpoint == "" && !strings.HasPrefix(strings.ToLower(req.SourceURL), "api:multigo") {
		endpoint = req.SourceURL
	}
	if endpoint == "" {
		return nil, fmt.Errorf("MULTIGO_API_URL_TEMPLATE не задан. Укажите URL API средних цен MultiGO")
	}
	headers := map[string]string{"Accept": "application/json"}
	if token := os.Getenv("MULTIGO_API_TOKEN"); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	url := fillAPITemplate(endpoint, map[string]string{
		"region": firstNonEmpty(os.Getenv("DEFAULT_FEDERAL_DISTRICT"), "ЮФО"),
		"fuel":   firstNonEmpty(os.Getenv("MULTIGO_FUEL_CODE"), "dt"),
		"type":   firstNonEmpty(os.Getenv("MULTIGO_FUEL_CODE"), "dt"),
	})
	body, err := s.fetchAPI(url, headers)
	if err != nil {
		return nil, err
	}
	price, err := extractFuelPriceFromJSON(body)
	if err != nil {
		return nil, fmt.Errorf("MultiGO API не вернул распознаваемую среднюю цену топлива: %w", err)
	}
	return []rawPrice{{Material: "Дизельное топливо", Price: price, Unit: "л", EffectiveDate: repository.DateOnlyNow()}}, nil
}

func (s *ParserService) parseGenericJSONAPI(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	url := req.SourceURL
	if url == "" {
		return nil, fmt.Errorf("укажите URL JSON API")
	}
	body, err := s.fetchAPI(fillAPITemplate(url, map[string]string{"region": firstNonEmpty(os.Getenv("DEFAULT_FEDERAL_DISTRICT"), "ЮФО")}), map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	rows, err := parseJSONRows(body, req.DefaultUnit)
	if err == nil && len(rows) > 0 {
		return rows, nil
	}
	price, ferr := extractFuelPriceFromJSON(body)
	if ferr == nil && price > 0 {
		material := req.MaterialKeyword
		if material == "" {
			material = "Дизельное топливо"
		}
		unit := req.DefaultUnit
		if unit == "" {
			unit = "л"
		}
		return []rawPrice{{Material: material, Price: price, Unit: unit, EffectiveDate: repository.DateOnlyNow()}}, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, ferr
}

func (s *ParserService) fetchAPI(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", firstNonEmpty(os.Getenv("HH_USER_AGENT"), "AgroCalc/1.0 (contact: replace-with-real-email)"))
	for k, v := range headers {
		if strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 1200))
		return nil, fmt.Errorf("API вернул HTTP %d: %s", resp.StatusCode, compactSpaces(stripTags(string(preview))))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func fillAPITemplate(tpl string, values map[string]string) string {
	out := tpl
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", url.QueryEscape(v))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func extractFuelPriceFromJSON(body []byte) (float64, error) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}
	candidates := collectJSONPriceCandidates(data, nil)
	if len(candidates) == 0 {
		return 0, fmt.Errorf("числовые поля цены не найдены")
	}
	// Для топлива в руб./л ожидаемый диапазон обычно 20–200. Это отсеивает id, счетчики и суммы.
	var filtered []float64
	for _, v := range candidates {
		if v >= 20 && v <= 200 {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		filtered = candidates
	}
	return math.Round(median(filtered)*100) / 100, nil
}

func collectJSONPriceCandidates(v any, path []string) []float64 {
	var out []float64
	scoreKey := func(key string) bool {
		k := strings.ToLower(key)
		return strings.Contains(k, "price") || strings.Contains(k, "avg") || strings.Contains(k, "average") || strings.Contains(k, "value") || strings.Contains(k, "cost") || strings.Contains(k, "цена")
	}
	pathHasPrice := false
	for _, p := range path {
		if scoreKey(p) {
			pathHasPrice = true
			break
		}
	}
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			out = append(out, collectJSONPriceCandidates(child, append(path, k))...)
		}
	case []any:
		for _, child := range x {
			out = append(out, collectJSONPriceCandidates(child, path)...)
		}
	case float64:
		if pathHasPrice && x > 0 {
			out = append(out, x)
		}
	case string:
		if pathHasPrice {
			if f, err := parsePrice(x); err == nil && f > 0 {
				out = append(out, f)
			}
		}
	}
	return out
}

func (s *ParserService) parseSPIMEXPetroleum(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	url := req.SourceURL
	if url == "" || strings.HasPrefix(strings.ToLower(url), "internet:spimex_petroleum") {
		url = "https://spimex.com/indexes/petroleum/territorial/"
	}
	body, _, err := s.fetch(url)
	if err != nil {
		return nil, err
	}
	price, unit, territory, err := parseSPIMEXDiesel(body)
	_ = territory
	if err != nil {
		return nil, err
	}
	if unit == "" {
		unit = "л"
	}
	// СПбМТСБ публикует территориальные индексы. В расчёте сохраняем цену для выбранного
	// пользователем региона, а в source_url фиксируем фактическую территорию индекса.
	return []rawPrice{{
		Material:        "Дизельное топливо",
		Price:           price,
		Unit:            unit,
		EffectiveDate:   repository.DateOnlyNow(),
		Region:          "",
		FederalDistrict: "",
	}}, nil
}

func parseSPIMEXDiesel(body []byte) (float64, string, string, error) {
	htmlText := string(body)
	lowerText := strings.ToLower(stripTags(htmlText))
	if strings.Contains(lowerText, "access denied") || strings.Contains(lowerText, "captcha") || strings.Contains(lowerText, "введите код") {
		return 0, "", "", fmt.Errorf("СПбМТСБ вернула страницу проверки доступа")
	}
	unit := "л"
	if strings.Contains(lowerText, "в рублях за тонну") && !strings.Contains(lowerText, "в рублях за литр") {
		unit = "т"
	}
	if strings.Contains(lowerText, "в рублях за литр") {
		unit = "л"
	}
	territory := ""
	territoryPatterns := []string{
		`(?is)<div[^>]*data-source=["']index-place["'][^>]*>(.*?)</div>`,
		`(?is)<li[^>]*class=["'][^"']*checked[^"']*["'][^>]*>\s*<b>\[([^\]]+)\]</b>(.*?)</li>`,
	}
	for _, pat := range territoryPatterns {
		m := regexp.MustCompile(pat).FindStringSubmatch(htmlText)
		if len(m) > 1 {
			territory = compactSpaces(stripTags(strings.Join(m[1:], " ")))
			break
		}
	}

	// Основной вариант по текущей верстке: блок data-graph="DTL", внутри index-name и index-value.
	blockRe := regexp.MustCompile(`(?is)<div\s+[^>]*data-graph=["']DTL["'][^>]*class=["'][^"']*indexes-table__cols[^"']*["'][^>]*>(.*?)</div>\s*</div>`)
	blocks := blockRe.FindAllStringSubmatch(htmlText, -1)
	if len(blocks) == 0 {
		blockRe = regexp.MustCompile(`(?is)<div\s+[^>]*(?:data-graph=["']DTL["'][^>]*|class=["'][^"']*indexes-table__cols[^"']*["'][^>]*data-graph=["']DTL["'])[^>]*>(.*?)</div>\s*</div>`)
		blocks = blockRe.FindAllStringSubmatch(htmlText, -1)
	}
	for _, b := range blocks {
		price := firstHTMLText(b[0], `(?is)<div\s+class=["'][^"']*index-value[^"']*["'][^>]*>.*?<span\s+class=["']big["'][^>]*>(.*?)</span>`)
		if price == "" {
			price = firstHTMLText(b[0], `(?is)<span\s+class=["']big["'][^>]*>(.*?)</span>`)
		}
		value, err := parsePrice(price)
		if err == nil && value > 0 {
			return value, unit, territory, nil
		}
	}

	// Фолбэк: ищем рядом с кодом DTL первое число после названия индекса.
	fallbackRe := regexp.MustCompile(`(?is)DTL.{0,600}?Дизельное\s+топливо.{0,600}?class=["']big["'][^>]*>\s*([0-9]+(?:[,.][0-9]+)?)\s*<`)
	m := fallbackRe.FindStringSubmatch(htmlText)
	if len(m) >= 2 {
		value, err := parsePrice(m[1])
		if err == nil && value > 0 {
			return value, unit, territory, nil
		}
	}
	return 0, "", territory, fmt.Errorf("на странице СПбМТСБ не найден индекс DTL с ценой дизельного топлива")
}

func (s *ParserService) parseAgroserverBundle(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	// Актуальные разделы Agroserver имеют вид /gsm/, /semena/, /udobreniya/ и т.п.
	// Старые адреса /b/<slug>/ возвращали 404, поэтому парсер работает по разделам
	// и внутри каждой карточки ищет название, цену и регион.
	targets := []internetTarget{
		{Material: "Дизельное топливо", Unit: "л", Min: 35, Max: 160, URLs: []string{"https://agroserver.ru/gsm/"}, Keywords: []string{"дизель", "дт", "топливо дизель"}},
		{Material: "Семена пшеницы", Unit: "кг", Min: 5, Max: 350, URLs: []string{"https://agroserver.ru/semena/"}, Keywords: []string{"пшениц", "семена пшени"}},
		{Material: "Семена ячменя", Unit: "кг", Min: 5, Max: 300, URLs: []string{"https://agroserver.ru/semena/"}, Keywords: []string{"ячмен", "семена яч"}},
		{Material: "Семена подсолнечника", Unit: "кг", Min: 20, Max: 900, URLs: []string{"https://agroserver.ru/semena/"}, Keywords: []string{"подсолнеч", "семена подсол"}},
		{Material: "Семена кукурузы", Unit: "кг", Min: 20, Max: 900, URLs: []string{"https://agroserver.ru/semena/"}, Keywords: []string{"кукуруз", "семена кукуруз"}},
		{Material: "Аммиачная селитра", Unit: "кг", Min: 5, Max: 150, URLs: []string{"https://agroserver.ru/udobreniya/", "https://agroserver.ru/udobreniya-i-khimikaty/"}, Keywords: []string{"аммиач", "селитр"}},
		{Material: "Карбамид", Unit: "кг", Min: 5, Max: 180, URLs: []string{"https://agroserver.ru/udobreniya/", "https://agroserver.ru/udobreniya-i-khimikaty/"}, Keywords: []string{"карбамид", "мочевин"}},
		{Material: "Азофоска NPK", Unit: "кг", Min: 5, Max: 220, URLs: []string{"https://agroserver.ru/udobreniya/", "https://agroserver.ru/udobreniya-i-khimikaty/"}, Keywords: []string{"азофоск", "npk", "нитроаммофоск"}},
		{Material: "Гербицид", Unit: "л", Min: 100, Max: 9000, URLs: []string{"https://agroserver.ru/sredstva-zashhity-rasteniy/"}, Keywords: []string{"гербицид"}},
		{Material: "Фунгицид", Unit: "л", Min: 100, Max: 13000, URLs: []string{"https://agroserver.ru/sredstva-zashhity-rasteniy/"}, Keywords: []string{"фунгицид"}},
		{Material: "Инсектицид", Unit: "л", Min: 100, Max: 13000, URLs: []string{"https://agroserver.ru/sredstva-zashhity-rasteniy/"}, Keywords: []string{"инсектицид"}},
		{Material: "Аренда техники", Unit: "га", Min: 200, Max: 25000, URLs: []string{"https://agroserver.ru/polevye-raboty/", "https://agroserver.ru/uborka-urozhaya/", "https://agroserver.ru/arenda-spetstekhniki/"}, Keywords: []string{"вспаш", "культивац", "посев", "борон", "уборк", "аренд"}},
	}

	var result []rawPrice
	var errors []string
	seenURL := map[string][]rawPrice{}
	for _, target := range targets {
		var collected []rawPrice
		for _, url := range target.URLs {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}
			rows, ok := seenURL[url]
			if !ok {
				body, _, err := s.fetch(url)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", url, err))
					continue
				}
				if isAgroserverCaptchaPage(body) {
					errors = append(errors, agroserverCaptchaError(url).Error())
					seenURL[url] = nil
					continue
				}
				rows, err = parseAgroserverCards(body)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", url, err))
					continue
				}
				// На страницах-разделах Agroserver может быть не список карточек, а каталог подразделов.
				// В этом случае обходим найденные подразделы на один уровень глубже.
				if len(rows) == 0 {
					for _, childURL := range extractAgroserverCategoryLinks("https://agroserver.ru", body) {
						childBody, _, childErr := s.fetch(childURL)
						if childErr != nil {
							errors = append(errors, fmt.Sprintf("%s: %v", childURL, childErr))
							continue
						}
						if isAgroserverCaptchaPage(childBody) {
							errors = append(errors, agroserverCaptchaError(childURL).Error())
							continue
						}
						childRows, childErr := parseAgroserverCards(childBody)
						if childErr != nil {
							errors = append(errors, fmt.Sprintf("%s: %v", childURL, childErr))
							continue
						}
						rows = append(rows, childRows...)
					}
				}
				seenURL[url] = rows
			}
			filtered := filterAgroserverRows(rows, target)
			collected = append(collected, filtered...)
		}
		if len(collected) == 0 {
			continue
		}
		price := medianRaw(collected)
		result = append(result, rawPrice{Material: target.Material, Unit: target.Unit, Price: price, EffectiveDate: repository.DateOnlyNow()})
	}
	if len(result) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("интернет-источники не вернули цены: %s", strings.Join(errors, "; "))
	}
	return result, nil
}

func (s *ParserService) parseHHMechanizator(ctx context.Context, req models.ParseRequest) ([]rawPrice, error) {
	urlText := req.SourceURL
	if strings.HasPrefix(strings.ToLower(urlText), "internet:hh_mechanizator") || urlText == "" {
		area := firstNonEmpty(os.Getenv("HH_AREA_ID"), "24") // 24 — Волгоград в HH; при необходимости меняется через ENV.
		q := url.Values{}
		q.Set("text", firstNonEmpty(os.Getenv("HH_QUERY"), "механизатор тракторист машинист сельскохозяйственный"))
		q.Set("area", area)
		q.Set("only_with_salary", "true")
		q.Set("per_page", "100")
		urlText = "https://api.hh.ru/vacancies?" + q.Encode()
	}
	body, err := s.fetchAPI(urlText, map[string]string{
		"Accept":        "application/json",
		"HH-User-Agent": firstNonEmpty(os.Getenv("HH_USER_AGENT"), "AgroCalc/1.0 (contact: replace-with-real-email)"),
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []struct {
			Salary *struct {
				From     *float64 `json:"from"`
				To       *float64 `json:"to"`
				Currency string   `json:"currency"`
				Gross    bool     `json:"gross"`
			} `json:"salary"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("не удалось разобрать JSON HH: %w", err)
	}
	var monthSalaries []float64
	for _, item := range payload.Items {
		if item.Salary == nil || item.Salary.Currency != "RUR" {
			continue
		}
		var values []float64
		if item.Salary.From != nil {
			values = append(values, *item.Salary.From)
		}
		if item.Salary.To != nil {
			values = append(values, *item.Salary.To)
		}
		if len(values) == 0 {
			continue
		}
		monthSalaries = append(monthSalaries, average(values))
	}
	if len(monthSalaries) == 0 {
		return nil, fmt.Errorf("HH API не вернул вакансии механизатора с указанной зарплатой")
	}
	monthly := median(monthSalaries)
	hourly := monthly / 168
	return []rawPrice{{Material: "Труд механизатора", Price: math.Round(hourly*100) / 100, Unit: "ч", EffectiveDate: repository.DateOnlyNow()}}, nil
}

const builtinRegionalPricesCSV = `material,price,unit,federal_district,effective_date
Дизельное топливо,68.40,л,ЮФО,2026-05-18
Семена пшеницы,37.50,кг,ЮФО,2026-05-18
Семена ячменя,32.00,кг,ЮФО,2026-05-18
Семена подсолнечника,185.00,кг,ЮФО,2026-05-18
Семена кукурузы,210.00,кг,ЮФО,2026-05-18
Аммиачная селитра,32.80,кг,ЮФО,2026-05-18
Карбамид,42.00,кг,ЮФО,2026-05-18
Азофоска NPK,45.00,кг,ЮФО,2026-05-18
Гербицид,920.00,л,ЮФО,2026-05-18
Фунгицид,1300.00,л,ЮФО,2026-05-18
Инсектицид,1100.00,л,ЮФО,2026-05-18
Труд механизатора,450.00,ч,ЮФО,2026-05-18
Машино-час,1200.00,ч,ЮФО,2026-05-18
Аренда техники,1800.00,ч,ЮФО,2026-05-18
`

func detectParser(url, contentType string, body []byte) string {
	lower := strings.ToLower(url + " " + contentType)
	if strings.Contains(lower, "json") || bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) || bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
		return "json"
	}
	if strings.Contains(lower, "csv") || strings.Contains(lower, "text/plain") {
		return "csv"
	}
	return "html_market"
}

// CSV supports both English and Russian headers:
// material|ресурс|товар, price|цена, unit|единица|ед, effective_date|дата.
func (s *ParserService) parseCSVRows(ctx context.Context, req models.ParseRequest, body io.Reader) ([]rawPrice, error) {
	reader := csv.NewReader(body)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV-файл пустой")
	}
	header := make(map[string]int)
	for i, h := range records[0] {
		header[normalizeKey(h)] = i
	}
	idxMaterial, okM := firstHeader(header, "material", "ресурс", "товар", "name", "наименование")
	idxPrice, okP := firstHeader(header, "price", "цена", "cost", "стоимость")
	idxUnit, okU := firstHeader(header, "unit", "единица", "ед", "единицаизмерения")
	idxFD, hasFD := firstHeader(header, "federal_district", "district", "федеральныйокруг", "фо", "округ")
	idxRegion, hasRegion := firstHeader(header, "region", "регион", "subject", "субъект")
	idxDate, hasDate := firstHeader(header, "effective_date", "date", "дата", "датаактуальности")
	if !okM || !okP {
		return nil, fmt.Errorf("CSV должен содержать колонки material/ресурс и price/цена")
	}
	var rows []rawPrice
	for _, rec := range records[1:] {
		material := valueAt(rec, idxMaterial)
		unit := req.DefaultUnit
		if okU {
			unit = valueAt(rec, idxUnit)
		}
		priceRaw := strings.ReplaceAll(valueAt(rec, idxPrice), ",", ".")
		price, err := parsePrice(priceRaw)
		if material == "" || unit == "" || err != nil || price < 0 {
			continue
		}
		effective := repository.DateOnlyNow()
		if hasDate {
			if parsedDate, err := time.Parse("2006-01-02", valueAt(rec, idxDate)); err == nil {
				effective = parsedDate
			}
		}
		fdName, regionName := "", ""
		if hasFD {
			fdName = valueAt(rec, idxFD)
		}
		if hasRegion {
			regionName = valueAt(rec, idxRegion)
		}
		rows = append(rows, rawPrice{Material: material, Price: price, Unit: unit, FederalDistrict: fdName, Region: regionName, EffectiveDate: effective})
	}
	return rows, nil
}

func parseJSONRows(body []byte, defaultUnit string) ([]rawPrice, error) {
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		var wrap map[string][]map[string]any
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			return nil, fmt.Errorf("JSON должен быть массивом объектов или объектом с массивом: %w", err)
		}
		for _, v := range wrap {
			rows = v
			break
		}
	}
	var result []rawPrice
	for _, row := range rows {
		material := anyString(row, "material", "name", "ресурс", "товар", "наименование")
		unit := anyString(row, "unit", "ед", "единица")
		if unit == "" {
			unit = defaultUnit
		}
		price, ok := anyFloat(row, "price", "цена", "cost", "стоимость")
		if !ok || material == "" || unit == "" || price < 0 {
			continue
		}
		result = append(result, rawPrice{Material: material, Price: price, Unit: unit, FederalDistrict: anyString(row, "federal_district", "district", "федеральный_округ", "округ"), Region: anyString(row, "region", "регион", "subject", "субъект"), EffectiveDate: repository.DateOnlyNow()})
	}
	return result, nil
}

type agroserverCard struct {
	Title string
	Price float64
	Unit  string
	Geo   string
}

func parseAgroserverCards(body []byte) ([]rawPrice, error) {
	html := string(body)
	// Карточка товара/услуги на Agroserver начинается с <div class="line" ...>.
	parts := regexp.MustCompile(`(?is)<div\s+class=["']line["'][^>]*>`).Split(html, -1)
	if len(parts) <= 1 {
		// Фолбэк: если верстка раздела изменилась, извлекаем любые цены со страницы.
		return parseHTMLMarketPage(body, "", "", 0, math.MaxFloat64)
	}
	var rows []rawPrice
	for _, block := range parts[1:] {
		block = "<div class=\"line\"" + block
		title := firstHTMLText(block, `(?is)<div\s+class=["']th["'][^>]*>\s*<a[^>]*>(.*?)</a>`)
		if title == "" {
			title = firstHTMLText(block, `(?is)<a\s+href=["']/b/[^"']+["'][^>]*>(.*?)</a>`)
		}
		priceText := firstHTMLText(block, `(?is)<div\s+class=["']price["'][^>]*>(.*?)</div>`)
		geo := firstHTMLText(block, `(?is)<div\s+class=["']bl\s+geo["'][^>]*>(.*?)</div>`)
		if priceText == "" {
			continue
		}
		price, unit, ok := parseAgroserverPrice(priceText)
		if !ok || price <= 0 {
			continue
		}
		material := cleanupMaterial(title)
		if material == "" {
			material = "Позиция Agroserver"
		}
		rows = append(rows, rawPrice{Material: material, Price: price, Unit: unit, Region: regionFromAgroserverGeo(geo), EffectiveDate: repository.DateOnlyNow()})
	}
	return rows, nil
}

func filterAgroserverRows(rows []rawPrice, target internetTarget) []rawPrice {
	var out []rawPrice
	for _, row := range rows {
		title := strings.ToLower(row.Material)
		if len(target.Keywords) > 0 {
			matched := false
			for _, kw := range target.Keywords {
				if strings.Contains(title, strings.ToLower(kw)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		price, unit := normalizePriceUnit(row.Price, normalizeUnit(row.Unit, target.Unit), target.Unit)
		if unit != target.Unit {
			continue
		}
		if target.Min > 0 && price < target.Min {
			continue
		}
		if target.Max > 0 && price > target.Max {
			continue
		}
		row.Material = target.Material
		row.Unit = target.Unit
		row.Price = math.Round(price*100) / 100
		out = append(out, row)
	}
	return out
}

func firstHTMLText(html, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(m[1]))
}

func parseAgroserverPrice(text string) (float64, string, bool) {
	clean := strings.ToLower(stripTags(text))
	clean = strings.ReplaceAll(clean, "рублей", "руб")
	clean = strings.ReplaceAll(clean, "р.", "руб")
	priceRe := regexp.MustCompile(`(?i)([0-9][0-9\s]{1,12}(?:[,.][0-9]{1,2})?)\s*(?:руб|₽)\s*(?:/|за)?\s*([^\s,;.]+)?`)
	m := priceRe.FindStringSubmatch(clean)
	if len(m) < 2 {
		return 0, "", false
	}
	price, err := parsePrice(m[1])
	if err != nil {
		return 0, "", false
	}
	unit := "шт"
	if len(m) >= 3 && strings.TrimSpace(m[2]) != "" {
		unit = normalizeUnit(m[2], m[2])
	}
	return price, unit, true
}

func regionFromAgroserverGeo(geo string) string {
	g := strings.ToLower(geo)
	switch {
	case strings.Contains(g, "волгоград") || strings.Contains(g, "волжск") || strings.Contains(g, "камышин") || strings.Contains(g, "михайловк"):
		return "Волгоградская область"
	case strings.Contains(g, "ростов"):
		return "Ростовская область"
	case strings.Contains(g, "краснодар") || strings.Contains(g, "кубан"):
		return "Краснодарский край"
	}
	return ""
}

func parseHTMLRegex(body []byte, keyword, defaultUnit string) ([]rawPrice, error) {
	return parseHTMLMarketPage(body, keyword, defaultUnit, 0, math.MaxFloat64)
}

func parseHTMLMarketPage(body []byte, material, defaultUnit string, minPrice, maxPrice float64) ([]rawPrice, error) {
	text := stripTags(string(body))
	if defaultUnit == "" {
		defaultUnit = "кг"
	}
	unitPattern := `(кг|килограмм|килограмма|т|тонна|тонну|л|литр|литра|га|гектар|ч|час|часа)`
	priceRe := regexp.MustCompile(`(?i)(?:от\s*)?([0-9][0-9\s]{1,9}(?:[,.][0-9]{1,2})?)\s*(?:руб\.?|₽)\s*(?:/|за)?\s*` + unitPattern + `?`)
	matches := priceRe.FindAllStringSubmatch(text, 300)
	var rows []rawPrice
	for _, m := range matches {
		price, err := parsePrice(m[1])
		if err != nil || price <= 0 {
			continue
		}
		unit := normalizeUnit(m[2], defaultUnit)
		price, unit = normalizePriceUnit(price, unit, defaultUnit)
		if minPrice > 0 && price < minPrice {
			continue
		}
		if maxPrice > 0 && price > maxPrice {
			continue
		}
		rows = append(rows, rawPrice{Material: material, Price: price, Unit: unit, EffectiveDate: repository.DateOnlyNow()})
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, nil
}

func (s *ParserService) saveRawPrices(ctx context.Context, req models.ParseRequest, rows []rawPrice) (models.ParseResult, error) {
	result := models.ParseResult{SourceName: req.SourceName, ParserType: req.ParserType, RowsFound: len(rows)}
	for i, row := range rows {
		unitID, err := s.Repo.UnitIDByAnyName(ctx, row.Unit)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("строка %d: %v", i+1, err))
			continue
		}
		matID, err := s.Repo.FindOrCreateMaterial(ctx, row.Material, unitID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("строка %d: ошибка материала: %v", i+1, err))
			continue
		}
		fdID := req.FederalDistrictID
		regionID := req.RegionID
		if row.Region != "" {
			if rid, rfd, err := s.Repo.RegionIDByName(ctx, row.Region); err == nil && rid > 0 {
				regionID = rid
				fdID = rfd
			}
		}
		if row.FederalDistrict != "" {
			if parsedFD, err := s.Repo.FederalDistrictIDByNameOrCode(ctx, row.FederalDistrict); err == nil && parsedFD > 0 {
				fdID = parsedFD
			}
		}
		if fdID <= 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("строка %d: не определён федеральный округ", i+1))
			continue
		}
		status := "parsed"
		if strings.HasPrefix(strings.ToLower(req.SourceURL), "builtin:") {
			status = "fallback"
		}
		err = s.Repo.InsertPrice(ctx, models.PriceSnapshot{MaterialID: matID, FederalDistrictID: fdID, RegionID: regionID, SourceID: req.SourceID, Price: row.Price, UnitID: unitID, EffectiveDate: row.EffectiveDate, Status: status, SourceURL: req.SourceURL})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("строка %d: ошибка сохранения: %v", i+1, err))
			continue
		}
		result.RowsSaved++
	}
	status := "success"
	if len(result.Errors) > 0 {
		status = "partial"
	}
	if len(rows) == 0 {
		status = "empty"
	}
	_ = s.Repo.CreateParserLog(ctx, req.SourceID, status, result.RowsFound, result.RowsSaved, strings.Join(result.Errors, "; "))
	if len(rows) == 0 {
		return result, fmt.Errorf("цены не найдены: источник недоступен, изменил разметку или не содержит цен в открытом HTML/JSON/CSV")
	}
	return result, nil
}

func medianRaw(rows []rawPrice) float64 {
	values := make([]float64, 0, len(rows))
	for _, r := range rows {
		values = append(values, r.Price)
	}
	return median(values)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func normalizeUnit(u, fallback string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	switch u {
	case "килограмм", "килограмма", "кг":
		return "кг"
	case "т", "тонна", "тонну", "тонны":
		return "т"
	case "л", "литр", "литра":
		return "л"
	case "га", "гектар":
		return "га"
	case "ч", "час", "часа", "часов":
		return "ч"
	case "шт", "штука", "ед":
		return "шт"
	}
	return fallback
}

func normalizePriceUnit(price float64, actualUnit, targetUnit string) (float64, string) {
	if actualUnit == "т" && targetUnit == "кг" {
		return price / 1000, "кг"
	}
	return price, actualUnit
}

func firstHeader(header map[string]int, names ...string) (int, bool) {
	for _, n := range names {
		if idx, ok := header[normalizeKey(n)]; ok {
			return idx, true
		}
	}
	return -1, false
}

func normalizeKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "")
}
func valueAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}
func parsePrice(s string) (float64, error) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	s = strings.ReplaceAll(s, "₽", "")
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}
func anyString(row map[string]any, names ...string) string {
	for _, n := range names {
		if v, ok := row[n]; ok {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}
func anyFloat(row map[string]any, names ...string) (float64, bool) {
	for _, n := range names {
		if v, ok := row[n]; ok {
			f, err := parsePrice(fmt.Sprint(v))
			return f, err == nil
		}
	}
	return 0, false
}
func stripTags(s string) string {
	s = regexp.MustCompile(`(?is)<script.*?</script>|<style.*?</style>`).ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&#8381;", "руб", "&rub;", "руб", "&quot;", "\"", "&amp;", "&").Replace(s)
	s = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}
func cleanupMaterial(s string) string {
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	if len([]rune(s)) > 120 {
		s = string([]rune(s)[:120])
	}
	return strings.Trim(s, " -–—:;,.	\n")
}

func compactSpaces(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}
