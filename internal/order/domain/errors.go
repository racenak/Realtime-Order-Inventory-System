package domain

import "errors"

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderNotCancellable = errors.New("order is not cancellable")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrEmptyCart          = errors.New("order must contain at least one item")
	ErrInvalidQuantity    = errors.New("quantity must be greater than 0")
)
