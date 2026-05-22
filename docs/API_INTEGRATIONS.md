# Интеграции API цен

В проект добавлены API-источники, чтобы не зависеть только от HTML-парсинга сайтов, которые могут блокировать Vercel.

## 1. Benzup API — цена дизельного топлива

Назначение: получение цены дизельного топлива по федеральному округу в формате JSON.

Переменные окружения Vercel:

```env
BENZUP_API_TOKEN=ваш_токен
BENZUP_API_URL_TEMPLATE=https://адрес-api-benzup/...?region={region}&fuel={fuel}
BENZUP_FUEL_CODE=diesel
```

Если `BENZUP_API_TOKEN` и `BENZUP_API_URL_TEMPLATE` заданы, источник автоматически добавляется в запуск `/api/cron/parse-prices` даже если в БД он выключен.

## 2. MultiGO API — средняя цена топлива

Назначение: получение средней цены топлива по федеральному округу.

Переменные окружения Vercel:

```env
MULTIGO_API_URL_TEMPLATE=https://адрес-api-multigo/...?region={region}&fuel={fuel}
MULTIGO_API_TOKEN=ваш_токен_если_требуется
MULTIGO_FUEL_CODE=dt
```

Если `MULTIGO_API_URL_TEMPLATE` задан, источник автоматически добавляется в запуск `/api/cron/parse-prices`.

## 3. HeadHunter API — ставка труда механизатора

Назначение: получение вакансий с зарплатой и пересчёт медианной зарплаты в руб./ч.

Переменные окружения:

```env
HH_AREA_ID=24
HH_QUERY=механизатор тракторист машинист сельскохозяйственный
HH_USER_AGENT=AgroCalc/1.0 (contact: your-real-email@example.ru)
```

Если переменные не заданы, используются значения по умолчанию для Волгограда.

## 4. Резервный источник

Если API недоступны или ключи не заданы, приложение использует резервную региональную ценовую базу `builtin:regional_prices`. Это нужно, чтобы расчёт не ломался при сбое внешнего источника.

## Проверка

Откройте:

```text
https://ваш-домен.vercel.app/api/cron/parse-prices
```

Если API работает, в ответе появится источник `Benzup API — цены топлива`, `MultiGO API — средняя цена топлива` или `HeadHunter — зарплаты механизатора` с `rows_saved > 0`.
