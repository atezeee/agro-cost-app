package services

import (
	"math"
	"testing"

	"agro-cost-app/pkg/models"
)

func TestMultiplyCoefficients(t *testing.T) {
	items := []models.ConditionCoefficient{
		{Value: 1.10},
		{Value: 1.05},
		{Value: 1.08},
	}
	got := MultiplyCoefficients(items)
	want := 1.2474
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("MultiplyCoefficients() = %v, want %v", got, want)
	}
}

func TestMachineOwnBreakdownSumsToBaseCost(t *testing.T) {
	const base = 2400.0
	parts := MachineOwnBreakdown(base)
	got := parts["Амортизация"] + parts["Ремонт"] + parts["Техническое обслуживание"]
	if math.Abs(got-base) > 0.000001 {
		t.Fatalf("own machine breakdown sum = %v, want %v", got, base)
	}
	if parts["Амортизация"] != 1200 || parts["Ремонт"] != 720 || parts["Техническое обслуживание"] != 480 {
		t.Fatalf("unexpected breakdown: %#v", parts)
	}
}
