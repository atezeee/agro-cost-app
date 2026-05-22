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
		"crops":      `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric, false, ''::text, ''::text FROM crops ORDER BY name`,
		"operations": `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, COALESCE(unit_id, 0), ''::text, 0::numeric, 0::numeric, COALESCE(requires_crop,true), COALESCE(resource_material_type,''), COALESCE(resource_title,'') FROM operations ORDER BY name`,
		"machines":   `SELECT id, name, COALESCE(machine_type, ''), COALESCE(cost_per_hour, 0), COALESCE(rent_cost_per_hour,0), 0::bigint, ''::text, 0::numeric, 0::numeric, false, ''::text, ''::text FROM machines ORDER BY name`,
		"materials":  `SELECT id, name, COALESCE(material_type, ''), COALESCE(default_price, 0), 0::numeric, COALESCE(unit_id, 0), ''::text, 0::numeric, 0::numeric, false, COALESCE(material_type,''), ''::text FROM materials ORDER BY name`,
		"cost_items": `SELECT id, name, COALESCE(description, ''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric, false, ''::text, ''::text FROM cost_items ORDER BY id`,
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
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.DefaultPrice, &item.RentPrice, &item.UnitID, &item.Role, &item.Productivity, &item.FuelRate, &item.RequiresCrop, &item.ResourceType, &item.ResourceTitle); err != nil {
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
		`ALTER TABLE operations ADD COLUMN IF NOT EXISTS requires_crop BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_material_type VARCHAR(100)`,
		`ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_title VARCHAR(255)`,
		`ALTER TABLE norms ADD COLUMN IF NOT EXISTS material_type VARCHAR(100)`,
		`CREATE TABLE IF NOT EXISTS condition_coefficients (
			id SERIAL PRIMARY KEY,
			group_code VARCHAR(50) NOT NULL,
			group_name VARCHAR(100) NOT NULL,
			name VARCHAR(255) NOT NULL,
			value NUMERIC(10,4) NOT NULL CHECK(value > 0),
			description TEXT,
			UNIQUE(group_code, name)
		)`,
		`CREATE TABLE IF NOT EXISTS tech_map_templates (
			id SERIAL PRIMARY KEY,
			crop_id INT NOT NULL REFERENCES crops(id) ON DELETE CASCADE,
			operation_id INT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
			sort_order INT NOT NULL DEFAULT 0,
			phase VARCHAR(100) NOT NULL DEFAULT '',
			is_required BOOLEAN NOT NULL DEFAULT TRUE,
			area_factor NUMERIC(10,4) NOT NULL DEFAULT 1 CHECK(area_factor >= 0),
			UNIQUE(crop_id, operation_id)
		)`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS calculation_mode VARCHAR(30) NOT NULL DEFAULT 'single_operation'`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS include_comparison BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_own_total NUMERIC(14,2)`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_rent_total NUMERIC(14,2)`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_delta NUMERIC(14,2)`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_cheaper_usage_type VARCHAR(20)`,
		`ALTER TABLE calculations ADD COLUMN IF NOT EXISTS total_machine_hours NUMERIC(14,4) NOT NULL DEFAULT 0`,
		`ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS price_source VARCHAR(255)`,
		`ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS price_date DATE`,
		`ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS source_url TEXT`,
		`ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS area_factor NUMERIC(10,4) NOT NULL DEFAULT 1`,
		`UPDATE operations SET requires_crop=false, resource_material_type=NULL, resource_title=NULL WHERE name IN ('Вспашка','Культивация','Лущение стерни','Боронование','Прикатывание','Междурядная обработка','Транспортировка урожая')`,
		`UPDATE operations SET requires_crop=true, resource_material_type=NULL, resource_title=NULL WHERE name='Уборка урожая'`,
		`UPDATE operations SET requires_crop=true, resource_material_type='seed', resource_title='Семенной материал' WHERE name='Посев'`,
		`UPDATE operations SET requires_crop=true, resource_material_type='fertilizer', resource_title='Удобрение' WHERE name='Внесение удобрений'`,
		`UPDATE operations SET requires_crop=true, resource_material_type='pesticide', resource_title='Средство защиты растений' WHERE name='Обработка СЗР'`,
		`INSERT INTO cost_items(name, description) VALUES ('Амортизация','Доля стоимости собственной техники'),('Ремонт','Ремонт собственной техники'),('Техническое обслуживание','ТО собственной техники') ON CONFLICT(name) DO NOTHING`,
		`INSERT INTO condition_coefficients(group_code, group_name, name, value, description) VALUES
			('moisture','Влажность','Нормальная влажность',1.00,'Базовые условия работы'),
			('moisture','Влажность','Повышенная влажность',1.10,'Работа после осадков или по влажной почве'),
			('moisture','Влажность','Переувлажнение',1.25,'Сложные условия движения и обработки'),
			('relief','Рельеф','Ровное поле',1.00,'Базовые условия'),
			('relief','Рельеф','Слабый уклон',1.05,'Небольшой перерасход времени и топлива'),
			('relief','Рельеф','Сложный рельеф',1.15,'Выраженные перепады и развороты'),
			('stoniness','Каменистость','Без камней',1.00,'Базовые условия'),
			('stoniness','Каменистость','Средняя каменистость',1.08,'Дополнительная нагрузка на агрегат'),
			('stoniness','Каменистость','Высокая каменистость',1.18,'Существенное снижение темпа работ'),
			('distance','Удалённость','До 5 км',1.00,'Базовые условия'),
			('distance','Удалённость','5–15 км',1.04,'Дополнительные переезды'),
			('distance','Удалённость','Более 15 км',1.10,'Значимые транспортные потери времени'),
			('complexity','Сложность','Обычная сложность',1.00,'Базовые условия'),
			('complexity','Сложность','Сложный контур поля',1.07,'Много разворотов и клиньев'),
			('complexity','Сложность','Очень сложный контур',1.16,'Существенные потери производительности')
		 ON CONFLICT(group_code, name) DO UPDATE SET group_name=EXCLUDED.group_name, value=EXCLUDED.value, description=EXCLUDED.description`,
		`INSERT INTO norms(crop_id, operation_id, material_id, rate, unit_id, material_type)
		 SELECT c.id, o.id, m.id,
			CASE WHEN m.name='Семена пшеницы' THEN 180 WHEN m.name='Семена ячменя' THEN 170 WHEN m.name='Семена подсолнечника' THEN 7 WHEN m.name='Семена кукурузы' THEN 25 WHEN m.name='Аммиачная селитра' THEN 100 WHEN m.name='Гербицид' THEN 1.2 ELSE 0 END,
			m.unit_id, m.material_type
		 FROM (VALUES
			('Пшеница','Посев','Семена пшеницы'),('Ячмень','Посев','Семена ячменя'),('Подсолнечник','Посев','Семена подсолнечника'),('Кукуруза','Посев','Семена кукурузы'),
			('Пшеница','Внесение удобрений','Аммиачная селитра'),('Ячмень','Внесение удобрений','Аммиачная селитра'),('Подсолнечник','Внесение удобрений','Аммиачная селитра'),('Кукуруза','Внесение удобрений','Аммиачная селитра'),
			('Пшеница','Обработка СЗР','Гербицид'),('Ячмень','Обработка СЗР','Гербицид'),('Подсолнечник','Обработка СЗР','Гербицид'),('Кукуруза','Обработка СЗР','Гербицид')
		 ) AS v(crop_name, operation_name, material_name)
		 JOIN crops c ON c.name=v.crop_name JOIN operations o ON o.name=v.operation_name JOIN materials m ON m.name=v.material_name
		 ON CONFLICT(crop_id, operation_id, material_id) DO UPDATE SET rate=EXCLUDED.rate, unit_id=EXCLUDED.unit_id, material_type=EXCLUDED.material_type`,
		`INSERT INTO tech_map_templates(crop_id, operation_id, sort_order, phase, is_required, area_factor)
		 SELECT c.id, o.id, v.sort_order, v.phase, v.is_required, v.area_factor
		 FROM crops c
		 JOIN (VALUES
			('Лущение стерни',10,'Послеуборочная обработка',false,1.0::numeric),('Вспашка',20,'Основная обработка почвы',true,1.0::numeric),('Боронование',30,'Закрытие влаги',false,1.0::numeric),
			('Культивация',40,'Предпосевная подготовка',true,1.0::numeric),('Внесение удобрений',50,'Питание',true,1.0::numeric),('Посев',60,'Посев',true,1.0::numeric),
			('Прикатывание',70,'После посева',false,1.0::numeric),('Обработка СЗР',80,'Защита растений',true,1.0::numeric),('Междурядная обработка',90,'Уход за посевами',false,1.0::numeric),
			('Уборка урожая',100,'Уборка',true,1.0::numeric),('Транспортировка урожая',110,'Логистика',false,1.0::numeric)
		 ) AS v(operation_name, sort_order, phase, is_required, area_factor) ON true
		 JOIN operations o ON o.name=v.operation_name
		 WHERE c.name IN ('Пшеница','Ячмень','Кукуруза','Подсолнечник')
		 ON CONFLICT(crop_id, operation_id) DO UPDATE SET sort_order=EXCLUDED.sort_order, phase=EXCLUDED.phase, is_required=EXCLUDED.is_required, area_factor=EXCLUDED.area_factor`,
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

func (r *Repository) ListOperationRules(ctx context.Context) ([]models.OperationRule, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, COALESCE(description,''), COALESCE(requires_crop,true), COALESCE(resource_material_type,''), COALESCE(resource_title,'') FROM operations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.OperationRule
	for rows.Next() {
		var item models.OperationRule
		if err := rows.Scan(&item.OperationID, &item.OperationName, &item.Description, &item.RequiresCrop, &item.ResourceType, &item.ResourceTitle); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) OperationRuleByID(ctx context.Context, id int64) (models.OperationRule, error) {
	var item models.OperationRule
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(description,''), COALESCE(requires_crop,true), COALESCE(resource_material_type,''), COALESCE(resource_title,'') FROM operations WHERE id=$1`, id).
		Scan(&item.OperationID, &item.OperationName, &item.Description, &item.RequiresCrop, &item.ResourceType, &item.ResourceTitle)
	if err == sql.ErrNoRows {
		return models.OperationRule{}, nil
	}
	return item, err
}

func (r *Repository) ListNorms(ctx context.Context, cropID, operationID int64) ([]models.Norm, error) {
	query := `
	SELECT n.id, n.crop_id, c.name, n.operation_id, o.name, n.material_id, m.name,
	       COALESCE(n.material_type, m.material_type, ''), n.rate, n.unit_id, u.short_name
	FROM norms n
	JOIN crops c ON c.id=n.crop_id
	JOIN operations o ON o.id=n.operation_id
	JOIN materials m ON m.id=n.material_id
	JOIN units u ON u.id=n.unit_id
	WHERE ($1::bigint=0 OR n.crop_id=$1) AND ($2::bigint=0 OR n.operation_id=$2)
	ORDER BY c.name, o.name, m.name`
	rows, err := r.DB.QueryContext(ctx, query, cropID, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Norm
	for rows.Next() {
		var item models.Norm
		if err := rows.Scan(&item.ID, &item.CropID, &item.CropName, &item.OperationID, &item.OperationName, &item.MaterialID, &item.MaterialName, &item.MaterialType, &item.Rate, &item.UnitID, &item.UnitName); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) NormFor(ctx context.Context, cropID, operationID int64, materialType string) (*models.Norm, error) {
	rows, err := r.ListNorms(ctx, cropID, operationID)
	if err != nil {
		return nil, err
	}
	for _, n := range rows {
		if strings.EqualFold(n.MaterialType, materialType) {
			return &n, nil
		}
	}
	return nil, nil
}

func (r *Repository) ListConditionCoefficients(ctx context.Context) ([]models.ConditionCoefficient, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, group_code, group_name, name, value, COALESCE(description,'') FROM condition_coefficients ORDER BY group_code, value, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.ConditionCoefficient
	for rows.Next() {
		var item models.ConditionCoefficient
		if err := rows.Scan(&item.ID, &item.GroupCode, &item.GroupName, &item.Name, &item.Value, &item.Description); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ConditionCoefficientsByIDs(ctx context.Context, ids []int64) ([]models.ConditionCoefficient, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	all, err := r.ListConditionCoefficients(ctx)
	if err != nil {
		return nil, err
	}
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var result []models.ConditionCoefficient
	for _, c := range all {
		if want[c.ID] {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *Repository) ListTechMapTemplates(ctx context.Context, cropID int64) ([]models.TechMapTemplate, error) {
	rows, err := r.DB.QueryContext(ctx, `
	SELECT t.id, t.crop_id, c.name, t.operation_id, o.name, t.sort_order, t.phase, t.is_required, t.area_factor
	FROM tech_map_templates t
	JOIN crops c ON c.id=t.crop_id
	JOIN operations o ON o.id=t.operation_id
	WHERE ($1::bigint=0 OR t.crop_id=$1)
	ORDER BY t.sort_order, o.name`, cropID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.TechMapTemplate
	for rows.Next() {
		var item models.TechMapTemplate
		if err := rows.Scan(&item.ID, &item.CropID, &item.CropName, &item.OperationID, &item.OperationName, &item.SortOrder, &item.Phase, &item.IsRequired, &item.AreaFactor); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) PrimaryOperationMachine(ctx context.Context, operationID int64) (models.DirectoryItem, error) {
	items, err := r.ListOperationMachines(ctx, operationID)
	if err != nil || len(items) == 0 {
		return models.DirectoryItem{}, err
	}
	return items[0], nil
}

func (r *Repository) MaterialByName(ctx context.Context, name string) (models.DirectoryItem, error) {
	var m models.DirectoryItem
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(material_type,''), COALESCE(default_price,0), 0::numeric, COALESCE(unit_id,0), ''::text, 0::numeric, 0::numeric FROM materials WHERE name=$1`, name).
		Scan(&m.ID, &m.Name, &m.Description, &m.DefaultPrice, &m.RentPrice, &m.UnitID, &m.Role, &m.Productivity, &m.FuelRate)
	if err == sql.ErrNoRows {
		return models.DirectoryItem{}, nil
	}
	return m, err
}

func (r *Repository) CostItemByName(ctx context.Context, name string) (models.DirectoryItem, error) {
	var c models.DirectoryItem
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, COALESCE(description,''), 0::numeric, 0::numeric, 0::bigint, ''::text, 0::numeric, 0::numeric FROM cost_items WHERE name=$1`, name).
		Scan(&c.ID, &c.Name, &c.Description, &c.DefaultPrice, &c.RentPrice, &c.UnitID, &c.Role, &c.Productivity, &c.FuelRate)
	if err == sql.ErrNoRows {
		return models.DirectoryItem{}, nil
	}
	return c, err
}

func (r *Repository) SaveCalculation(ctx context.Context, req models.CalculationRequest, result models.CalculationResult) (int64, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var calcID int64
	var ownTotal, rentTotal, delta any
	var cheaper any
	if result.Comparison != nil {
		ownTotal = result.Comparison.OwnTotal
		rentTotal = result.Comparison.RentTotal
		delta = result.Comparison.Delta
		cheaper = result.Comparison.CheaperUsageType
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO calculations(user_id, guest_id, crop_id, federal_district_id, region_id, area_ha, total_cost, cost_per_ha, calculation_mode, include_comparison, comparison_own_total, comparison_rent_total, comparison_delta, comparison_cheaper_usage_type, total_machine_hours)
		VALUES(NULLIF($1,0),NULLIF($2,''),$3,$4,NULLIF($5,0),$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id`,
		req.UserID, req.GuestID, req.CropID, req.FederalDistrictID, req.RegionID, req.AreaHa, result.TotalCost, result.CostPerHa, result.CalculationMode, result.IncludeComparison, ownTotal, rentTotal, delta, cheaper, result.TotalMachineHours).Scan(&calcID)
	if err != nil {
		return 0, err
	}
	for _, row := range result.Rows {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO calculation_rows(calculation_id, operation_id, material_id, machine_id, machine_usage_type, cost_item_id, price_snapshot_id, quantity, rate, price, coefficient, amount, machine_price_source, price_source, price_date, source_url, area_factor)
			VALUES($1,$2,$3,NULLIF($4,0),$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),NULLIF($15,'')::date,NULLIF($16,''),$17)`,
			calcID, row.OperationID, row.MaterialID, row.MachineID, row.MachineUsageType, row.CostItemID, row.PriceSnapshotID, row.Quantity, row.Rate, row.Price, row.Coefficient, row.Amount, row.MachinePriceSource, row.PriceSource, row.PriceDate, row.SourceURL, row.AreaFactor)
		if err != nil {
			return 0, err
		}
	}
	return calcID, tx.Commit()
}

func (r *Repository) ListCalculations(ctx context.Context, userID int64, guestID string) ([]models.CalculationResult, error) {
	query := `SELECT c.id, c.crop_id, cr.name, c.federal_district_id, fd.name, COALESCE(c.region_id,0), COALESCE(reg.name,''), c.area_ha, c.total_cost, c.cost_per_ha, c.created_at, COALESCE(c.calculation_mode,'single_operation'), COALESCE(c.include_comparison,false), COALESCE(c.total_machine_hours,0)
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
		if err := rows.Scan(&c.ID, &c.CropID, &c.CropName, &c.FederalDistrictID, &c.FederalDistrict, &c.RegionID, &c.RegionName, &c.AreaHa, &c.TotalCost, &c.CostPerHa, &c.CreatedAt, &c.CalculationMode, &c.IncludeComparison, &c.TotalMachineHours); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) GetCalculation(ctx context.Context, id, userID int64, guestID string) (models.CalculationResult, error) {
	query := `SELECT c.id, c.crop_id, cr.name, c.federal_district_id, fd.name, COALESCE(c.region_id,0), COALESCE(reg.name,''), c.area_ha, c.total_cost, c.cost_per_ha, c.created_at,
		       COALESCE(c.calculation_mode,'single_operation'), COALESCE(c.include_comparison,false), c.comparison_own_total, c.comparison_rent_total, c.comparison_delta, COALESCE(c.comparison_cheaper_usage_type,''), COALESCE(c.total_machine_hours,0)
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
	var ownTotal, rentTotal, delta sql.NullFloat64
	var cheaper string
	err := r.DB.QueryRowContext(ctx, query, args...).Scan(&c.ID, &c.CropID, &c.CropName, &c.FederalDistrictID, &c.FederalDistrict, &c.RegionID, &c.RegionName, &c.AreaHa, &c.TotalCost, &c.CostPerHa, &c.CreatedAt, &c.CalculationMode, &c.IncludeComparison, &ownTotal, &rentTotal, &delta, &cheaper, &c.TotalMachineHours)
	if err != nil {
		return c, err
	}
	if ownTotal.Valid && rentTotal.Valid && delta.Valid {
		c.Comparison = &models.CalculationComparison{OwnTotal: ownTotal.Float64, RentTotal: rentTotal.Float64, Delta: delta.Float64, CheaperUsageType: cheaper}
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT cr.operation_id, COALESCE(op.name,''), cr.material_id, COALESCE(mat.name,''),
		       COALESCE(cr.machine_id,0), COALESCE(m.name,''), COALESCE(cr.machine_usage_type,''),
		       cr.cost_item_id, COALESCE(ci.name,''), cr.price_snapshot_id, cr.quantity, cr.price, cr.rate, cr.coefficient, cr.amount, COALESCE(cr.machine_price_source,''),
		       COALESCE(cr.price_source,''), COALESCE(to_char(cr.price_date, 'YYYY-MM-DD'), ''), COALESCE(cr.source_url,''), COALESCE(cr.area_factor,1)
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
		if err := rows.Scan(&row.OperationID, &row.OperationName, &row.MaterialID, &row.MaterialName, &row.MachineID, &row.MachineName, &row.MachineUsageType, &row.CostItemID, &row.CostItemName, &row.PriceSnapshotID, &row.Quantity, &row.Price, &row.Rate, &row.Coefficient, &row.Amount, &row.MachinePriceSource, &row.PriceSource, &row.PriceDate, &row.SourceURL, &row.AreaFactor); err != nil {
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
