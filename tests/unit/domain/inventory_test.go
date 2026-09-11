package domain_test

import (
	"testing"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
	"github.com/stretchr/testify/assert"
)

func TestInventory_Available(t *testing.T) {
	tests := []struct {
		name            string
		quantityOnHand  int
		quantityReserved int
		expected        int
	}{
		{"full stock available", 100, 0, 100},
		{"partially reserved", 100, 30, 70},
		{"fully reserved", 100, 100, 0},
		{"over-reserved (edge case)", 50, 60, -10},
		{"zero stock", 0, 0, 0},
		{"negative on hand (data issue)", -5, 0, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &domain.Inventory{
				QuantityOnHand:   tt.quantityOnHand,
				QuantityReserved: tt.quantityReserved,
			}
			assert.Equal(t, tt.expected, inv.Available())
		})
	}
}

func TestInventoryMovementTypes(t *testing.T) {
	assert.Equal(t, domain.MovementType("in"), domain.MovementIn)
	assert.Equal(t, domain.MovementType("out"), domain.MovementOut)
	assert.Equal(t, domain.MovementType("reserve"), domain.MovementReserve)
	assert.Equal(t, domain.MovementType("release"), domain.MovementRelease)
	assert.Equal(t, domain.MovementType("adjustment"), domain.MovementAdjustment)
	assert.Equal(t, domain.MovementType("transfer_in"), domain.MovementTransferIn)
	assert.Equal(t, domain.MovementType("transfer_out"), domain.MovementTransferOut)
}
