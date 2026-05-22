package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agro-cost-app/pkg/models"
	"agro-cost-app/pkg/repository"
)

type CalculationService struct{ Repo *repository.Repository }

func NewCalculationService(repo *repository.Repository) *CalculationService {
	return &CalculationService{Repo: repo}
}

func MultiplyCoefficients(items []models.ConditionCoefficient) float64 {
	result := 1.0
	for _, item := range items {
		if item.Value > 0 {
			result *= item.Value
		}
	}
	return result
}

func MachineOwnBreakdown(costPerHour float64) map[string]float64 {
	return map[string]float64{
		"Амортизация": costPerHour * 0.50,
		"Ремонт":      costPerHour * 0.30,
		"Техническое обслуживание": costPerHour * 0.20,
	}
}

func (s *CalculationService) BuildDraft(ctx context.Context, req models.CalculationDraftRequest) (models.CalculationDraftResult, error) {
	if req.CropID <= 0 {
		return models.CalculationDraftResult{}, fmt.Errorf("выберите культуру")
	}
	if req.AreaHa <= 0 {
		return models.CalculationDraftResult{}, fmt.Errorf("площадь должна быть больше нуля")
	}
	coeffs, err := s.Repo.ConditionCoefficientsByIDs(ctx, req.ConditionCoefficientIDs)
	if err != nil {
		return models.CalculationDraftResult{}, err
	}
	coef := MultiplyCoefficients(coeffs)
	if coef == 0 {
		coef = 1
	}
	mode := strings.TrimSpace(req.CalculationMode)
	if mode == "" {
		mode = "single_operation"
	}
	result := models.CalculationDraftResult{ConditionCoefficient: coef, ConditionSummary: coeffs}
	if mode == "tech_map" {
		templates, err := s.Repo.ListTechMapTemplates(ctx, req.CropID)
		if err != nil {
			return result, err
		}
		result.Templates = templates
		templateReq := req
		templateReq.MachineID = 0
		templateReq.Productivity = 0
		templateReq.FuelRate = 0
		templateReq.ResourceMaterialID = 0
		templateReq.ResourceRate = 0
		templateReq.ManualPrice = nil
		selected := map[int64]bool{}
		for _, id := range req.SelectedTemplateIDs {
			selected[id] = true
		}
		useAll := len(selected) == 0
		for _, t := range templates {
			if !useAll && !selected[t.ID] && !t.IsRequired {
				continue
			}
			rows, err := s.draftOperationRows(ctx, templateReq, t.OperationID, t.AreaFactor, coef)
			if err != nil {
				return result, err
			}
			result.Rows = append(result.Rows, rows...)
		}
		return result, nil
	}
	if req.OperationID <= 0 {
		return result, fmt.Errorf("выберите агротехническую работу")
	}
	rows, err := s.draftOperationRows(ctx, req, req.OperationID, 1, coef)
	if err != nil {
		return result, err
	}
	result.Rows = rows
	return result, nil
}

func (s *CalculationService) draftOperationRows(ctx context.Context, req models.CalculationDraftRequest, operationID int64, areaFactor, coef float64) ([]models.CalculationInputRow, error) {
	rule, err := s.Repo.OperationRuleByID(ctx, operationID)
	if err != nil {
		return nil, err
	}
	if rule.OperationID == 0 {
		return nil, fmt.Errorf("операция не найдена")
	}
	machineID := req.MachineID
	productivity := req.Productivity
	fuelRate := req.FuelRate
	if machineID == 0 || productivity <= 0 || fuelRate < 0 {
		m, err := s.Repo.PrimaryOperationMachine(ctx, operationID)
		if err != nil {
			return nil, err
		}
		if machineID == 0 {
			machineID = m.ID
		}
		if productivity <= 0 {
			productivity = m.Productivity
		}
		if fuelRate <= 0 {
			fuelRate = m.FuelRate
		}
	}
	if productivity <= 0 {
		productivity = 1
	}
	if areaFactor <= 0 {
		areaFactor = 1
	}
	usage := strings.ToLower(strings.TrimSpace(req.MachineUsageType))
	if usage == "" {
		usage = "own"
	}
	var rows []models.CalculationInputRow
	add := func(materialName, costItemName string, rate float64, manual *float64) error {
		if rate <= 0 {
			return nil
		}
		material, err := s.Repo.MaterialByName(ctx, materialName)
		if err != nil {
			return err
		}
		item, err := s.Repo.CostItemByName(ctx, costItemName)
		if err != nil {
			return err
		}
		if material.ID == 0 || item.ID == 0 {
			return nil
		}
		rows = append(rows, models.CalculationInputRow{OperationID: operationID, MaterialID: material.ID, MachineID: machineID, MachineUsageType: usage, CostItemID: item.ID, Rate: rate, ManualPrice: manual, Coefficient: coef, AreaFactor: areaFactor})
		return nil
	}
	if err := add("Дизельное топливо", "Топливо", fuelRate, nil); err != nil {
		return nil, err
	}
	hoursPerHa := 1 / productivity
	if err := add("Труд механизатора", "Оплата труда", hoursPerHa, nil); err != nil {
		return nil, err
	}
	if usage == "rent" {
		if err := add("Аренда техники", "Эксплуатация техники", hoursPerHa, nil); err != nil {
			return nil, err
		}
	} else {
		for _, name := range []string{"Амортизация", "Ремонт", "Техническое обслуживание"} {
			if err := add("Машино-час", name, hoursPerHa, nil); err != nil {
				return nil, err
			}
		}
	}
	if rule.ResourceType != "" {
		var materialID int64
		rate := req.ResourceRate
		if req.ResourceMaterialID > 0 {
			materialID = req.ResourceMaterialID
		}
		norm, err := s.Repo.NormFor(ctx, req.CropID, operationID, rule.ResourceType)
		if err != nil {
			return nil, err
		}
		if norm != nil {
			if materialID == 0 {
				materialID = norm.MaterialID
			}
			if rate <= 0 {
				rate = norm.Rate
			}
		}
		itemName := resourceCostItemName(rule.ResourceType)
		item, err := s.Repo.CostItemByName(ctx, itemName)
		if err != nil {
			return nil, err
		}
		if materialID > 0 && item.ID > 0 && rate > 0 {
			rows = append(rows, models.CalculationInputRow{OperationID: operationID, MaterialID: materialID, MachineID: machineID, MachineUsageType: usage, CostItemID: item.ID, Rate: rate, ManualPrice: req.ManualPrice, Coefficient: coef, AreaFactor: areaFactor})
		}
	}
	return rows, nil
}

func resourceCostItemName(materialType string) string {
	switch materialType {
	case "seed":
		return "Семена"
	case "fertilizer":
		return "Удобрения"
	case "pesticide":
		return "Средства защиты растений"
	default:
		return "Эксплуатация техники"
	}
}

func (s *CalculationService) Calculate(ctx context.Context, req models.CalculationRequest, save bool) (models.CalculationResult, error) {
	if req.CropID <= 0 {
		return models.CalculationResult{}, fmt.Errorf("выберите культуру")
	}
	if req.FederalDistrictID <= 0 {
		return models.CalculationResult{}, fmt.Errorf("выберите федеральный округ")
	}
	if req.AreaHa <= 0 {
		return models.CalculationResult{}, fmt.Errorf("площадь должна быть больше нуля")
	}
	if len(req.Rows) == 0 {
		return models.CalculationResult{}, fmt.Errorf("добавьте хотя бы одну строку расчёта")
	}

	mode := strings.TrimSpace(req.CalculationMode)
	if mode == "" {
		mode = "single_operation"
	}
	calcDate := calculationDate(req.CalculationDate)
	result := models.CalculationResult{CropID: req.CropID, FederalDistrictID: req.FederalDistrictID, RegionID: req.RegionID, AreaHa: req.AreaHa, CalculationMode: mode, IncludeComparison: req.IncludeComparison}
	compRows, err := s.Repo.ConditionCoefficientsByIDs(ctx, req.ConditionCoefficientIDs)
	if err != nil {
		return result, err
	}
	requestCoef := MultiplyCoefficients(compRows)
	if requestCoef == 0 {
		requestCoef = 1
	}
	for i, input := range req.Rows {
		if input.OperationID <= 0 {
			return result, fmt.Errorf("строка %d: выберите операцию", i+1)
		}
		if input.MaterialID <= 0 {
			return result, fmt.Errorf("строка %d: выберите ресурс", i+1)
		}
		if input.CostItemID <= 0 {
			return result, fmt.Errorf("строка %d: выберите статью затрат", i+1)
		}
		if input.Rate < 0 {
			return result, fmt.Errorf("строка %d: норма расхода не может быть отрицательной", i+1)
		}
		coef := input.Coefficient
		if coef == 0 {
			coef = requestCoef
		}
		if coef == 1 && len(req.ConditionCoefficientIDs) > 0 {
			coef = requestCoef
		}
		if coef < 0 {
			return result, fmt.Errorf("строка %d: коэффициент не может быть отрицательным", i+1)
		}
		areaFactor := input.AreaFactor
		if areaFactor == 0 {
			areaFactor = 1
		}

		costItem, err := s.Repo.CostItemByID(ctx, input.CostItemID)
		if err != nil {
			return result, fmt.Errorf("строка %d: ошибка статьи затрат: %w", i+1, err)
		}
		usage := strings.ToLower(strings.TrimSpace(input.MachineUsageType))
		if usage == "" {
			usage = "own"
		}
		if usage != "own" && usage != "rent" {
			return result, fmt.Errorf("строка %d: тип техники должен быть own или rent", i+1)
		}

		price := 0.0
		var priceID *int64
		priceSource := "ручной ввод"
		priceRegion := ""
		priceDate := DateString(calcDate)
		sourceURL := ""

		// Для строк эксплуатации техники цена зависит от режима: собственная техника или аренда.
		isMachineCost := isMachineCostItem(costItem.Name)
		machinePriceSource := ""
		if input.ManualPrice != nil {
			if *input.ManualPrice < 0 {
				return result, fmt.Errorf("строка %d: цена не может быть отрицательной", i+1)
			}
			price = *input.ManualPrice
			priceDate = DateString(calcDate)
			if isMachineCost {
				machinePriceSource = "ручная цена техники"
			}
		} else if isMachineCost && input.MachineID > 0 {
			machine, err := s.Repo.MachineByID(ctx, input.MachineID)
			if err != nil {
				return result, fmt.Errorf("строка %d: ошибка техники: %w", i+1, err)
			}
			if usage == "rent" {
				if machine.RentPrice <= 0 {
					return result, fmt.Errorf("строка %d: в справочнике техники не указана арендная стоимость машино-часа", i+1)
				}
				price = machine.RentPrice
				priceSource = "аренда техники"
				machinePriceSource = "арендная ставка из справочника техники"
			} else {
				if machine.DefaultPrice <= 0 {
					return result, fmt.Errorf("строка %d: в справочнике техники не указана стоимость машино-часа", i+1)
				}
				price = ownMachinePrice(costItem.Name, machine.DefaultPrice)
				priceSource = "собственная техника"
				machinePriceSource = "стоимость машино-часа собственной техники"
			}
			priceDate = DateString(calcDate)
			priceRegion = "внутренние данные предприятия"
		} else {
			latest, err := s.Repo.LatestPrice(ctx, input.MaterialID, req.RegionID, req.FederalDistrictID)
			if err != nil {
				return result, fmt.Errorf("строка %d: ошибка получения цены: %w", i+1, err)
			}
			if latest == nil {
				return result, fmt.Errorf("строка %d: нет цены для выбранного федерального округа, укажите цену вручную", i+1)
			}
			price = latest.Price
			id := latest.ID
			priceID = &id
			priceSource = latest.SourceName
			priceRegion = latest.RegionName
			priceDate = latest.EffectiveDate.Format("2006-01-02")
			sourceURL = latest.SourceURL
			if priceRegion == "" {
				priceRegion = latest.FederalDistrict
			}
		}

		quantity := req.AreaHa * areaFactor * input.Rate * coef
		amount := quantity * price
		result.TotalCost += amount
		if contributesMachineHours(costItem.Name) {
			result.TotalMachineHours += quantity
		}
		result.Rows = append(result.Rows, models.CalculationResultRow{
			OperationID:        input.OperationID,
			MaterialID:         input.MaterialID,
			MachineID:          input.MachineID,
			MachineUsageType:   usage,
			MachinePriceSource: machinePriceSource,
			CostItemID:         input.CostItemID,
			PriceSnapshotID:    priceID,
			Quantity:           quantity,
			Rate:               input.Rate,
			Price:              price,
			Coefficient:        coef,
			Amount:             amount,
			PriceSource:        priceSource,
			PriceRegion:        priceRegion,
			PriceDate:          priceDate,
			SourceURL:          sourceURL,
			AreaFactor:         areaFactor,
		})
	}
	result.CostPerHa = result.TotalCost / req.AreaHa
	if req.IncludeComparison {
		own, err := s.calculateTotalForUsage(ctx, req, "own")
		if err != nil {
			return result, err
		}
		rent, err := s.calculateTotalForUsage(ctx, req, "rent")
		if err != nil {
			return result, err
		}
		cheaper := "own"
		if rent < own {
			cheaper = "rent"
		}
		result.Comparison = &models.CalculationComparison{OwnTotal: own, RentTotal: rent, Delta: own - rent, CheaperUsageType: cheaper}
	}
	if save {
		id, err := s.Repo.SaveCalculation(ctx, req, result)
		if err != nil {
			return result, err
		}
		result.ID = id
	}
	return result, nil
}

func DateString(t time.Time) string {
	return t.Format("2006-01-02")
}

func calculationDate(value string) time.Time {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(value)); err == nil {
		return t
	}
	return time.Now()
}

func isMachineCostItem(name string) bool {
	value := strings.ToLower(name)
	return strings.Contains(value, "техник") || strings.Contains(value, "машин") || strings.Contains(value, "амортизац") || strings.Contains(value, "ремонт") || strings.Contains(value, "обслуж")
}

func ownMachinePrice(costItemName string, defaultPrice float64) float64 {
	for name, price := range MachineOwnBreakdown(defaultPrice) {
		if strings.EqualFold(costItemName, name) {
			return price
		}
	}
	return defaultPrice
}

func contributesMachineHours(costItemName string) bool {
	value := strings.ToLower(costItemName)
	return strings.Contains(value, "амортизац") || strings.Contains(value, "эксплуатац")
}

func (s *CalculationService) calculateTotalForUsage(ctx context.Context, req models.CalculationRequest, usageOverride string) (float64, error) {
	total := 0.0
	for i, input := range req.Rows {
		costItem, err := s.Repo.CostItemByID(ctx, input.CostItemID)
		if err != nil {
			return 0, fmt.Errorf("строка %d: ошибка статьи затрат: %w", i+1, err)
		}
		if usageOverride == "rent" && (strings.EqualFold(costItem.Name, "Ремонт") || strings.EqualFold(costItem.Name, "Техническое обслуживание")) {
			continue
		}
		coef := input.Coefficient
		if coef == 0 {
			coef = 1
		}
		areaFactor := input.AreaFactor
		if areaFactor == 0 {
			areaFactor = 1
		}
		price := 0.0
		if isMachineCostItem(costItem.Name) && input.MachineID > 0 {
			machine, err := s.Repo.MachineByID(ctx, input.MachineID)
			if err != nil {
				return 0, err
			}
			if usageOverride == "rent" {
				price = machine.RentPrice
			} else {
				price = ownMachinePrice(costItem.Name, machine.DefaultPrice)
				if strings.Contains(strings.ToLower(costItem.Name), "эксплуатац") {
					price = machine.DefaultPrice
				}
			}
		} else if input.ManualPrice != nil {
			price = *input.ManualPrice
		} else {
			latest, err := s.Repo.LatestPrice(ctx, input.MaterialID, req.RegionID, req.FederalDistrictID)
			if err != nil {
				return 0, err
			}
			if latest != nil {
				price = latest.Price
			}
		}
		total += req.AreaHa * areaFactor * input.Rate * coef * price
	}
	return total, nil
}
