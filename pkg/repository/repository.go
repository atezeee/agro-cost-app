package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"agro-cost-app/pkg/models"
)

type Repository struct {
	DB *sql.DB
}

func New(db *sql.DB) *Repository { return &Repository{DB: db} }

func (r *Repository) ListFederalDistricts(ctx context.Context) ([]models.FederalDistrict, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, code FROM federal_districts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.FederalDistrict
	for rows.Next() {
		var item models.FederalDistrict
		if err := rows.Scan(&item.ID, &item.Name, &item.Code); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListRegions(ctx context.Context, fdID int64) ([]models.Region, error) {
	query := `SELECT id, federal_district_id, name, code FROM regions`
	var args []any
	if fdID > 0 {
		query += ` WHERE federal_district_id=$1`
		args = append(args, fdID)
	}
	query += ` ORDER BY name`
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Region
	for rows.Next() {
		var item models.Region
		if err := rows.Scan(&item.ID, &item.FederalDistrictID, &item.Name, &item.Code); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListUnits(ctx context.Context) ([]models.Unit, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, short_name FROM units ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Unit
	for rows.Next() {
		var item models.Unit
		if err := rows.Scan(&item.ID, &item.Name, &item.ShortName); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListDirectory(ctx context.Context, table string) ([]models.DirectoryItem, error) {
	allowed := map[string]string{
		"crops":      `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric FROM crops ORDER BY name`,
		"operations": `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, COALESCE(unit_id, 0), ''::text, 0::numeric, 0::numeric FROM operations ORDER BY name`,
		"machines":   `SELECT id, name, COALESCE(machine_type, ''), COALESCE(cost_per_hour, 0), COALESCE(rent_cost_per_hour,0), 0::bigint, ''::text, 0::numeric, 0::numeric FROM machines ORDER BY name`,
		"materials":  `SELECT id, name, COALESCE(material_type, ''), COALESCE(default_price, 0), 0::numeric, COALESCE(unit_id, 0), ''::text, 0::numeric, 0::numeric FROM materials ORDER BY name`,
		"cost_items": `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric FROM cost_items ORDER BY id`,
	}
	query, ok := allowed[table]
	if !ok {
		return nil, fmt.Errorf("unknown directory: %s", table)
	}
	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.DirectoryItem
	for rows.Next() {
		var item models.DirectoryItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.DefaultPrice, &item.RentPrice, &item.UnitID, &item.Role, &item.Productivity, &item.FuelRate); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListPriceSources(ctx context.Context) ([]models.PriceSource, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, source_type, category, parser_type, COALESCE(url,''), COALESCE(note,''), priority, is_active FROM price_sources ORDER BY priority, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.PriceSource
	for rows.Next() {
		var item models.PriceSource
		if err := rows.Scan(&item.ID, &item.Name, &item.SourceType, &item.Category, &item.ParserType, &item.URL, &item.Note, &item.Priority, &item.IsActive); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ListActivePriceSources(ctx context.Context) ([]models.PriceSource, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, source_type, category, parser_type, COALESCE(url,''), COALESCE(note,''), priority, is_active FROM price_sources WHERE is_active=true ORDER BY priority, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.PriceSource
	for rows.Next() {
		var item models.PriceSource
		if err := rows.Scan(&item.ID, &item.Name, &item.SourceType, &item.Category, &item.ParserType, &item.URL, &item.Note, &item.Priority, &item.IsActive); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) FederalDistrictIDByNameOrCode(ctx context.Context, value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	var id int64
	err := r.DB.QueryRowContext(ctx, `SELECT id FROM federal_districts WHERE lower(name)=lower($1) OR lower(code)=lower($1) LIMIT 1`, value).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func (r *Repository) RegionIDByName(ctx context.Context, value string) (int64, int64, error) {
	if value == "" {
		return 0, 0, nil
	}
	var id, fdID int64
	err := r.DB.QueryRowContext(ctx, `SELECT id, federal_district_id FROM regions WHERE lower(name)=lower($1) OR lower(code)=lower($1) LIMIT 1`, value).Scan(&id, &fdID)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return id, fdID, err
}

func (r *Repository) PriceSourceByID(ctx context.Context, id int64) (*models.PriceSource, error) {
	if id <= 0 {
		return nil, nil
	}
	var item models.PriceSource
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, source_type, category, parser_type, COALESCE(url,''), COALESCE(note,''), priority, is_active FROM price_sources WHERE id=$1`, id).
		Scan(&item.ID, &item.Name, &item.SourceType, &item.Category, &item.ParserType, &item.URL, &item.Note, &item.Priority, &item.IsActive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) EnsureRuntimeSchema(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE calculations ALTER COLUMN region_id DROP NOT NULL`,
		`UPDATE price_sources SET is_active=false WHERE name NOT IN ('Benzup — средние цены топлива по регионам','Демонстрационный CSV')`,
		`UPDATE price_sources SET priority=1, is_active=true, parser_type='benzup_index_region', source_type='internet', category='fuel', url='https://benzup.ru/index-region', note='Основной источник цены дизельного топлива: таблица Benzup index-region, колонка ДТ, значения агрегируются до федеральных округов' WHERE name='Benzup — средние цены топлива по регионам'`,
		`UPDATE price_sources SET priority=99, is_active=true, parser_type='csv', source_type='local_csv', category='mixed', url='builtin:regional_prices', note='Резервная ценовая база для остальных ресурсов и случаев, когда внешний источник недоступен' WHERE name='Демонстрационный CSV'`,
		`INSERT INTO price_sources(name, source_type, category, parser_type, url, note, priority, is_active)
		 VALUES ('Benzup — средние цены топлива по регионам','internet','fuel','benzup_index_region','https://benzup.ru/index-region','Основной источник цены дизельного топлива: таблица Benzup index-region, колонка ДТ, значения агрегируются до федеральных округов',1,true)
		 ON CONFLICT (name) DO UPDATE SET source_type=EXCLUDED.source_type, category=EXCLUDED.category, parser_type=EXCLUDED.parser_type, url=EXCLUDED.url, note=EXCLUDED.note, priority=EXCLUDED.priority, is_active=true`,
		`INSERT INTO price_sources(name, source_type, category, parser_type, url, note, priority, is_active)
		 VALUES ('Демонстрационный CSV','local_csv','mixed','csv','builtin:regional_prices','Резервная ценовая база для остальных ресурсов и случаев, когда внешний источник недоступен',99,true)
		 ON CONFLICT (name) DO UPDATE SET source_type=EXCLUDED.source_type, category=EXCLUDED.category, parser_type=EXCLUDED.parser_type, url=EXCLUDED.url, note=EXCLUDED.note, priority=EXCLUDED.priority, is_active=true`,
		`INSERT INTO machines(name, machine_type, cost_per_hour, rent_cost_per_hour) VALUES
		 ('Трактор МТЗ-1221','трактор',1550,2350),
		 ('Трактор Кировец К-744Р4','трактор',2950,4300),
		 ('Трактор John Deere 8345R','трактор',4100,5900),
		 ('Плуг Lemken Juwel 8','почвообрабатывающий агрегат',850,1350),
		 ('Культиватор КШУ-8','почвообрабатывающий агрегат',780,1250),
		 ('Сеялка Horsch Pronto 6 DC','посевной агрегат',1300,2100),
		 ('Сеялка Vaderstad Tempo','посевной агрегат',1450,2300),
		 ('Разбрасыватель Amazone ZA-M','агрегат для внесения удобрений',980,1650),
		 ('Опрыскиватель Amazone UX','агрегат для защиты растений',1250,1950),
		 ('Комбайн ACROS 595 Plus','уборочная техника',4200,5900),
		 ('Комбайн Claas Lexion 770','уборочная техника',5600,7600),
		 ('Грузовой автомобиль КАМАЗ','транспорт',1650,2500)
		 ON CONFLICT(name) DO UPDATE SET machine_type=EXCLUDED.machine_type, cost_per_hour=EXCLUDED.cost_per_hour, rent_cost_per_hour=EXCLUDED.rent_cost_per_hour`,
		`INSERT INTO operation_machines(operation_id, machine_id, role, productivity_ha_per_hour, fuel_rate_l_per_ha)
		 SELECT o.id, m.id, v.role, v.prod, v.fuel
		 FROM (VALUES
		   ('Вспашка','Трактор Кировец К-744Р4','энергетическое средство',1.35,24.0),
		   ('Вспашка','Трактор John Deere 8345R','энергетическое средство',1.45,23.0),
		   ('Вспашка','Плуг Lemken Juwel 8','рабочий агрегат',1.35,0.0),
		   ('Культивация','Трактор МТЗ-1221','энергетическое средство',2.90,11.0),
		   ('Культивация','Культиватор КШУ-8','рабочий агрегат',2.90,0.0),
		   ('Посев','Трактор МТЗ-1221','энергетическое средство',2.20,8.0),
		   ('Посев','Сеялка Horsch Pronto 6 DC','посевной агрегат',2.20,0.0),
		   ('Посев','Сеялка Vaderstad Tempo','посевной агрегат',1.90,0.0),
		   ('Внесение удобрений','Трактор МТЗ-1221','энергетическое средство',3.80,7.0),
		   ('Внесение удобрений','Разбрасыватель Amazone ZA-M','агрегат',3.80,0.0),
		   ('Обработка СЗР','Трактор МТЗ-1221','энергетическое средство',8.50,4.0),
		   ('Обработка СЗР','Опрыскиватель Amazone UX','агрегат',8.50,0.0),
		   ('Междурядная обработка','Трактор МТЗ-1221','энергетическое средство',2.70,8.5),
		   ('Междурядная обработка','Культиватор КШУ-8','рабочий агрегат',2.70,0.0),
		   ('Уборка урожая','Комбайн ACROS 595 Plus','уборочная техника',1.90,17.5),
		   ('Уборка урожая','Комбайн Claas Lexion 770','уборочная техника',2.30,18.5),
		   ('Транспортировка урожая','Грузовой автомобиль КАМАЗ','транспорт',5.50,0.0)
		 ) AS v(operation_name, machine_name, role, prod, fuel)
		 JOIN operations o ON o.name=v.operation_name
		 JOIN machines m ON m.name=v.machine_name
		 ON CONFLICT(operation_id, machine_id) DO UPDATE SET role=EXCLUDED.role, productivity_ha_per_hour=EXCLUDED.productivity_ha_per_hour, fuel_rate_l_per_ha=EXCLUDED.fuel_rate_l_per_ha`,
	}
	for _, stmt := range statements {
		if _, err := r.DB.ExecContext(ctx, stmt); err != nil {
			// Старые базы могут не содержать отдельные источники/таблицы после частичных миграций.
			// Это не должно останавливать сайт, потому что основная ошибка, которую нужно снять,
			// — NOT NULL у calculations.region_id.
			continue
		}
	}
	return nil
}

func (r *Repository) LatestPrice(ctx context.Context, materialID, regionID, fdID int64) (*models.PriceSnapshot, error) {
	query := `
	SELECT ps.id, ps.material_id, m.name, ps.federal_district_id, fd.name,
	       COALESCE(ps.region_id, 0), COALESCE(reg.name, ''), ps.source_id, src.name,
	       ps.price, ps.unit_id, u.short_name, ps.effective_date, ps.parsed_at, ps.status, ps.source_url
	FROM price_snapshots ps
	JOIN materials m ON m.id = ps.material_id
	JOIN federal_districts fd ON fd.id = ps.federal_district_id
	LEFT JOIN regions reg ON reg.id = ps.region_id
	JOIN price_sources src ON src.id = ps.source_id
	JOIN units u ON u.id = ps.unit_id
	WHERE ps.material_id=$1 AND ps.status IN ('validated','new','parsed','fallback')
	  AND (
	      ($2::bigint > 0 AND ps.region_id=$2)
	      OR (ps.region_id IS NULL AND ps.federal_district_id=$3)
	  )
	ORDER BY
	  CASE WHEN $2::bigint > 0 AND ps.region_id=$2 THEN 0 ELSE 1 END,
	  CASE
	    WHEN lower(src.name) LIKE '%benzup%' THEN 0
	    WHEN lower(src.name) LIKE '%multigo%' THEN 1
	    WHEN lower(src.name) LIKE '%спбмтсб%' OR lower(src.name) LIKE '%spimex%' THEN 2
	    WHEN src.name = 'Демонстрационный CSV' THEN 9
	    ELSE 5
	  END,
	  src.priority ASC,
	  ps.effective_date DESC,
	  ps.parsed_at DESC
	LIMIT 1`
	var p models.PriceSnapshot
	err := r.DB.QueryRowContext(ctx, query, materialID, regionID, fdID).Scan(
		&p.ID, &p.MaterialID, &p.MaterialName, &p.FederalDistrictID, &p.FederalDistrict,
		&p.RegionID, &p.RegionName, &p.SourceID, &p.SourceName,
		&p.Price, &p.UnitID, &p.UnitName, &p.EffectiveDate, &p.ParsedAt, &p.Status, &p.SourceURL,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) FindOrCreateMaterial(ctx context.Context, name string, unitID int64) (int64, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `SELECT id FROM materials WHERE lower(name)=lower($1)`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	err = r.DB.QueryRowContext(ctx, `INSERT INTO materials(name, material_type, unit_id, default_price) VALUES($1,'parsed', $2, 0) RETURNING id`, name, unitID).Scan(&id)
	return id, err
}

func (r *Repository) UnitIDByShortName(ctx context.Context, short string) (int64, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `SELECT id FROM units WHERE lower(short_name)=lower($1) LIMIT 1`, short).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("неизвестная единица измерения: %s", short)
	}
	return id, err
}

func (r *Repository) UnitIDByAnyName(ctx context.Context, unit string) (int64, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `SELECT id FROM units WHERE lower(short_name)=lower($1) OR lower(name)=lower($1) LIMIT 1`, unit).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("неизвестная единица измерения: %s", unit)
	}
	return id, err
}

func (r *Repository) EnsurePriceSource(ctx context.Context, req models.ParseRequest) (int64, error) {
	if req.SourceID > 0 {
		return req.SourceID, nil
	}
	if req.SourceName == "" {
		req.SourceName = "Внешний источник"
	}
	if req.SourceType == "" {
		req.SourceType = "external"
	}
	if req.ParserType == "" {
		req.ParserType = "csv"
	}
	if req.Category == "" {
		req.Category = "mixed"
	}
	var id int64
	priority := 50
	nameLower := strings.ToLower(req.SourceName)
	parserLower := strings.ToLower(req.ParserType)
	if strings.Contains(nameLower, "benzup") || parserLower == "benzup_index_region" || parserLower == "benzup_fuel_api" {
		priority = 1
	}
	if req.SourceName == "Демонстрационный CSV" || strings.Contains(req.SourceURL, "builtin:regional_prices") {
		priority = 99
	}
	err := r.DB.QueryRowContext(ctx, `
		INSERT INTO price_sources(name, source_type, category, parser_type, url, note, priority, is_active)
		VALUES($1,$2,$3,$4,$5,'Добавлен автоматически', $6, true)
		ON CONFLICT(name) DO UPDATE SET
			source_type=EXCLUDED.source_type,
			category=EXCLUDED.category,
			parser_type=EXCLUDED.parser_type,
			url=EXCLUDED.url,
			priority=LEAST(price_sources.priority, EXCLUDED.priority),
			is_active=true
		RETURNING id`,
		req.SourceName, req.SourceType, req.Category, req.ParserType, req.SourceURL, priority).Scan(&id)
	return id, err
}

func (r *Repository) InsertPrice(ctx context.Context, p models.PriceSnapshot) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO price_snapshots(material_id, federal_district_id, region_id, source_id, price, unit_id, effective_date, status, source_url)
		VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,$9)`,
		p.MaterialID, p.FederalDistrictID, p.RegionID, p.SourceID, p.Price, p.UnitID, p.EffectiveDate, p.Status, p.SourceURL,
	)
	return err
}

func (r *Repository) CreateParserLog(ctx context.Context, sourceID int64, status string, found, saved int, msg string) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO parser_logs(source_id, started_at, finished_at, status, rows_found, rows_saved, error_message) VALUES($1,NOW(),NOW(),$2,$3,$4,$5)`, sourceID, status, found, saved, msg)
	return err
}

func (r *Repository) ListPrices(ctx context.Context, regionID, fdID int64) ([]models.PriceSnapshot, error) {
	query := `
	SELECT ps.id, ps.material_id, m.name, ps.federal_district_id, fd.name,
	       COALESCE(ps.region_id, 0), COALESCE(reg.name, ''), ps.source_id, src.name,
	       ps.price, ps.unit_id, u.short_name, ps.effective_date, ps.parsed_at, ps.status, ps.source_url
	FROM price_snapshots ps
	JOIN materials m ON m.id = ps.material_id
	JOIN federal_districts fd ON fd.id = ps.federal_district_id
	LEFT JOIN regions reg ON reg.id = ps.region_id
	JOIN price_sources src ON src.id = ps.source_id
	JOIN units u ON u.id = ps.unit_id
	WHERE ($2::bigint=0 OR ps.federal_district_id=$2)
	  AND ($1::bigint=0 OR ps.region_id=$1)
	ORDER BY
	  CASE
	    WHEN lower(src.name) LIKE '%benzup%' THEN 0
	    WHEN lower(src.name) LIKE '%multigo%' THEN 1
	    WHEN lower(src.name) LIKE '%спбмтсб%' OR lower(src.name) LIKE '%spimex%' THEN 2
	    WHEN src.name = 'Демонстрационный CSV' THEN 9
	    ELSE 5
	  END,
	  src.priority ASC,
	  ps.effective_date DESC,
	  ps.parsed_at DESC,
	  m.name
	LIMIT 200`
	rows, err := r.DB.QueryContext(ctx, query, regionID, fdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.PriceSnapshot
	for rows.Next() {
		var p models.PriceSnapshot
		if err := rows.Scan(&p.ID, &p.MaterialID, &p.MaterialName, &p.FederalDistrictID, &p.FederalDistrict,
			&p.RegionID, &p.RegionName, &p.SourceID, &p.SourceName, &p.Price, &p.UnitID, &p.UnitName,
			&p.EffectiveDate, &p.ParsedAt, &p.Status, &p.SourceURL); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) ListOperationMachines(ctx context.Context, operationID int64) ([]models.DirectoryItem, error) {
	query := `
	SELECT m.id, m.name, COALESCE(m.machine_type,''), COALESCE(m.cost_per_hour,0), COALESCE(m.rent_cost_per_hour,0),
	       0::bigint, om.role, COALESCE(om.productivity_ha_per_hour,0), COALESCE(om.fuel_rate_l_per_ha,0)
	FROM operation_machines om
	JOIN machines m ON m.id=om.machine_id
	WHERE om.operation_id=$1
	ORDER BY om.role, m.name`
	rows, err := r.DB.QueryContext(ctx, query, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.DirectoryItem
	for rows.Next() {
		var item models.DirectoryItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.DefaultPrice, &item.RentPrice, &item.UnitID, &item.Role, &item.Productivity, &item.FuelRate); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) SaveCalculation(ctx context.Context, req models.CalculationRequest, result models.CalculationResult) (int64, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var calcID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO calculations(user_id, guest_id, crop_id, federal_district_id, region_id, area_ha, total_cost, cost_per_ha) VALUES(NULLIF($1,0),NULLIF($2,''),$3,$4,NULLIF($5,0),$6,$7,$8) RETURNING id`, req.UserID, req.GuestID, req.CropID, req.FederalDistrictID, req.RegionID, req.AreaHa, result.TotalCost, result.CostPerHa).Scan(&calcID)
	if err != nil {
		return 0, err
	}
	for _, row := range result.Rows {
		_, err := tx.ExecContext(ctx, `INSERT INTO calculation_rows(calculation_id, operation_id, material_id, machine_id, machine_usage_type, cost_item_id, price_snapshot_id, quantity, rate, price, coefficient, amount, machine_price_source) VALUES($1,$2,$3,NULLIF($4,0),$5,$6,$7,$8,$9,$10,$11,$12,$13)`, calcID, row.OperationID, row.MaterialID, row.MachineID, row.MachineUsageType, row.CostItemID, row.PriceSnapshotID, row.Quantity, row.Rate, row.Price, row.Coefficient, row.Amount, row.MachinePriceSource)
		if err != nil {
			return 0, err
		}
	}
	return calcID, tx.Commit()
}

func (r *Repository) ListCalculations(ctx context.Context, userID int64, guestID string) ([]models.CalculationResult, error) {
	query := `SELECT c.id, c.crop_id, cr.name, c.federal_district_id, fd.name, COALESCE(c.region_id,0), COALESCE(reg.name,''), c.area_ha, c.total_cost, c.cost_per_ha, c.created_at
		FROM calculations c
		JOIN crops cr ON cr.id=c.crop_id
		JOIN federal_districts fd ON fd.id=c.federal_district_id
		LEFT JOIN regions reg ON reg.id=c.region_id`
	var args []any
	if userID > 0 {
		query += ` WHERE c.user_id=$1`
		args = append(args, userID)
	} else {
		query += ` WHERE COALESCE(c.guest_id,'')=$1`
		args = append(args, guestID)
	}
	query += ` ORDER BY c.created_at DESC LIMIT 100`
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.CalculationResult
	for rows.Next() {
		var c models.CalculationResult
		if err := rows.Scan(&c.ID, &c.CropID, &c.CropName, &c.FederalDistrictID, &c.FederalDistrict, &c.RegionID, &c.RegionName, &c.AreaHa, &c.TotalCost, &c.CostPerHa, &c.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) GetCalculation(ctx context.Context, id, userID int64, guestID string) (models.CalculationResult, error) {
	query := `SELECT c.id, c.crop_id, cr.name, c.federal_district_id, fd.name, COALESCE(c.region_id,0), COALESCE(reg.name,''), c.area_ha, c.total_cost, c.cost_per_ha, c.created_at
		FROM calculations c
		JOIN crops cr ON cr.id=c.crop_id
		JOIN federal_districts fd ON fd.id=c.federal_district_id
		LEFT JOIN regions reg ON reg.id=c.region_id
		WHERE c.id=$1`
	args := []any{id}
	if userID > 0 {
		query += ` AND c.user_id=$2`
		args = append(args, userID)
	} else {
		query += ` AND COALESCE(c.guest_id,'')=$2`
		args = append(args, guestID)
	}
	var c models.CalculationResult
	err := r.DB.QueryRowContext(ctx, query, args...).Scan(&c.ID, &c.CropID, &c.CropName, &c.FederalDistrictID, &c.FederalDistrict, &c.RegionID, &c.RegionName, &c.AreaHa, &c.TotalCost, &c.CostPerHa, &c.CreatedAt)
	if err != nil {
		return c, err
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT cr.operation_id, COALESCE(op.name,''), cr.material_id, COALESCE(mat.name,''),
		       COALESCE(cr.machine_id,0), COALESCE(m.name,''), COALESCE(cr.machine_usage_type,''),
		       cr.cost_item_id, COALESCE(ci.name,''), cr.price_snapshot_id, cr.quantity, cr.price, cr.rate, cr.coefficient, cr.amount, COALESCE(cr.machine_price_source,'')
		FROM calculation_rows cr
		LEFT JOIN operations op ON op.id=cr.operation_id
		LEFT JOIN materials mat ON mat.id=cr.material_id
		LEFT JOIN machines m ON m.id=cr.machine_id
		LEFT JOIN cost_items ci ON ci.id=cr.cost_item_id
		WHERE cr.calculation_id=$1
		ORDER BY cr.id`, id)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var row models.CalculationResultRow
		if err := rows.Scan(&row.OperationID, &row.OperationName, &row.MaterialID, &row.MaterialName, &row.MachineID, &row.MachineName, &row.MachineUsageType, &row.CostItemID, &row.CostItemName, &row.PriceSnapshotID, &row.Quantity, &row.Price, &row.Rate, &row.Coefficient, &row.Amount, &row.MachinePriceSource); err != nil {
			return c, err
		}
		c.Rows = append(c.Rows, row)
	}
	return c, rows.Err()
}

func (r *Repository) DeleteCalculation(ctx context.Context, id, userID int64, guestID string) error {
	query := `DELETE FROM calculations WHERE id=$1`
	args := []any{id}
	if userID > 0 {
		query += ` AND user_id=$2`
		args = append(args, userID)
	} else {
		query += ` AND COALESCE(guest_id,'')=$2`
		args = append(args, guestID)
	}
	res, err := r.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("расчёт не найден или недоступен")
	}
	return nil
}

func DateOnlyNow() time.Time { return time.Now().Truncate(24 * time.Hour) }

func (r *Repository) CreateUser(ctx context.Context, fullName, email, passwordHash string) (models.User, error) {
	var u models.User
	err := r.DB.QueryRowContext(ctx, `INSERT INTO users(full_name, email, password_hash, role) VALUES($1,$2,$3,'economist') RETURNING id, full_name, email, role, COALESCE(password_hash,''), created_at`, fullName, email, passwordHash).
		Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	err := r.DB.QueryRowContext(ctx, `SELECT id, full_name, COALESCE(email,''), role, COALESCE(password_hash,''), created_at FROM users WHERE lower(email)=lower($1)`, email).
		Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return models.User{}, nil
	}
	return u, err
}

func (r *Repository) UserByID(ctx context.Context, id int64) (models.User, error) {
	var u models.User
	err := r.DB.QueryRowContext(ctx, `SELECT id, full_name, COALESCE(email,''), role, COALESCE(password_hash,''), created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.FullName, &u.Email, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return models.User{}, nil
	}
	return u, err
}

func (r *Repository) MachineByID(ctx context.Context, id int64) (models.DirectoryItem, error) {
	var m models.DirectoryItem
	if id <= 0 {
		return m, nil
	}
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(machine_type,''), COALESCE(cost_per_hour,0), COALESCE(rent_cost_per_hour,0), 0::bigint, ''::text, 0::numeric, 0::numeric FROM machines WHERE id=$1`, id).
		Scan(&m.ID, &m.Name, &m.Description, &m.DefaultPrice, &m.RentPrice, &m.UnitID, &m.Role, &m.Productivity, &m.FuelRate)
	if err == sql.ErrNoRows {
		return models.DirectoryItem{}, nil
	}
	return m, err
}

func (r *Repository) CostItemByID(ctx context.Context, id int64) (models.DirectoryItem, error) {
	var c models.DirectoryItem
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(description,''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric FROM cost_items WHERE id=$1`, id).
		Scan(&c.ID, &c.Name, &c.Description, &c.DefaultPrice, &c.RentPrice, &c.UnitID, &c.Role, &c.Productivity, &c.FuelRate)
	if err == sql.ErrNoRows {
		return models.DirectoryItem{}, nil
	}
	return c, err
}

func (r *Repository) PriceSourceByName(ctx context.Context, name string) (*models.PriceSource, error) {
	if name == "" {
		return nil, nil
	}
	var item models.PriceSource
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, source_type, category, parser_type, COALESCE(url,''), COALESCE(note,''), priority, is_active FROM price_sources WHERE name=$1`, name).
		Scan(&item.ID, &item.Name, &item.SourceType, &item.Category, &item.ParserType, &item.URL, &item.Note, &item.Priority, &item.IsActive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
