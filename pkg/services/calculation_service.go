package services

import (
	"context"
	"fmt"
	"strings"

	"agro-cost-app/pkg/models"
	"agro-cost-app/pkg/repository"
)

type CalculationService struct{ Repo *repository.Repository }

func NewCalculationService(repo *repository.Repository) *CalculationService {
	return &CalculationService{Repo: repo}
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

	result := models.CalculationResult{CropID: req.CropID, FederalDistrictID: req.FederalDistrictID, RegionID: req.RegionID, AreaHa: req.AreaHa}
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
			coef = 1
		}
		if coef < 0 {
			return result, fmt.Errorf("строка %d: коэффициент не может быть отрицательным", i+1)
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

		// Для строк эксплуатации техники цена зависит от режима: собственная техника или аренда.
		isMachineCost := strings.Contains(strings.ToLower(costItem.Name), "техник") || strings.Contains(strings.ToLower(costItem.Name), "машин")
		machinePriceSource := ""
		if input.ManualPrice != nil {
			if *input.ManualPrice < 0 {
				return result, fmt.Errorf("строка %d: цена не может быть отрицательной", i+1)
			}
			price = *input.ManualPrice
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
				price = machine.DefaultPrice
				priceSource = "собственная техника"
				machinePriceSource = "стоимость машино-часа собственной техники"
			}
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
			if priceRegion == "" {
				priceRegion = latest.FederalDistrict
			}
		}

		quantity := req.AreaHa * input.Rate * coef
		amount := quantity * price
		result.TotalCost += amount
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
		})
	}
	result.CostPerHa = result.TotalCost / req.AreaHa
	if save {
		id, err := s.Repo.SaveCalculation(ctx, req, result)
		if err != nil {
			return result, err
		}
		result.ID = id
	}
	return result, nil
}
