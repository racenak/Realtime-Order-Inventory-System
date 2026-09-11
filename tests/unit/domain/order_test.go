package domain_test

import (
	"testing"
	"time"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	"github.com/stretchr/testify/assert"
)

func TestOrder_CalculateTotal(t *testing.T) {
	tests := []struct {
		name     string
		items    []domain.OrderItem
		discount float64
		shipping float64
		tax      float64
		expected float64
	}{
		{
			name: "single item",
			items: []domain.OrderItem{
				{Quantity: 2, UnitPrice: 25.00, TotalPrice: 50.00},
			},
			discount: 0,
			shipping: 5.00,
			tax:      4.00,
			expected: 59.00,
		},
		{
			name: "multiple items",
			items: []domain.OrderItem{
				{Quantity: 1, UnitPrice: 10.00, TotalPrice: 10.00},
				{Quantity: 3, UnitPrice: 5.00, TotalPrice: 15.00},
			},
			discount: 2.00,
			shipping: 3.00,
			tax:      2.00,
			expected: 28.00,
		},
		{
			name: "empty items",
			items: []domain.OrderItem{
				{Quantity: 1, UnitPrice: 0, TotalPrice: 0},
			},
			discount: 0,
			shipping: 0,
			tax:      0,
			expected: 0,
		},
		{
			name: "with discount",
			items: []domain.OrderItem{
				{Quantity: 1, UnitPrice: 100.00, TotalPrice: 100.00},
			},
			discount: 10.00,
			shipping: 0,
			tax:      0,
			expected: 90.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &domain.Order{
				Items:          tt.items,
				DiscountAmount: tt.discount,
				ShippingFee:    tt.shipping,
				TaxAmount:      tt.tax,
			}
			order.CalculateTotal()
			assert.Equal(t, tt.expected, order.TotalAmount)
			assert.Equal(t, tt.expected, order.TotalAmount)
		})
	}
}

func TestOrder_CanCancel(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.OrderStatus
		expected bool
	}{
		{"pending payment can cancel", domain.StatusPendingPayment, true},
		{"paid can cancel", domain.StatusPaid, true},
		{"processing cannot cancel", domain.StatusProcessing, false},
		{"shipped cannot cancel", domain.StatusShipped, false},
		{"delivered cannot cancel", domain.StatusDelivered, false},
		{"cancelled cannot cancel", domain.StatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &domain.Order{Status: tt.status}
			assert.Equal(t, tt.expected, order.CanCancel())
		})
	}
}

func TestOrder_CalculateTotal_SetsSubtotal(t *testing.T) {
	order := &domain.Order{
		Items: []domain.OrderItem{
			{Quantity: 2, UnitPrice: 30.00, TotalPrice: 60.00},
			{Quantity: 1, UnitPrice: 40.00, TotalPrice: 40.00},
		},
		DiscountAmount: 5.00,
		ShippingFee:    10.00,
		TaxAmount:      8.00,
	}

	order.CalculateTotal()

	assert.Equal(t, 100.00, order.Subtotal)
	assert.Equal(t, 113.00, order.TotalAmount) // 100 - 5 + 10 + 8
}

func TestOrderFields(t *testing.T) {
	now := time.Now()
	order := &domain.Order{
		ID:         "order-123",
		CustomerID: "cust-456",
		Status:     domain.StatusPendingPayment,
		Currency:   "USD",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	assert.Equal(t, "order-123", order.ID)
	assert.Equal(t, "cust-456", order.CustomerID)
	assert.Equal(t, domain.StatusPendingPayment, order.Status)
	assert.Equal(t, "USD", order.Currency)
}
