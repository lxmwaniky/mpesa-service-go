package domain

import (
	"context"
	"errors"
)

var ErrAlreadyProcessed = errors.New("transaction already processed")

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByCheckoutRequestID(ctx context.Context, checkoutRequestID string) (*Transaction, error)
	GetByExternalReference(ctx context.Context, extRef string) (*Transaction, error)
	Update(ctx context.Context, tx *Transaction) error
	UpdateReceipt(ctx context.Context, checkoutRequestID string, receiptNumber string) error
	Ping(ctx context.Context) error
}