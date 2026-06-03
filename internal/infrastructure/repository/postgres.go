package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schemaSQL string

type PostgresDB struct {
	DB *sql.DB
}

func NewPostgresDB(databaseURL string) (*PostgresDB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return nil, err
	}

	return &PostgresDB{DB: db}, nil
}

func (p *PostgresDB) Close() error {
	return p.DB.Close()
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresTransactionRepository(db *sql.DB) domain.TransactionRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (external_reference, merchant_request_id, checkout_request_id, phone_number, amount, mpesa_receipt_number, status, result_code, result_desc, transaction_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`
	err := r.db.QueryRowContext(
		ctx,
		query,
		tx.ExternalReference,
		tx.MerchantRequestID,
		tx.CheckoutRequestID,
		tx.PhoneNumber,
		tx.Amount,
		tx.MpesaReceiptNumber,
		tx.Status,
		tx.ResultCode,
		tx.ResultDesc,
		tx.TransactionType,
		tx.CreatedAt,
		tx.UpdatedAt,
	).Scan(&tx.ID)
	return err
}

func (r *postgresRepository) GetByCheckoutRequestID(ctx context.Context, checkoutRequestID string) (*domain.Transaction, error) {
	query := `
		SELECT id, external_reference, merchant_request_id, checkout_request_id, phone_number, amount, mpesa_receipt_number, status, result_code, result_desc, transaction_type, created_at, updated_at
		FROM transactions
		WHERE checkout_request_id = $1
	`
	var tx domain.Transaction
	err := r.db.QueryRowContext(ctx, query, checkoutRequestID).Scan(
		&tx.ID,
		&tx.ExternalReference,
		&tx.MerchantRequestID,
		&tx.CheckoutRequestID,
		&tx.PhoneNumber,
		&tx.Amount,
		&tx.MpesaReceiptNumber,
		&tx.Status,
		&tx.ResultCode,
		&tx.ResultDesc,
		&tx.TransactionType,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *postgresRepository) GetByExternalReference(ctx context.Context, extRef string) (*domain.Transaction, error) {
	query := `
		SELECT id, external_reference, merchant_request_id, checkout_request_id, phone_number, amount, mpesa_receipt_number, status, result_code, result_desc, transaction_type, created_at, updated_at
		FROM transactions
		WHERE external_reference = $1
		ORDER BY id DESC
		LIMIT 1
	`
	var tx domain.Transaction
	err := r.db.QueryRowContext(ctx, query, extRef).Scan(
		&tx.ID,
		&tx.ExternalReference,
		&tx.MerchantRequestID,
		&tx.CheckoutRequestID,
		&tx.PhoneNumber,
		&tx.Amount,
		&tx.MpesaReceiptNumber,
		&tx.Status,
		&tx.ResultCode,
		&tx.ResultDesc,
		&tx.TransactionType,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *postgresRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	query := `
		UPDATE transactions
		SET mpesa_receipt_number = $1, status = $2, result_code = $3, result_desc = $4, updated_at = $5
		WHERE checkout_request_id = $6 AND status = 'PENDING'
	`
	result, err := r.db.ExecContext(
		ctx,
		query,
		tx.MpesaReceiptNumber,
		tx.Status,
		tx.ResultCode,
		tx.ResultDesc,
		tx.UpdatedAt,
		tx.CheckoutRequestID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAlreadyProcessed
	}
	return nil
}

func (r *postgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}