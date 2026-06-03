package domain

import "context"

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByCheckoutRequestID(ctx context.Context, checkoutRequestID string) (*Transaction, error)
	GetByExternalReference(ctx context.Context, extRef string) (*Transaction, error)
	Update(ctx context.Context, tx *Transaction) error
}