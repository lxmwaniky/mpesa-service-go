package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
	"github.com/lxmwaniky/mpesa-service-go/internal/infrastructure/daraja"
)

type mockTransactionRepository struct {
	transactions map[string]*domain.Transaction
	createErr    error
	getErr       error
	updateErr    error
	pingErr      error
}

func newMockRepo() *mockTransactionRepository {
	return &mockTransactionRepository{
		transactions: make(map[string]*domain.Transaction),
	}
}

func (m *mockTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.transactions[tx.ExternalReference] = tx
	return nil
}

func (m *mockTransactionRepository) GetByCheckoutRequestID(ctx context.Context, checkoutRequestID string) (*domain.Transaction, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, tx := range m.transactions {
		if tx.CheckoutRequestID != nil && *tx.CheckoutRequestID == checkoutRequestID {
			return tx, nil
		}
	}
	return nil, nil
}

func (m *mockTransactionRepository) GetByExternalReference(ctx context.Context, extRef string) (*domain.Transaction, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if tx, exists := m.transactions[extRef]; exists {
		return tx, nil
	}
	return nil, nil
}

func (m *mockTransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.transactions[tx.ExternalReference] = tx
	return nil
}

func (m *mockTransactionRepository) Ping(ctx context.Context) error {
	return m.pingErr
}

type mockDarajaGateway struct {
	resp *daraja.STKPushResponse
	err  error
}

func (m *mockDarajaGateway) SendSTKPush(ctx context.Context, phone string, amount float64, ref, desc string) (*daraja.STKPushResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestInitiateSTKPush_Success(t *testing.T) {
	repo := newMockRepo()
	gateway := &mockDarajaGateway{
		resp: &daraja.STKPushResponse{
			MerchantRequestID: "merch-123",
			CheckoutRequestID: "checkout-123",
		},
	}

	uc := NewMpesaUsecase(repo, gateway)
	tx, err := uc.InitiateSTKPush(context.Background(), "ref-1", "0700000000", 100.0, "test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tx.ExternalReference != "ref-1" {
		t.Errorf("expected external ref 'ref-1', got %s", tx.ExternalReference)
	}

	if *tx.MerchantRequestID != "merch-123" {
		t.Errorf("expected merchant request ID 'merch-123', got %s", *tx.MerchantRequestID)
	}
}

func TestInitiateSTKPush_Duplicate(t *testing.T) {
	repo := newMockRepo()
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		Status:            domain.StatusPending,
	}

	gateway := &mockDarajaGateway{}
	uc := NewMpesaUsecase(repo, gateway)

	_, err := uc.InitiateSTKPush(context.Background(), "ref-1", "0700000000", 100.0, "test")
	if err == nil {
		t.Error("expected duplicate error, got nil")
	}
}

func TestProcessSTKCallback_Success(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
	}

	uc := NewMpesaUsecase(repo, nil)

	payload := &domain.STKCallbackPayload{
		Body: domain.STKCallbackBody{
			StkCallback: domain.STKCallback{
				CheckoutRequestID: "checkout-123",
				ResultCode:        0,
				ResultDesc:        "Success",
				CallbackMetadata: &domain.STKCallbackMetadata{
					Item: []domain.STKCallbackMetadataItem{
						{Name: "MpesaReceiptNumber", Value: "REC123"},
					},
				},
			},
		},
	}

	err := uc.ProcessSTKCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions["ref-1"]
	if tx.Status != domain.StatusSuccess {
		t.Errorf("expected status %s, got %s", domain.StatusSuccess, tx.Status)
	}

	if *tx.MpesaReceiptNumber != "REC123" {
		t.Errorf("expected receipt 'REC123', got %s", *tx.MpesaReceiptNumber)
	}
}

func TestProcessSTKCallback_AlreadyProcessed(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusSuccess,
	}

	uc := NewMpesaUsecase(repo, nil)

	payload := &domain.STKCallbackPayload{
		Body: domain.STKCallbackBody{
			StkCallback: domain.STKCallback{
				CheckoutRequestID: "checkout-123",
				ResultCode:        0,
				ResultDesc:        "Success",
			},
		},
	}

	err := uc.ProcessSTKCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessSTKCallback_ConcurrencyAlreadyProcessedSentinel(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
	}
	repo.updateErr = domain.ErrAlreadyProcessed

	uc := NewMpesaUsecase(repo, nil)

	payload := &domain.STKCallbackPayload{
		Body: domain.STKCallbackBody{
			StkCallback: domain.STKCallback{
				CheckoutRequestID: "checkout-123",
				ResultCode:        0,
				ResultDesc:        "Success",
			},
		},
	}

	err := uc.ProcessSTKCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("expected nil error when repo returns ErrAlreadyProcessed, got: %v", err)
	}
}

func TestGetTransactionStatus(t *testing.T) {
	repo := newMockRepo()
	repo.transactions["ref-1"] = &domain.Transaction{
		ID:                42,
		ExternalReference: "ref-1",
		Status:            domain.StatusSuccess,
		Amount:            250.0,
		ResultCode:        0,
		ResultDesc:        "Success",
		UpdatedAt:         time.Now(),
	}

	uc := NewMpesaUsecase(repo, nil)

	status, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.ID != 42 {
		t.Errorf("expected ID 42, got %d", status.ID)
	}

	if status.Status != domain.StatusSuccess {
		t.Errorf("expected status SUCCESS, got %s", status.Status)
	}

	if status.UserMessage != "Payment completed successfully." {
		t.Errorf("expected user message, got: %s", status.UserMessage)
	}
}
