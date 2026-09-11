package domain

import (
	"time"
)

type OrderStatus string

const (
	StatusPendingPayment OrderStatus = "pending_payment"
	StatusPaid           OrderStatus = "paid"
	StatusProcessing     OrderStatus = "processing"
	StatusShipped        OrderStatus = "shipped"
	StatusDelivered      OrderStatus = "delivered"
	StatusCancelled      OrderStatus = "cancelled"
)

type Order struct {
	ID              string      `json:"id" db:"id"`
	CustomerID      string      `json:"customer_id" db:"customer_id"`
	Status          OrderStatus `json:"status" db:"status"`
	Currency        string      `json:"currency" db:"currency"`
	Subtotal        float64     `json:"subtotal" db:"subtotal"`
	DiscountAmount  float64     `json:"discount_amount" db:"discount_amount"`
	ShippingFee     float64     `json:"shipping_fee" db:"shipping_fee"`
	TaxAmount       float64     `json:"tax_amount" db:"tax_amount"`
	TotalAmount     float64     `json:"total_amount" db:"total_amount"`
	ShippingAddress Address     `json:"shipping_address"`
	Items           []OrderItem `json:"items,omitempty"`
	IdempotencyKey  string      `json:"idempotency_key,omitempty" db:"idempotency_key"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
}

type OrderItem struct {
	ID          string  `json:"id" db:"id"`
	OrderID     string  `json:"order_id" db:"order_id"`
	ProductID   string  `json:"product_id" db:"product_id"`
	SKU         string  `json:"sku" db:"sku"`
	ProductName string  `json:"product_name" db:"product_name"`
	Quantity    int     `json:"quantity" db:"quantity"`
	UnitPrice   float64 `json:"unit_price" db:"unit_price"`
	TotalPrice  float64 `json:"total_price" db:"total_price"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}

func (o *Order) CalculateTotal() {
	o.Subtotal = 0
	for _, item := range o.Items {
		o.Subtotal += item.TotalPrice
	}
	o.TotalAmount = o.Subtotal - o.DiscountAmount + o.ShippingFee + o.TaxAmount
}

func (o *Order) CanCancel() bool {
	return o.Status == StatusPendingPayment || o.Status == StatusPaid
}
