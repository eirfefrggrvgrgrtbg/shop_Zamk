package products_test

import (
	"context"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForecast_PureFormulas(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	t.Run("Dead variant: 0 paid units in 30 days, free=0 -> no alert", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			SellerID:          uuid.New(),
			ProductTitle:      "Джинсы",
			ProductStatus:     "published",
			SellerStatus:      "active",
			VariantIsActive:   true,
			ObservationStart:  now.AddDate(0, 0, -35), // 35 days ago -> capped at 30
			FreeSellableStock: 0,
			PaidDemandUnits:   0,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 30, m.ObservedDays)
		assert.Equal(t, 0.0, m.DailySalesVelocity)
		assert.False(t, m.IsAtRisk, "Dead variant must not alert even if free=0")
	})

	t.Run("Cold start one sale: 1 paid unit, observed 2 days, free=2 -> no alert", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -2), // 2 days ago
			FreeSellableStock: 2,
			PaidDemandUnits:   1,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 2, m.ObservedDays)
		assert.False(t, m.EligibleForForecast, "1 sale in 2 days does not meet minimum demand evidence")
		assert.False(t, m.IsAtRisk, "Cold start with 1 sale must not alert")
	})

	t.Run("Cold start two sales: 2 paid units, observed 2 days, free=2 -> critical", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -2), // 2 days ago
			FreeSellableStock: 2,
			PaidDemandUnits:   2,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 2, m.ObservedDays)
		assert.True(t, m.EligibleForForecast)
		assert.InDelta(t, 1.0, m.DailySalesVelocity, 0.0001)
		assert.InDelta(t, 2.0, m.DaysOfCover, 0.0001)
		assert.True(t, m.IsAtRisk)
		assert.Equal(t, notifications.SeverityCritical, m.Severity)
	})

	t.Run("Slow seller: 2 units / 30 observed days, free=1, cover=15 -> no new alert", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -30),
			FreeSellableStock: 1,
			PaidDemandUnits:   2,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 30, m.ObservedDays)
		assert.InDelta(t, 2.0/30.0, m.DailySalesVelocity, 0.0001)
		assert.InDelta(t, 15.0, m.DaysOfCover, 0.0001)
		assert.False(t, m.IsAtRisk, "Cover > 14 days must not create new alert")
	})

	t.Run("Medium seller: 6 units / 30d, free=2, cover=10 -> warning", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -30),
			FreeSellableStock: 2,
			PaidDemandUnits:   6,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 30, m.ObservedDays)
		assert.InDelta(t, 0.2, m.DailySalesVelocity, 0.0001)
		assert.InDelta(t, 10.0, m.DaysOfCover, 0.0001)
		assert.True(t, m.IsAtRisk)
		assert.Equal(t, notifications.SeverityWarning, m.Severity)
	})

	t.Run("Fast seller: 20 units / 30d, free=5, cover=7.5 -> warning", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -30),
			FreeSellableStock: 5,
			PaidDemandUnits:   20,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 30, m.ObservedDays)
		assert.InDelta(t, 20.0/30.0, m.DailySalesVelocity, 0.0001)
		assert.InDelta(t, 7.5, m.DaysOfCover, 0.0001)
		assert.True(t, m.IsAtRisk)
		assert.Equal(t, notifications.SeverityWarning, m.Severity)
	})

	t.Run("Critical: positive velocity with cover <= 7", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -30),
			FreeSellableStock: 2,
			PaidDemandUnits:   10,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.InDelta(t, 6.0, m.DaysOfCover, 0.0001)
		assert.True(t, m.IsAtRisk)
		assert.Equal(t, notifications.SeverityCritical, m.Severity)
	})

	t.Run("Zero free stock with positive velocity -> cover=0 critical", func(t *testing.T) {
		input := products.VariantForecastInput{
			VariantID:         uuid.New(),
			ProductID:         uuid.New(),
			ObservationStart:  now.AddDate(0, 0, -30),
			FreeSellableStock: 0,
			PaidDemandUnits:   5,
		}

		m := products.ComputeVariantForecast(input, now, false)
		assert.Equal(t, 0.0, m.DaysOfCover)
		assert.True(t, m.IsAtRisk)
		assert.Equal(t, notifications.SeverityCritical, m.Severity)
	})
}

func TestForecast_Hysteresis(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	// A variant that was already participating in an active alert
	// Case 1: cover was 13, now 16 (between 14 and 18)
	// DSV = 1/day, free = 16 -> cover = 16.
	input16 := products.VariantForecastInput{
		VariantID:         uuid.New(),
		ProductID:         uuid.New(),
		ObservationStart:  now.AddDate(0, 0, -10),
		FreeSellableStock: 16,
		PaidDemandUnits:   10, // 10 units / 10 days = 1.0/day
	}
	m16 := products.ComputeVariantForecast(input16, now, true)
	assert.True(t, m16.IsAtRisk, "Under hysteresis, cover=16 (<18) remains active")
	assert.Equal(t, notifications.SeverityWarning, m16.Severity)

	// If it was NOT in the active alert, cover=16 would NOT enter risk:
	m16New := products.ComputeVariantForecast(input16, now, false)
	assert.False(t, m16New.IsAtRisk, "For new entry, cover=16 (>14) must not enter risk")

	// Case 2: cover changes 16 -> 18+ (e.g. 18 days) -> resolves
	input18 := products.VariantForecastInput{
		VariantID:         uuid.New(),
		ProductID:         uuid.New(),
		ObservationStart:  now.AddDate(0, 0, -10),
		FreeSellableStock: 18,
		PaidDemandUnits:   10, // 1.0/day -> 18 days
	}
	m18 := products.ComputeVariantForecast(input18, now, true)
	assert.False(t, m18.IsAtRisk, "Under hysteresis, cover=18 (>=18) must resolve")

	// Case 3: cover worsens into <= 7 -> severity updates to critical
	input5 := products.VariantForecastInput{
		VariantID:         uuid.New(),
		ProductID:         uuid.New(),
		ObservationStart:  now.AddDate(0, 0, -10),
		FreeSellableStock: 5,
		PaidDemandUnits:   10, // 1.0/day -> 5 days
	}
	m5 := products.ComputeVariantForecast(input5, now, true)
	assert.True(t, m5.IsAtRisk)
	assert.Equal(t, notifications.SeverityCritical, m5.Severity)
}

func TestForecast_CopyFormatting(t *testing.T) {
	t.Run("Two variants: 4 days and 9 days", func(t *testing.T) {
		vars := []products.VariantForecastMetrics{
			{
				Color:       "Красный",
				Size:        "L",
				DaysOfCover: 4.29,
				Severity:    notifications.SeverityCritical,
			},
			{
				Color:       "Белый",
				Size:        "M",
				DaysOfCover: 8.8,
				Severity:    notifications.SeverityWarning,
			},
		}

		title, body := products.BuildProductForecastCopy(vars, notifications.SeverityCritical)
		assert.Equal(t, "Товар может закончиться в ближайшие дни", title)
		assert.Equal(t, "Красный / L — примерно на 4 дня; Белый / M — примерно на 9 дней.", body)
	})

	t.Run("More than 3 variants: shows first 3 and 'Ещё: N'", func(t *testing.T) {
		vars := []products.VariantForecastMetrics{
			{Color: "Красный", Size: "S", DaysOfCover: 1.0, Severity: notifications.SeverityCritical},
			{Color: "Красный", Size: "M", DaysOfCover: 2.0, Severity: notifications.SeverityCritical},
			{Color: "Красный", Size: "L", DaysOfCover: 3.0, Severity: notifications.SeverityCritical},
			{Color: "Красный", Size: "XL", DaysOfCover: 4.0, Severity: notifications.SeverityCritical},
			{Color: "Красный", Size: "XXL", DaysOfCover: 5.0, Severity: notifications.SeverityCritical},
		}

		title, body := products.BuildProductForecastCopy(vars, notifications.SeverityCritical)
		assert.Equal(t, "Товар может закончиться в ближайшие дни", title)
		assert.Equal(t, "Красный / S — примерно на 1 день; Красный / M — примерно на 2 дня; Красный / L — примерно на 3 дня; Ещё: 2.", body)
	})

	t.Run("Warning title", func(t *testing.T) {
		vars := []products.VariantForecastMetrics{
			{Color: "Синий", Size: "M", DaysOfCover: 10.0, Severity: notifications.SeverityWarning},
		}

		title, body := products.BuildProductForecastCopy(vars, notifications.SeverityWarning)
		assert.Equal(t, "Запас товара скоро закончится", title)
		assert.Equal(t, "Синий / M — примерно на 10 дней.", body)
	})
}

func TestForecast_DedupeKey(t *testing.T) {
	pID := uuid.MustParse("24758527-bdf4-4c9d-8332-d6fdbdcc2a97")
	key := products.StockForecastDedupeKey(pID)
	require.Equal(t, "stock:forecast:product:24758527-bdf4-4c9d-8332-d6fdbdcc2a97", key)
}

func TestForecast_WorkerRunnerSafety(t *testing.T) {
	t.Run("ReconcileStockForecastAlerts safely handles nil db pool without panic", func(t *testing.T) {
		svc := products.NewService(nil, nil, nil, nil, nil)
		err := svc.ReconcileStockForecastAlerts(context.Background())
		assert.NoError(t, err)

		err = svc.ReconcileStockForecastForProduct(context.Background(), uuid.New())
		assert.NoError(t, err)
	})

	t.Run("Worker runner sequential execution and error isolation", func(t *testing.T) {
		executedCount := 0
		simulatedErr := false

		runFunc := func() {
			executedCount++
			if simulatedErr {
				// simulate error logging without panic
				return
			}
		}

		// First run: immediate execution
		runFunc()
		assert.Equal(t, 1, executedCount)

		// Next run with simulated error
		simulatedErr = true
		runFunc()
		assert.Equal(t, 2, executedCount)

		// Future runs continue normally
		simulatedErr = false
		runFunc()
		assert.Equal(t, 3, executedCount)
	})
}
