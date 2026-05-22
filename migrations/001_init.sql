CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    full_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash TEXT,
    role VARCHAR(50) NOT NULL DEFAULT 'economist',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

CREATE TABLE IF NOT EXISTS units (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    short_name VARCHAR(20) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS federal_districts (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(20) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS regions (
    id SERIAL PRIMARY KEY,
    federal_district_id INT NOT NULL REFERENCES federal_districts(id),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    UNIQUE(federal_district_id, name)
);

CREATE TABLE IF NOT EXISTS crops (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS operations (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    unit_id INT REFERENCES units(id),
    description TEXT
);

CREATE TABLE IF NOT EXISTS machines (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    machine_type VARCHAR(100),
    cost_per_hour NUMERIC(12,2) DEFAULT 0,
    rent_cost_per_hour NUMERIC(12,2) DEFAULT 0
);

ALTER TABLE machines ADD COLUMN IF NOT EXISTS rent_cost_per_hour NUMERIC(12,2) DEFAULT 0;

CREATE TABLE IF NOT EXISTS operation_machines (
    id SERIAL PRIMARY KEY,
    operation_id INT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    machine_id INT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    role VARCHAR(100) NOT NULL DEFAULT 'основная техника',
    productivity_ha_per_hour NUMERIC(12,4) DEFAULT 1,
    fuel_rate_l_per_ha NUMERIC(12,4) DEFAULT 0,
    UNIQUE(operation_id, machine_id)
);

CREATE TABLE IF NOT EXISTS materials (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    material_type VARCHAR(100),
    unit_id INT REFERENCES units(id),
    default_price NUMERIC(12,2) DEFAULT 0
);

CREATE TABLE IF NOT EXISTS cost_items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE IF NOT EXISTS norms (
    id SERIAL PRIMARY KEY,
    crop_id INT NOT NULL REFERENCES crops(id),
    operation_id INT NOT NULL REFERENCES operations(id),
    material_id INT NOT NULL REFERENCES materials(id),
    rate NUMERIC(12,4) NOT NULL CHECK(rate >= 0),
    unit_id INT NOT NULL REFERENCES units(id),
    UNIQUE(crop_id, operation_id, material_id)
);

CREATE TABLE IF NOT EXISTS price_sources (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    source_type VARCHAR(100) NOT NULL DEFAULT 'external',
    category VARCHAR(100) NOT NULL DEFAULT 'mixed',
    parser_type VARCHAR(50) NOT NULL DEFAULT 'auto',
    url TEXT,
    note TEXT,
    priority INT NOT NULL DEFAULT 50,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS price_snapshots (
    id SERIAL PRIMARY KEY,
    material_id INT NOT NULL REFERENCES materials(id),
    federal_district_id INT NOT NULL REFERENCES federal_districts(id),
    region_id INT REFERENCES regions(id),
    source_id INT NOT NULL REFERENCES price_sources(id),
    price NUMERIC(12,2) NOT NULL CHECK(price >= 0),
    unit_id INT NOT NULL REFERENCES units(id),
    effective_date DATE NOT NULL DEFAULT CURRENT_DATE,
    parsed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'new',
    source_url TEXT
);

CREATE INDEX IF NOT EXISTS idx_price_lookup ON price_snapshots(material_id, region_id, federal_district_id, effective_date DESC, parsed_at DESC);

CREATE TABLE IF NOT EXISTS parser_logs (
    id SERIAL PRIMARY KEY,
    source_id INT REFERENCES price_sources(id),
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    rows_found INT NOT NULL DEFAULT 0,
    rows_saved INT NOT NULL DEFAULT 0,
    error_message TEXT
);

CREATE TABLE IF NOT EXISTS calculations (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    crop_id INT NOT NULL REFERENCES crops(id),
    federal_district_id INT NOT NULL REFERENCES federal_districts(id),
    region_id INT REFERENCES regions(id),
    area_ha NUMERIC(12,2) NOT NULL CHECK(area_ha > 0),
    total_cost NUMERIC(14,2) NOT NULL CHECK(total_cost >= 0),
    cost_per_ha NUMERIC(14,2) NOT NULL CHECK(cost_per_ha >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE calculations ADD COLUMN IF NOT EXISTS guest_id TEXT;
ALTER TABLE calculations ALTER COLUMN region_id DROP NOT NULL;
CREATE INDEX IF NOT EXISTS idx_calculations_guest_id ON calculations(guest_id);

CREATE TABLE IF NOT EXISTS calculation_rows (
    id SERIAL PRIMARY KEY,
    calculation_id INT NOT NULL REFERENCES calculations(id) ON DELETE CASCADE,
    operation_id INT NOT NULL REFERENCES operations(id),
    material_id INT NOT NULL REFERENCES materials(id),
    machine_id INT REFERENCES machines(id),
    machine_usage_type VARCHAR(20) NOT NULL DEFAULT 'own',
    cost_item_id INT NOT NULL REFERENCES cost_items(id),
    price_snapshot_id INT REFERENCES price_snapshots(id),
    quantity NUMERIC(14,4) NOT NULL CHECK(quantity >= 0),
    rate NUMERIC(12,4) NOT NULL CHECK(rate >= 0),
    price NUMERIC(12,2) NOT NULL CHECK(price >= 0),
    coefficient NUMERIC(10,4) NOT NULL DEFAULT 1 CHECK(coefficient >= 0),
    amount NUMERIC(14,2) NOT NULL CHECK(amount >= 0)
);

ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS machine_usage_type VARCHAR(20) NOT NULL DEFAULT 'own';
ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS machine_price_source VARCHAR(100) DEFAULT '';

INSERT INTO units(name, short_name) VALUES
('литр', 'л'), ('килограмм', 'кг'), ('тонна', 'т'), ('гектар', 'га'), ('час', 'ч')
ON CONFLICT(short_name) DO NOTHING;

INSERT INTO federal_districts(name, code) VALUES
('Центральный федеральный округ', 'ЦФО'),
('Южный федеральный округ', 'ЮФО'),
('Северо-Кавказский федеральный округ', 'СКФО'),
('Приволжский федеральный округ', 'ПФО'),
('Уральский федеральный округ', 'УФО'),
('Сибирский федеральный округ', 'СФО'),
('Дальневосточный федеральный округ', 'ДФО'),
('Северо-Западный федеральный округ', 'СЗФО'),
('Новые регионы / исторические территории', 'НР')
ON CONFLICT(code) DO NOTHING;

INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Белгородская область', '01' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Брянская область', '02' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Владимирская область', '03' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Воронежская область', '04' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ивановская область', '05' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Калужская область', '06' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Костромская область', '07' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Курская область', '08' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Липецкая область', '09' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Московская область', '10' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Орловская область', '11' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Рязанская область', '12' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Смоленская область', '13' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Тамбовская область', '14' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Тверская область', '15' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Тульская область', '16' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ярославская область', '17' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'город Москва', '18' FROM federal_districts WHERE code='ЦФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Карелия', '19' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Коми', '20' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Архангельская область', '21' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Вологодская область', '22' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Калининградская область', '23' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ленинградская область', '24' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Мурманская область', '25' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Новгородская область', '26' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Псковская область', '27' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'город Санкт-Петербург', '28' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ненецкий автономный округ', '29' FROM federal_districts WHERE code='СЗФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Адыгея', '30' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Калмыкия', '31' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Крым', '32' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Краснодарский край', '33' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Астраханская область', '34' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Волгоградская область', '35' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ростовская область', '36' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'город Севастополь', '37' FROM federal_districts WHERE code='ЮФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Дагестан', '38' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Ингушетия', '39' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Кабардино-Балкарская Республика', '40' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Карачаево-Черкесская Республика', '41' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Северная Осетия – Алания', '42' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Чеченская Республика', '43' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ставропольский край', '44' FROM federal_districts WHERE code='СКФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Башкортостан', '45' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Марий Эл', '46' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Мордовия', '47' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Татарстан', '48' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Удмуртская Республика', '49' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Чувашская Республика', '50' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Пермский край', '51' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Кировская область', '52' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Нижегородская область', '53' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Оренбургская область', '54' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Пензенская область', '55' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Самарская область', '56' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Саратовская область', '57' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ульяновская область', '58' FROM federal_districts WHERE code='ПФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Курганская область', '59' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Свердловская область', '60' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Тюменская область', '61' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Челябинская область', '62' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ханты-Мансийский автономный округ – Югра', '63' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Ямало-Ненецкий автономный округ', '64' FROM federal_districts WHERE code='УФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Алтай', '65' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Тыва', '66' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Хакасия', '67' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Алтайский край', '68' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Красноярский край', '69' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Иркутская область', '70' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Кемеровская область – Кузбасс', '71' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Новосибирская область', '72' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Омская область', '73' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Томская область', '74' FROM federal_districts WHERE code='СФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Бурятия', '75' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Республика Саха (Якутия)', '76' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Забайкальский край', '77' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Камчатский край', '78' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Приморский край', '79' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Хабаровский край', '80' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Амурская область', '81' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Магаданская область', '82' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Сахалинская область', '83' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Еврейская автономная область', '84' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Чукотский автономный округ', '85' FROM federal_districts WHERE code='ДФО'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Донецкая Народная Республика', '86' FROM federal_districts WHERE code='НР'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Луганская Народная Республика', '87' FROM federal_districts WHERE code='НР'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Запорожская область', '88' FROM federal_districts WHERE code='НР'
ON CONFLICT(federal_district_id, name) DO NOTHING;
INSERT INTO regions(federal_district_id, name, code)
SELECT id, 'Херсонская область', '89' FROM federal_districts WHERE code='НР'
ON CONFLICT(federal_district_id, name) DO NOTHING;

INSERT INTO crops(name, description) VALUES
('Пшеница', 'Зерновая культура; включает озимую и яровую технологию'),
('Ячмень', 'Зерновая культура'),
('Подсолнечник', 'Масличная культура'),
('Кукуруза', 'Зерновая и кормовая культура'),
('Соя', 'Зернобобовая культура'),
('Горох', 'Зернобобовая культура'),
('Рапс', 'Масличная культура'),
('Овёс', 'Зерновая культура'),
('Сахарная свёкла', 'Техническая культура')
ON CONFLICT(name) DO NOTHING;

INSERT INTO operations(name, unit_id, description)
SELECT 'Вспашка', id, 'Основная обработка почвы' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Культивация', id, 'Предпосевная обработка' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Посев', id, 'Посев сельскохозяйственной культуры' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Внесение удобрений', id, 'Внесение минеральных удобрений' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Обработка СЗР', id, 'Защита растений' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;

INSERT INTO operations(name, unit_id, description)
SELECT 'Лущение стерни', id, 'Поверхностная обработка почвы после уборки' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Боронование', id, 'Закрытие влаги и выравнивание поверхности поля' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Прикатывание', id, 'Уплотнение почвы после посева' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Междурядная обработка', id, 'Уход за пропашными культурами' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Уборка урожая', id, 'Комбайновая уборка культуры' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;
INSERT INTO operations(name, unit_id, description)
SELECT 'Транспортировка урожая', id, 'Перевозка урожая от поля до склада/тока' FROM units WHERE short_name='га'
ON CONFLICT(name) DO NOTHING;

INSERT INTO machines(name, machine_type, cost_per_hour, rent_cost_per_hour) VALUES
('Трактор МТЗ-82', 'трактор', 1200, 1800),
('Трактор К-744', 'трактор', 2600, 3800),
('Плуг ПЛН-5-35', 'почвообрабатывающий агрегат', 500, 900),
('Культиватор КПС-4', 'почвообрабатывающий агрегат', 600, 950),
('Дискатор БДМ', 'почвообрабатывающий агрегат', 700, 1100),
('Борона зубовая', 'почвообрабатывающий агрегат', 350, 650),
('Каток кольчато-шпоровый', 'почвообрабатывающий агрегат', 300, 600),
('Сеялка СЗ-3,6', 'посевной агрегат', 700, 1200),
('Сеялка точного высева', 'посевной агрегат', 900, 1600),
('Разбрасыватель удобрений', 'агрегат для внесения удобрений', 650, 1200),
('Опрыскиватель', 'агрегат для защиты растений', 800, 1500),
('Комбайн зерноуборочный', 'уборочная техника', 3500, 5200),
('Грузовой автомобиль', 'транспорт', 1400, 2200)
ON CONFLICT(name) DO UPDATE SET machine_type=EXCLUDED.machine_type, cost_per_hour=EXCLUDED.cost_per_hour, rent_cost_per_hour=EXCLUDED.rent_cost_per_hour;

-- Связи агротехнических операций и рекомендуемой техники.
INSERT INTO operation_machines(operation_id, machine_id, role, productivity_ha_per_hour, fuel_rate_l_per_ha)
SELECT o.id, m.id, v.role, v.prod, v.fuel
FROM (VALUES
    ('Вспашка','Трактор К-744','энергетическое средство',1.20,25.0),
    ('Вспашка','Плуг ПЛН-5-35','рабочий агрегат',1.20,0.0),
    ('Лущение стерни','Трактор МТЗ-82','энергетическое средство',3.00,10.0),
    ('Лущение стерни','Дискатор БДМ','рабочий агрегат',3.00,0.0),
    ('Культивация','Трактор МТЗ-82','энергетическое средство',2.50,12.0),
    ('Культивация','Культиватор КПС-4','рабочий агрегат',2.50,0.0),
    ('Боронование','Трактор МТЗ-82','энергетическое средство',4.00,6.0),
    ('Боронование','Борона зубовая','рабочий агрегат',4.00,0.0),
    ('Посев','Трактор МТЗ-82','энергетическое средство',2.00,8.0),
    ('Посев','Сеялка СЗ-3,6','посевной агрегат',2.00,0.0),
    ('Посев','Сеялка точного высева','посевной агрегат',1.70,0.0),
    ('Внесение удобрений','Трактор МТЗ-82','энергетическое средство',3.50,7.0),
    ('Внесение удобрений','Разбрасыватель удобрений','агрегат',3.50,0.0),
    ('Обработка СЗР','Трактор МТЗ-82','энергетическое средство',8.00,4.0),
    ('Обработка СЗР','Опрыскиватель','агрегат',8.00,0.0),
    ('Прикатывание','Трактор МТЗ-82','энергетическое средство',4.00,5.0),
    ('Прикатывание','Каток кольчато-шпоровый','рабочий агрегат',4.00,0.0),
    ('Междурядная обработка','Трактор МТЗ-82','энергетическое средство',2.50,9.0),
    ('Междурядная обработка','Культиватор КПС-4','рабочий агрегат',2.50,0.0),
    ('Уборка урожая','Комбайн зерноуборочный','уборочная техника',1.80,18.0),
    ('Транспортировка урожая','Грузовой автомобиль','транспорт',5.00,0.0)
) AS v(operation_name, machine_name, role, prod, fuel)
JOIN operations o ON o.name=v.operation_name
JOIN machines m ON m.name=v.machine_name
ON CONFLICT(operation_id, machine_id) DO UPDATE SET role=EXCLUDED.role, productivity_ha_per_hour=EXCLUDED.productivity_ha_per_hour, fuel_rate_l_per_ha=EXCLUDED.fuel_rate_l_per_ha;

INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Дизельное топливо', 'fuel', id, 65 FROM units WHERE short_name='л'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена пшеницы', 'seed', id, 35 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Аммиачная селитра', 'fertilizer', id, 32 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Гербицид', 'pesticide', id, 900 FROM units WHERE short_name='л'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Труд механизатора', 'labor', id, 450 FROM units WHERE short_name='ч'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Машино-час', 'machinery', id, 1200 FROM units WHERE short_name='ч'
ON CONFLICT(name) DO NOTHING;

INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Аренда техники', 'machinery_rent', id, 1800 FROM units WHERE short_name='ч'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена ячменя', 'seed', id, 32 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена подсолнечника', 'seed', id, 185 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена кукурузы', 'seed', id, 210 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена сои', 'seed', id, 95 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена гороха', 'seed', id, 55 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена рапса', 'seed', id, 240 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена овса', 'seed', id, 30 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Семена сахарной свёклы', 'seed', id, 520 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Карбамид', 'fertilizer', id, 42 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Азофоска NPK', 'fertilizer', id, 45 FROM units WHERE short_name='кг'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Фунгицид', 'pesticide', id, 1300 FROM units WHERE short_name='л'
ON CONFLICT(name) DO NOTHING;
INSERT INTO materials(name, material_type, unit_id, default_price)
SELECT 'Инсектицид', 'pesticide', id, 1100 FROM units WHERE short_name='л'
ON CONFLICT(name) DO NOTHING;

INSERT INTO cost_items(name, description) VALUES
('Топливо', 'Затраты на ГСМ'), ('Семена', 'Затраты на семенной материал'), ('Удобрения', 'Затраты на удобрения'), ('Средства защиты растений', 'Затраты на СЗР'), ('Эксплуатация техники', 'Машино-часы и обслуживание'), ('Оплата труда', 'Трудовые затраты')
ON CONFLICT(name) DO NOTHING;

INSERT INTO price_sources(name, source_type, category, parser_type, url, note, priority, is_active) VALUES
('Демонстрационный CSV', 'local_csv', 'mixed', 'csv', 'samples/prices_yufo.csv', 'Локальный демонстрационный файл для проверки импорта', 10, true),
('СПбМТСБ — территориальные индексы нефтепродуктов', 'exchange', 'fuel', 'spimex_petroleum', 'https://spimex.com/indexes/petroleum/territorial/', 'Территориальные индексы нефтепродуктов СПбМТСБ; используется цена DTL в рублях за литр', 1, true),
('ЕМИСС / Росстат — региональные показатели', 'official_stats', 'fuel_labor', 'json', 'https://www.fedstat.ru/', 'Официальная статистика: потребительские цены, индексы цен, зарплатные показатели; при наличии API используется JSON-парсер', 30, true),
('Agroserver — коммерческие предложения', 'marketplace', 'seed_fertilizer_pesticide_machinery', 'html_regex', 'https://agroserver.ru/', 'Аграрная торговая площадка для семян, удобрений, СЗР, техники и услуг; HTML-парсер требует настройки под конкретную страницу категории', 40, true),
('Поле.рф — аграрный маркетплейс', 'marketplace', 'seed_fertilizer_pesticide', 'html_regex', 'https://поле.рф/', 'Маркетплейс аграрных товаров; HTML-парсер используется как шаблон, если страница доступна без авторизации', 45, true),
('ФГИС Сатурн — справочная проверка СЗР', 'registry', 'pesticide', 'html_regex', 'https://saturn.fsvps.ru/', 'Используется не как источник цены, а как контроль легальности/прослеживаемости пестицидов и агрохимикатов', 60, false),
('Benzup API — цены топлива', 'api', 'fuel', 'benzup_fuel_api', 'api:benzup', 'Коммерческий API мониторинга цен на АЗС. Для включения задайте BENZUP_API_TOKEN и BENZUP_API_URL_TEMPLATE', 2, false),
('Benzup — средние цены топлива по регионам', 'internet', 'fuel', 'benzup_index_region', 'https://benzup.ru/index-region', 'Открытая таблица средних цен топлива Benzup по регионам. Парсер берёт колонку ДТ и усредняет значения по федеральным округам', 1, true),
('MultiGO API — средняя цена топлива', 'api', 'fuel', 'multigo_fuel_api', 'api:multigo', 'API средних цен топлива по региону. Для включения задайте MULTIGO_API_URL_TEMPLATE и при необходимости MULTIGO_API_TOKEN', 3, false)
ON CONFLICT(name) DO NOTHING;

-- Интернет-парсинг: основной стабильный источник топлива — территориальные индексы СПбМТСБ.
-- Agroserver и HH оставлены в реестре, но по умолчанию выключены, так как часто блокируют serverless-запросы.
UPDATE price_sources SET url='builtin:regional_prices', parser_type='csv', priority=99, is_active=true, note='Резервная ценовая база на случай недоступности интернет-источников' WHERE name='Демонстрационный CSV';

UPDATE price_sources SET url='https://benzup.ru/index-region', parser_type='benzup_index_region', priority=1, is_active=true, note='Открытая таблица средних цен топлива Benzup по регионам: колонка ДТ, руб./л. Значения субъектов усредняются по федеральным округам' WHERE name='Benzup — средние цены топлива по регионам';
UPDATE price_sources SET is_active=false, note='Источник отключён по умолчанию: с Vercel соединение часто отклоняется. Вместо него используется Benzup index-region' WHERE name='СПбМТСБ — территориальные индексы нефтепродуктов';
UPDATE price_sources SET is_active=false WHERE name='СПбМТСБ — нефтепродукты и удобрения';
UPDATE price_sources SET url='internet:agroserver_bundle', parser_type='agroserver_bundle', priority=20, is_active=false, note='Дополнительный источник коммерческих предложений. Может блокировать serverless-запросы, поэтому выключен по умолчанию' WHERE name='Agroserver — коммерческие предложения';
UPDATE price_sources SET is_active=false, note='Официальная статистика оставлена как справочный источник; автоматический API-парсер требует отдельного набора показателей ЕМИСС' WHERE name='ЕМИСС / Росстат — региональные показатели';
UPDATE price_sources SET is_active=false WHERE name IN ('Поле.рф — аграрный маркетплейс','ФГИС Сатурн — справочная проверка СЗР');
UPDATE price_sources SET is_active=false, note='СПбМТСБ оставлен в реестре, но выключен по умолчанию: с Vercel соединение может отклоняться источником. Для стабильной работы используйте Benzup/MultiGO API или отдельный worker' WHERE name='СПбМТСБ — территориальные индексы нефтепродуктов';
UPDATE price_sources SET url='api:benzup', parser_type='benzup_fuel_api', priority=2, is_active=false, note='Коммерческий API мониторинга цен на АЗС. Включите источник после добавления BENZUP_API_TOKEN и BENZUP_API_URL_TEMPLATE в Vercel' WHERE name='Benzup API — цены топлива';
UPDATE price_sources SET url='api:multigo', parser_type='multigo_fuel_api', priority=3, is_active=false, note='API средних цен топлива по региону. Включите источник после добавления MULTIGO_API_URL_TEMPLATE и, если требуется, MULTIGO_API_TOKEN' WHERE name='MultiGO API — средняя цена топлива';

INSERT INTO price_sources(name, source_type, category, parser_type, url, note, priority, is_active)
VALUES ('HeadHunter — зарплаты механизатора', 'api', 'labor', 'hh_salary', 'internet:hh_mechanizator', 'Открытый API HeadHunter: медианная зарплата вакансий механизатора пересчитывается в руб./ч. Токен не требуется, нужен корректный HH_USER_AGENT', 10, true)
ON CONFLICT(name) DO UPDATE SET parser_type=EXCLUDED.parser_type, url=EXCLUDED.url, note=EXCLUDED.note, priority=EXCLUDED.priority, is_active=EXCLUDED.is_active);


-- Демонстрационные цены по федеральным округам, чтобы расчёт работал сразу после деплоя без выбора конкретного региона.
INSERT INTO price_snapshots(material_id, federal_district_id, region_id, source_id, price, unit_id, effective_date, status, source_url)
SELECT m.id, fd.id, NULL, ps.id, ROUND((v.price * fdv.factor)::numeric, 2), u.id, CURRENT_DATE, 'validated', 'initial_seed_fd'
FROM (VALUES
    ('Дизельное топливо', 68.40, 'л'),
    ('Семена пшеницы', 37.50, 'кг'),
    ('Аммиачная селитра', 32.80, 'кг'),
    ('Гербицид', 920.00, 'л'),
    ('Труд механизатора', 450.00, 'ч'),
    ('Машино-час', 1200.00, 'ч'),
    ('Аренда техники', 1800.00, 'ч'),
    ('Семена ячменя', 32.00, 'кг'),
    ('Семена подсолнечника', 185.00, 'кг'),
    ('Семена кукурузы', 210.00, 'кг'),
    ('Семена сои', 95.00, 'кг'),
    ('Семена гороха', 55.00, 'кг'),
    ('Семена рапса', 240.00, 'кг'),
    ('Семена овса', 30.00, 'кг'),
    ('Семена сахарной свёклы', 520.00, 'кг'),
    ('Карбамид', 42.00, 'кг'),
    ('Азофоска NPK', 45.00, 'кг'),
    ('Фунгицид', 1300.00, 'л'),
    ('Инсектицид', 1100.00, 'л')
) AS v(material, price, unit_code)
CROSS JOIN (VALUES
    ('ЦФО', 1.03::numeric),
    ('ЮФО', 1.00::numeric),
    ('СКФО', 0.98::numeric),
    ('ПФО', 0.99::numeric),
    ('СЗФО', 1.05::numeric),
    ('УФО', 1.04::numeric),
    ('СФО', 1.06::numeric),
    ('ДФО', 1.12::numeric),
    ('НР', 1.07::numeric)
) AS fdv(code, factor)
JOIN federal_districts fd ON fd.code = fdv.code
JOIN materials m ON m.name = v.material
JOIN units u ON u.short_name = v.unit_code
JOIN price_sources ps ON ps.name = 'Демонстрационный CSV'
WHERE NOT EXISTS (
    SELECT 1 FROM price_snapshots existing
    WHERE existing.material_id=m.id AND existing.federal_district_id=fd.id AND existing.region_id IS NULL AND existing.source_id=ps.id
);

-- Расчётные правила, нормы и технологические карты.
ALTER TABLE operations ADD COLUMN IF NOT EXISTS requires_crop BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_material_type VARCHAR(100);
ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_title VARCHAR(255);

UPDATE operations SET requires_crop=false, resource_material_type=NULL, resource_title=NULL
WHERE name IN ('Вспашка','Культивация','Лущение стерни','Боронование','Прикатывание','Междурядная обработка','Транспортировка урожая');
UPDATE operations SET requires_crop=true, resource_material_type=NULL, resource_title=NULL
WHERE name IN ('Уборка урожая');
UPDATE operations SET requires_crop=true, resource_material_type='seed', resource_title='Семенной материал'
WHERE name='Посев';
UPDATE operations SET requires_crop=true, resource_material_type='fertilizer', resource_title='Удобрение'
WHERE name='Внесение удобрений';
UPDATE operations SET requires_crop=true, resource_material_type='pesticide', resource_title='Средство защиты растений'
WHERE name='Обработка СЗР';

ALTER TABLE norms ADD COLUMN IF NOT EXISTS material_type VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_norms_lookup ON norms(crop_id, operation_id, material_type);

CREATE TABLE IF NOT EXISTS condition_coefficients (
    id SERIAL PRIMARY KEY,
    group_code VARCHAR(50) NOT NULL,
    group_name VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    value NUMERIC(10,4) NOT NULL CHECK(value > 0),
    description TEXT,
    UNIQUE(group_code, name)
);

CREATE TABLE IF NOT EXISTS tech_map_templates (
    id SERIAL PRIMARY KEY,
    crop_id INT NOT NULL REFERENCES crops(id) ON DELETE CASCADE,
    operation_id INT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    phase VARCHAR(100) NOT NULL DEFAULT '',
    is_required BOOLEAN NOT NULL DEFAULT TRUE,
    area_factor NUMERIC(10,4) NOT NULL DEFAULT 1 CHECK(area_factor >= 0),
    UNIQUE(crop_id, operation_id)
);

ALTER TABLE calculations ADD COLUMN IF NOT EXISTS calculation_mode VARCHAR(30) NOT NULL DEFAULT 'single_operation';
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS include_comparison BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_own_total NUMERIC(14,2);
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_rent_total NUMERIC(14,2);
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_delta NUMERIC(14,2);
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS comparison_cheaper_usage_type VARCHAR(20);
ALTER TABLE calculations ADD COLUMN IF NOT EXISTS total_machine_hours NUMERIC(14,4) NOT NULL DEFAULT 0;

ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS price_source VARCHAR(255);
ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS price_date DATE;
ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE calculation_rows ADD COLUMN IF NOT EXISTS area_factor NUMERIC(10,4) NOT NULL DEFAULT 1;

INSERT INTO cost_items(name, description) VALUES
('Амортизация', 'Доля стоимости собственной техники'),
('Ремонт', 'Ремонт собственной техники'),
('Техническое обслуживание', 'ТО собственной техники')
ON CONFLICT(name) DO NOTHING;

INSERT INTO condition_coefficients(group_code, group_name, name, value, description) VALUES
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
ON CONFLICT(group_code, name) DO UPDATE SET group_name=EXCLUDED.group_name, value=EXCLUDED.value, description=EXCLUDED.description;

INSERT INTO norms(crop_id, operation_id, material_id, rate, unit_id, material_type)
SELECT c.id, o.id, m.id,
       CASE
           WHEN m.name='Семена пшеницы' THEN 180
           WHEN m.name='Семена ячменя' THEN 170
           WHEN m.name='Семена подсолнечника' THEN 7
           WHEN m.name='Семена кукурузы' THEN 25
           WHEN m.name='Семена сои' THEN 75
           WHEN m.name='Семена гороха' THEN 220
           WHEN m.name='Семена рапса' THEN 6
           WHEN m.name='Семена овса' THEN 160
           WHEN m.name='Семена сахарной свёклы' THEN 4
           WHEN m.name='Аммиачная селитра' THEN 100
           WHEN m.name='Гербицид' THEN 1.2
           ELSE 0
       END,
       m.unit_id,
       m.material_type
FROM (VALUES
    ('Пшеница','Посев','Семена пшеницы'),
    ('Ячмень','Посев','Семена ячменя'),
    ('Подсолнечник','Посев','Семена подсолнечника'),
    ('Кукуруза','Посев','Семена кукурузы'),
    ('Соя','Посев','Семена сои'),
    ('Горох','Посев','Семена гороха'),
    ('Рапс','Посев','Семена рапса'),
    ('Овёс','Посев','Семена овса'),
    ('Сахарная свёкла','Посев','Семена сахарной свёклы'),
    ('Пшеница','Внесение удобрений','Аммиачная селитра'),
    ('Ячмень','Внесение удобрений','Аммиачная селитра'),
    ('Подсолнечник','Внесение удобрений','Аммиачная селитра'),
    ('Кукуруза','Внесение удобрений','Аммиачная селитра'),
    ('Соя','Внесение удобрений','Аммиачная селитра'),
    ('Горох','Внесение удобрений','Аммиачная селитра'),
    ('Рапс','Внесение удобрений','Аммиачная селитра'),
    ('Овёс','Внесение удобрений','Аммиачная селитра'),
    ('Сахарная свёкла','Внесение удобрений','Аммиачная селитра'),
    ('Пшеница','Обработка СЗР','Гербицид'),
    ('Ячмень','Обработка СЗР','Гербицид'),
    ('Подсолнечник','Обработка СЗР','Гербицид'),
    ('Кукуруза','Обработка СЗР','Гербицид'),
    ('Соя','Обработка СЗР','Гербицид'),
    ('Горох','Обработка СЗР','Гербицид'),
    ('Рапс','Обработка СЗР','Гербицид'),
    ('Овёс','Обработка СЗР','Гербицид'),
    ('Сахарная свёкла','Обработка СЗР','Гербицид')
) AS v(crop_name, operation_name, material_name)
JOIN crops c ON c.name=v.crop_name
JOIN operations o ON o.name=v.operation_name
JOIN materials m ON m.name=v.material_name
ON CONFLICT(crop_id, operation_id, material_id) DO UPDATE SET rate=EXCLUDED.rate, unit_id=EXCLUDED.unit_id, material_type=EXCLUDED.material_type;

INSERT INTO tech_map_templates(crop_id, operation_id, sort_order, phase, is_required, area_factor)
SELECT c.id, o.id, v.sort_order, v.phase, v.is_required, v.area_factor
FROM crops c
JOIN (VALUES
    ('Лущение стерни',10,'Послеуборочная обработка',false,1.0::numeric),
    ('Вспашка',20,'Основная обработка почвы',true,1.0::numeric),
    ('Боронование',30,'Закрытие влаги',false,1.0::numeric),
    ('Культивация',40,'Предпосевная подготовка',true,1.0::numeric),
    ('Внесение удобрений',50,'Питание',true,1.0::numeric),
    ('Посев',60,'Посев',true,1.0::numeric),
    ('Прикатывание',70,'После посева',false,1.0::numeric),
    ('Обработка СЗР',80,'Защита растений',true,1.0::numeric),
    ('Междурядная обработка',90,'Уход за посевами',false,1.0::numeric),
    ('Уборка урожая',100,'Уборка',true,1.0::numeric),
    ('Транспортировка урожая',110,'Логистика',false,1.0::numeric)
) AS v(operation_name, sort_order, phase, is_required, area_factor) ON true
JOIN operations o ON o.name=v.operation_name
WHERE c.name IN ('Пшеница','Ячмень','Кукуруза','Подсолнечник','Соя','Горох','Рапс','Овёс','Сахарная свёкла')
ON CONFLICT(crop_id, operation_id) DO UPDATE SET sort_order=EXCLUDED.sort_order, phase=EXCLUDED.phase, is_required=EXCLUDED.is_required, area_factor=EXCLUDED.area_factor;
