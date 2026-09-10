package domain

import "errors"

var (
	ErrInsufficientStock      = errors.New("insufficient stock")
	ErrInventoryNotFound      = errors.New("inventory not found")
	ErrConcurrentModification = errors.New("concurrent modification detected")
	ErrReservationNotFound    = errors.New("reservation not found")
	ErrReservationExpired     = errors.New("reservation expired")
	ErrInvalidQuantity        = errors.New("invalid quantity")
)
