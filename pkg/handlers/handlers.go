package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"agro-cost-app/pkg/models"
	"agro-cost-app/pkg/repository"
	"agro-cost-app/pkg/services"
)

type Handler struct {
	Repo         *repository.Repository
	Calc         *services.CalculationService
	Parser       *services.ParserService
	Auth         *services.AuthService
	AuthRequired bool
}

func New(repo *repository.Repository, calc *services.CalculationService, parser *services.ParserService, auth *services.AuthService, authRequired bool) *Handler {
	return &Handler{Repo: repo, Calc: calc, Parser: parser, Auth: auth, AuthRequired: authRequired}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/cron/parse-prices", h.cronParsePrices)
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("GET /api/auth/me", h.me)

	mux.HandleFunc("GET /api/federal-districts", h.federalDistricts)
	mux.HandleFunc("GET /api/regions", h.regions)
	mux.HandleFunc("GET /api/units", h.units)
	mux.HandleFunc("GET /api/crops", h.directory("crops"))
	mux.HandleFunc("GET /api/operations", h.directory("operations"))
	mux.HandleFunc("GET /api/machines", h.directory("machines"))
	mux.HandleFunc("GET /api/operation-machines", h.operationMachines)
	mux.HandleFunc("GET /api/materials", h.directory("materials"))
	mux.HandleFunc("GET /api/cost-items", h.directory("cost_items"))
	mux.HandleFunc("GET /api/prices", h.prices)
	mux.HandleFunc("GET /api/prices/sources", h.priceSources)
	mux.HandleFunc("POST /api/prices/parse", h.requireAuth(h.parsePrices))
	mux.HandleFunc("POST /api/prices/parse-source", h.requireAuth(h.parsePriceSource))
	mux.HandleFunc("POST /api/prices/import-csv", h.requireAuth(h.importCSV))
	mux.HandleFunc("POST /api/calculate", h.calculate(false))
	mux.HandleFunc("POST /api/calculations", h.calculate(true))
	mux.HandleFunc("GET /api/calculations", h.calculations)
	mux.HandleFunc("GET /api/calculations/{id}", h.calculationDetail)
	mux.HandleFunc("DELETE /api/calculations/{id}", h.deleteCalculation)
}

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.AuthRequired {
			next(w, r)
			return
		}
		if _, ok := h.Auth.CurrentUser(r.Context(), r); !ok {
			respondError(w, http.StatusUnauthorized, "необходимо войти в систему")
			return
		}
		next(w, r)
	}
}

func (h *Handler) cronParsePrices(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(r.URL.Query().Get("secret"))
	// Если CRON_SECRET задан в окружении, Vercel Cron должен передавать его в query-параметре.
	if expected := strings.TrimSpace(os.Getenv("CRON_SECRET")); expected != "" && secret != expected {
		respondError(w, http.StatusUnauthorized, "неверный CRON_SECRET")
		return
	}
	fdID, _ := strconv.ParseInt(r.URL.Query().Get("federal_district_id"), 10, 64)
	regionID, _ := strconv.ParseInt(r.URL.Query().Get("region_id"), 10, 64)
	if fdID == 0 {
		if parsedFD, err := h.Repo.FederalDistrictIDByNameOrCode(r.Context(), getenvDefault("DEFAULT_FEDERAL_DISTRICT", "ЮФО")); err == nil && parsedFD > 0 {
			fdID = parsedFD
		}
	}
	res, err := h.Parser.ParseAllSources(r.Context(), fdID, regionID)
	respond(w, res, err)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	respond(w, map[string]string{"status": "ok"}, nil)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct{ FullName, Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	user, err := h.Auth.Register(r.Context(), req.FullName, req.Email, req.Password)
	if err == nil {
		h.Auth.SetSessionCookie(w, user.ID)
	}
	respond(w, user, err)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	user, err := h.Auth.Login(r.Context(), req.Email, req.Password)
	if err == nil {
		h.Auth.SetSessionCookie(w, user.ID)
	}
	respond(w, user, err)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.Auth.ClearSessionCookie(w)
	respond(w, map[string]string{"status": "ok"}, nil)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.Auth.CurrentUser(r.Context(), r)
	respond(w, map[string]any{"authenticated": ok, "user": user, "auth_required": h.AuthRequired}, nil)
}

func (h *Handler) federalDistricts(w http.ResponseWriter, r *http.Request) {
	items, err := h.Repo.ListFederalDistricts(r.Context())
	respond(w, items, err)
}

func (h *Handler) regions(w http.ResponseWriter, r *http.Request) {
	fdID, _ := strconv.ParseInt(r.URL.Query().Get("federal_district_id"), 10, 64)
	items, err := h.Repo.ListRegions(r.Context(), fdID)
	respond(w, items, err)
}

func (h *Handler) units(w http.ResponseWriter, r *http.Request) {
	items, err := h.Repo.ListUnits(r.Context())
	respond(w, items, err)
}

func (h *Handler) directory(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.Repo.ListDirectory(r.Context(), name)
		respond(w, items, err)
	}
}

func (h *Handler) operationMachines(w http.ResponseWriter, r *http.Request) {
	operationID, _ := strconv.ParseInt(r.URL.Query().Get("operation_id"), 10, 64)
	if operationID <= 0 {
		items, err := h.Repo.ListDirectory(r.Context(), "machines")
		respond(w, items, err)
		return
	}
	items, err := h.Repo.ListOperationMachines(r.Context(), operationID)
	respond(w, items, err)
}

func (h *Handler) prices(w http.ResponseWriter, r *http.Request) {
	regionID, _ := strconv.ParseInt(r.URL.Query().Get("region_id"), 10, 64)
	fdID, _ := strconv.ParseInt(r.URL.Query().Get("federal_district_id"), 10, 64)
	items, err := h.Repo.ListPrices(r.Context(), regionID, fdID)
	respond(w, items, err)
}

func (h *Handler) priceSources(w http.ResponseWriter, r *http.Request) {
	items, err := h.Repo.ListPriceSources(r.Context())
	respond(w, items, err)
}

func (h *Handler) parsePrices(w http.ResponseWriter, r *http.Request) {
	var req models.ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	res, err := h.Parser.ParseFromURL(r.Context(), req)
	respond(w, res, err)
}

func (h *Handler) parsePriceSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID          int64 `json:"source_id"`
		FederalDistrictID int64 `json:"federal_district_id"`
		RegionID          int64 `json:"region_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	res, err := h.Parser.ParseSourceByID(r.Context(), req.SourceID, req.FederalDistrictID, req.RegionID)
	respond(w, res, err)
}

func (h *Handler) importCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "не удалось прочитать форму")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "прикрепите CSV-файл")
		return
	}
	defer file.Close()
	fdID, _ := strconv.ParseInt(r.FormValue("federal_district_id"), 10, 64)
	regionID, _ := strconv.ParseInt(r.FormValue("region_id"), 10, 64)
	sourceID, _ := strconv.ParseInt(r.FormValue("source_id"), 10, 64)
	req := models.ParseRequest{SourceID: sourceID, SourceName: r.FormValue("source_name"), SourceURL: r.FormValue("source_url"), SourceType: "local_csv", Category: "mixed", ParserType: "csv", FederalDistrictID: fdID, RegionID: regionID}
	if req.SourceName == "" {
		req.SourceName = "CSV импорт"
	}
	if req.SourceURL == "" {
		req.SourceURL = "local_upload"
	}
	sid, err := h.Repo.EnsurePriceSource(r.Context(), req)
	if err != nil {
		respond(w, nil, err)
		return
	}
	req.SourceID = sid
	res, err := h.Parser.ParseCSV(r.Context(), req, file)
	respond(w, res, err)
}

func (h *Handler) calculate(save bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.CalculationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "некорректный JSON")
			return
		}
		if user, ok := h.Auth.CurrentUser(r.Context(), r); ok && req.UserID == 0 {
			req.UserID = user.ID
		} else if save {
			req.GuestID = h.guestID(w, r)
		}
		res, err := h.Calc.Calculate(r.Context(), req, save)
		respond(w, res, err)
	}
}

func (h *Handler) calculations(w http.ResponseWriter, r *http.Request) {
	if user, ok := h.Auth.CurrentUser(r.Context(), r); ok {
		items, err := h.Repo.ListCalculations(r.Context(), user.ID, "")
		respond(w, items, err)
		return
	}
	guestID := h.guestID(w, r)
	items, err := h.Repo.ListCalculations(r.Context(), 0, guestID)
	respond(w, items, err)
}

func (h *Handler) calculationDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if id <= 0 {
		respondError(w, http.StatusBadRequest, "некорректный идентификатор расчёта")
		return
	}
	if user, ok := h.Auth.CurrentUser(r.Context(), r); ok {
		item, err := h.Repo.GetCalculation(r.Context(), id, user.ID, "")
		respond(w, item, err)
		return
	}
	guestID := h.guestID(w, r)
	item, err := h.Repo.GetCalculation(r.Context(), id, 0, guestID)
	respond(w, item, err)
}

func (h *Handler) deleteCalculation(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if id <= 0 {
		respondError(w, http.StatusBadRequest, "некорректный идентификатор расчёта")
		return
	}
	if user, ok := h.Auth.CurrentUser(r.Context(), r); ok {
		err := h.Repo.DeleteCalculation(r.Context(), id, user.ID, "")
		respond(w, map[string]string{"status": "deleted"}, err)
		return
	}
	guestID := h.guestID(w, r)
	err := h.Repo.DeleteCalculation(r.Context(), id, 0, guestID)
	respond(w, map[string]string{"status": "deleted"}, err)
}

func (h *Handler) guestID(w http.ResponseWriter, r *http.Request) string {
	const cookieName = "agro_guest_id"
	if c, err := r.Cookie(cookieName); err == nil && strings.TrimSpace(c.Value) != "" {
		return c.Value
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "guest-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", MaxAge: 60 * 60 * 24 * 365, SameSite: http.SameSiteLaxMode, HttpOnly: true, Secure: r.TLS != nil})
	return id
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		if strings.Contains(msg, "выберите") || strings.Contains(msg, "должна") || strings.Contains(msg, "нет цены") || strings.Contains(msg, "CSV") || strings.Contains(msg, "укажите") || strings.Contains(msg, "неверный") || strings.Contains(msg, "пароль") {
			status = http.StatusBadRequest
		}
		respondError(w, status, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func getenvDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func DrainAndClose(rc io.ReadCloser) { io.Copy(io.Discard, rc); rc.Close() }
