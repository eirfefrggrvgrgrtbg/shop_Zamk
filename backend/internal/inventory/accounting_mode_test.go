package inventory_test

import (
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateInventoryHealth_AccountingModesAndInvariants(t *testing.T) {
	t.Run("Pure Serialized: physical equals aggregate, no legacy", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 10, Reserved: 2, Available: 8}
		phys := inventory.PhysicalStock{Warehouse: 10, Allocated: 2, Free: 8}
		leg := inventory.LegacyStock{OnHand: 0, Reserved: 0, Available: 0}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "serialized", mode)
		assert.Equal(t, "healthy", health.Status)
		assert.Empty(t, health.Issues)
	})

	t.Run("Pure Legacy: aggregate exists, zero physical ZMU", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 20, Reserved: 5, Available: 15}
		phys := inventory.PhysicalStock{Warehouse: 0, Allocated: 0, Free: 0}
		leg := inventory.LegacyStock{OnHand: 20, Reserved: 5, Available: 15}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "legacy", mode)
		assert.Equal(t, "healthy", health.Status)
		assert.Empty(t, health.Issues)
	})

	t.Run("Mixed: physical and legacy coexist", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 29, Reserved: 2, Available: 27}
		phys := inventory.PhysicalStock{Warehouse: 4, Allocated: 2, Free: 2}
		leg := inventory.LegacyStock{OnHand: 25, Reserved: 0, Available: 25}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "mixed", mode)
		assert.Equal(t, "healthy", health.Status)
		assert.Empty(t, health.Issues)
	})

	t.Run("Physical exceeds aggregate total: mode is serialized, health warning", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 5, Reserved: 0, Available: 5}
		phys := inventory.PhysicalStock{Warehouse: 10, Allocated: 0, Free: 10}
		leg := inventory.LegacyStock{OnHand: -5, Reserved: 0, Available: -5}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "serialized", mode)
		assert.Equal(t, "warning", health.Status)
		assert.Contains(t, health.Issues, "physical_exceeds_aggregate")
		assert.Contains(t, health.Issues, "legacy_projection_negative")
	})

	t.Run("Allocated exceeds physical warehouse: mode is mixed, health critical", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 10, Reserved: 5, Available: 5}
		phys := inventory.PhysicalStock{Warehouse: 2, Allocated: 4, Free: 0}
		leg := inventory.LegacyStock{OnHand: 8, Reserved: 1, Available: 7}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "mixed", mode)
		assert.Equal(t, "critical", health.Status)
		assert.Contains(t, health.Issues, "allocated_exceeds_physical")
	})

	t.Run("Reserved exceeds total stock: mode is serialized, health critical", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 5, Reserved: 8, Available: 0}
		phys := inventory.PhysicalStock{Warehouse: 5, Allocated: 5, Free: 0}
		leg := inventory.LegacyStock{OnHand: 0, Reserved: 3, Available: -3}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "serialized", mode)
		assert.Equal(t, "critical", health.Status)
		assert.Contains(t, health.Issues, "reserved_exceeds_total")
	})

	t.Run("MIXED + stale_active_allocation: mode remains mixed, health warning", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 29, Reserved: 1, Available: 28}
		phys := inventory.PhysicalStock{Warehouse: 4, Allocated: 1, Free: 3, StaleAllocated: 1}
		leg := inventory.LegacyStock{OnHand: 25, Reserved: 0, Available: 25}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "mixed", mode)
		assert.Equal(t, "warning", health.Status)
		assert.Contains(t, health.Issues, "stale_active_allocation")
	})

	t.Run("SERIALIZED + stale_active_allocation: mode remains serialized, health warning", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 5, Reserved: 1, Available: 4}
		phys := inventory.PhysicalStock{Warehouse: 5, Allocated: 1, Free: 4, StaleAllocated: 1}
		leg := inventory.LegacyStock{OnHand: 0, Reserved: 0, Available: 0}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "serialized", mode)
		assert.Equal(t, "warning", health.Status)
		assert.Contains(t, health.Issues, "stale_active_allocation")
	})

	t.Run("LEGACY + unrelated health issue: mode remains legacy, health critical", func(t *testing.T) {
		agg := inventory.AggregateStock{Total: 10, Reserved: 15, Available: -5}
		phys := inventory.PhysicalStock{Warehouse: 0, Allocated: 0, Free: 0}
		leg := inventory.LegacyStock{OnHand: 10, Reserved: 15, Available: -5}

		mode, health := inventory.EvaluateInventoryHealth(agg, phys, leg)
		assert.Equal(t, "legacy", mode)
		assert.Equal(t, "critical", health.Status)
		assert.Contains(t, health.Issues, "reserved_exceeds_total")
	})
}
