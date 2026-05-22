package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type FederalDistrict struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type Region struct {
	ID                int64  `json:"id"`
	FederalDistrictID int64  `json:"federal_district_id"`
	Name              string `json:"name"`
	Code              string `json:"code"`
}

type DirectoryItem struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	DefaultPrice float64 `json:"default_price,omitempty"`
	RentPrice    float64 `json:"rent_price,omitempty"`
	UnitID       int64   `json:"unit_id,omitempty"`
	Role         string  `json:"role,omitempty"`
	Productivity float64 `json:"productivity_ha_per_hour,omitempty"`
	FuelRate     float64 `json:"fuel_rate_l_per_ha,omitempty"`
}

type Unit struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

type PriceSource struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	Category   string `json:"category"`
	ParserType string `json:"parser_type"`
	URL        string `json:"url"`
	Note       string `json:"note"`
	Priority   int    `json:"priority"`
	IsActive   bool   `json:"is_active"`
}

type PriceSnapshot struct {
	ID                int64     `json:"id"`
	MaterialID        int64     `json:"material_id"`
	MaterialName      string    `json:"material_name,omitempty"`
	FederalDistrictID int64     `json:"federal_district_id"`
	FederalDistrict   string    `json:"federal_district,omitempty"`
	RegionID          int64     `json:"region_id"`
	RegionName        string    `json:"region_name,omitempty"`
	SourceID          int64     `json:"source_id"`
	SourceName        string    `json:"source_name,omitempty"`
	Price             float64   `json:"price"`
	UnitID            int64     `json:"unit_id"`
	UnitName          string    `json:"unit_name,omitempty"`
	EffectiveDate     time.Time `json:"effective_date"`
	ParsedAt          time.Time `json:"parsed_at"`
	Status            string    `json:"status"`
	SourceURL         string    `json:"source_url"`
}

type CalculationRequest struct {
	UserID            int64                 `json:"user_id"`
	GuestID           string                `json:"guest_id,omitempty"`
	CropID            int64                 `json:"crop_id"`
	FederalDistrictID int64                 `json:"federal_district_id"`
	RegionID          int64                 `json:"region_id"`
	AreaHa            float64               `json:"area_ha"`
	Rows              []CalculationInputRow `json:"rows"`
}

type CalculationInputRow struct {
	OperationID      int64    `json:"operation_id"`
	MaterialID       int64    `json:"material_id"`
	MachineID        int64    `json:"machine_id"`
	MachineUsageType string   `json:"machine_usage_type"`
	CostItemID       int64    `json:"cost_item_id"`
	Rate             float64  `json:"rate"`
	ManualPrice      *float64 `json:"manual_price,omitempty"`
	Coefficient      float64  `json:"coefficient"`
}

type CalculationResult struct {
	ID                int64                  `json:"id"`
	CropID            int64                  `json:"crop_id"`
	CropName          string                 `json:"crop_name,omitempty"`
	FederalDistrictID int64                  `json:"federal_district_id,omitempty"`
	FederalDistrict   string                 `json:"federal_district,omitempty"`
	RegionID          int64                  `json:"region_id"`
	RegionName        string                 `json:"region_name,omitempty"`
	AreaHa            float64                `json:"area_ha"`
	TotalCost         float64                `json:"total_cost"`
	CostPerHa         float64                `json:"cost_per_ha"`
	CreatedAt         time.Time              `json:"created_at"`
	Rows              []CalculationResultRow `json:"rows"`
}

type CalculationResultRow struct {
	OperationID        int64   `json:"operation_id"`
	OperationName      string  `json:"operation_name,omitempty"`
	MaterialID         int64   `json:"material_id"`
	MaterialName       string  `json:"material_name,omitempty"`
	MachineID          int64   `json:"machine_id,omitempty"`
	MachineName        string  `json:"machine_name,omitempty"`
	MachineUsageType   string  `json:"machine_usage_type,omitempty"`
	MachinePriceSource string  `json:"machine_price_source,omitempty"`
	CostItemID         int64   `json:"cost_item_id"`
	CostItemName       string  `json:"cost_item_name,omitempty"`
	PriceSnapshotID    *int64  `json:"price_snapshot_id,omitempty"`
	Quantity           float64 `json:"quantity"`
	Price              float64 `json:"price"`
	Rate               float64 `json:"rate"`
	Coefficient        float64 `json:"coefficient"`
	Amount             float64 `json:"amount"`
	PriceSource        string  `json:"price_source,omitempty"`
	PriceRegion        string  `json:"price_region,omitempty"`
}

type ParseRequest struct {
	SourceID          int64  `json:"source_id"`
	SourceURL         string `json:"source_url"`
	SourceName        string `json:"source_name"`
	SourceType        string `json:"source_type"`
	Category          string `json:"category"`
	ParserType        string `json:"parser_type"`
	FederalDistrictID int64  `json:"federal_district_id"`
	RegionID          int64  `json:"region_id"`
	MaterialKeyword   string `json:"material_keyword"`
	DefaultUnit       string `json:"default_unit"`
}

type ParseResult struct {
	SourceName string   `json:"source_name,omitempty"`
	ParserType string   `json:"parser_type,omitempty"`
	RowsFound  int      `json:"rows_found"`
	RowsSaved  int      `json:"rows_saved"`
	Errors     []string `json:"errors"`
}

type ParseAllResult struct {
	StartedAt      time.Time     `json:"started_at"`
	FinishedAt     time.Time     `json:"finished_at"`
	SourcesTotal   int           `json:"sources_total"`
	SourcesSuccess int           `json:"sources_success"`
	SourcesError   int           `json:"sources_error"`
	RowsFound      int           `json:"rows_found"`
	RowsSaved      int           `json:"rows_saved"`
	Results        []ParseResult `json:"results"`
	Errors         []string      `json:"errors"`
}
