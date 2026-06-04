package usecase

import (
	"context"
	"fmt"
	"strings"
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

func (m *mockTransactionRepository) UpdateReceipt(ctx context.Context, checkoutRequestID string, receiptNumber string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for _, tx := range m.transactions {
		if tx.CheckoutRequestID != nil && *tx.CheckoutRequestID == checkoutRequestID {
			tx.MpesaReceiptNumber = &receiptNumber
			tx.UpdatedAt = time.Now()
			break
		}
	}
	return nil
}

func (m *mockTransactionRepository) Ping(ctx context.Context) error {
	return m.pingErr
}

type mockDarajaGateway struct {
	resp            *daraja.STKPushResponse
	queryResp       *daraja.STKQueryResponse
	err             error
	queryErr        error
	registerC2BResp *daraja.C2BRegisterResponse
	registerC2BErr  error
}

func (m *mockDarajaGateway) SendSTKPush(ctx context.Context, phone string, amount float64, ref, desc string) (*daraja.STKPushResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func (m *mockDarajaGateway) QuerySTKPush(ctx context.Context, checkoutRequestID string) (*daraja.STKQueryResponse, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResp, nil
}

func (m *mockDarajaGateway) RegisterC2BURLs(ctx context.Context, validationURL, confirmationURL string, apiVersion string) (*daraja.C2BRegisterResponse, error) {
	if m.registerC2BErr != nil {
		return nil, m.registerC2BErr
	}
	return m.registerC2BResp, nil
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

func TestProcessSTKCallback_FallbackReceiptRecovery(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusSuccess,
		MpesaReceiptNumber: nil,
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
						{Name: "MpesaReceiptNumber", Value: "RECOVERY999"},
					},
				},
			},
		},
	}

	err := uc.ProcessSTKCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error during fallback receipt recovery: %v", err)
	}

	tx := repo.transactions["ref-1"]
	if tx.MpesaReceiptNumber == nil || *tx.MpesaReceiptNumber != "RECOVERY999" {
		t.Errorf("expected receipt recovered 'RECOVERY999', got %v", tx.MpesaReceiptNumber)
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

func TestGetTransactionStatus_PendingQuerySuccess(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ID:                42,
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
		TransactionType:   "STK_PUSH",
		Amount:            250.0,
		UpdatedAt:         time.Now(),
	}

	gateway := &mockDarajaGateway{
		queryResp: &daraja.STKQueryResponse{
			ResponseCode: "0",
			ResultCode:   0,
			ResultDesc:   "Success",
		},
	}

	uc := NewMpesaUsecase(repo, gateway)

	status, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != domain.StatusSuccess {
		t.Errorf("expected status SUCCESS, got %s", status.Status)
	}

	dbTx := repo.transactions["ref-1"]
	if dbTx.Status != domain.StatusSuccess {
		t.Errorf("expected db transaction status SUCCESS, got %s", dbTx.Status)
	}
}

func TestGetTransactionStatus_PendingQueryFailed(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ID:                42,
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
		TransactionType:   "STK_PUSH",
		Amount:            250.0,
		UpdatedAt:         time.Now(),
	}

	gateway := &mockDarajaGateway{
		queryResp: &daraja.STKQueryResponse{
			ResponseCode: "0",
			ResultCode:   1032,
			ResultDesc:   "Request cancelled by user",
		},
	}

	uc := NewMpesaUsecase(repo, gateway)

	status, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != domain.StatusFailed {
		t.Errorf("expected status FAILED, got %s", status.Status)
	}

	dbTx := repo.transactions["ref-1"]
	if dbTx.Status != domain.StatusFailed {
		t.Errorf("expected db transaction status FAILED, got %s", dbTx.Status)
	}
}

func TestGetTransactionStatus_PendingQueryAPIError(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ID:                42,
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
		TransactionType:   "STK_PUSH",
		Amount:            250.0,
		UpdatedAt:         time.Now(),
	}

	gateway := &mockDarajaGateway{
		queryErr: fmt.Errorf("network timeout"),
	}

	uc := NewMpesaUsecase(repo, gateway)

	status, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != domain.StatusPending {
		t.Errorf("expected status PENDING, got %s", status.Status)
	}

	dbTx := repo.transactions["ref-1"]
	if dbTx.Status != domain.StatusPending {
		t.Errorf("expected db transaction status to remain PENDING, got %s", dbTx.Status)
	}
}

func TestGetTransactionStatus_RepositoryError(t *testing.T) {
	repo := newMockRepo()
	repo.getErr = fmt.Errorf("database error")

	uc := NewMpesaUsecase(repo, nil)

	_, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err == nil {
		t.Error("expected error from repository, got nil")
	}
	if err.Error() != "database error" {
		t.Errorf("expected 'database error', got '%v'", err)
	}
}

func TestGetTransactionStatus_NotFound(t *testing.T) {
	repo := newMockRepo()

	uc := NewMpesaUsecase(repo, nil)

	status, err := uc.GetTransactionStatus(context.Background(), "ref-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != nil {
		t.Errorf("expected nil status for not found transaction, got %v", status)
	}
}

func TestInitiateSTKPush_DatabaseError(t *testing.T) {
	repo := newMockRepo()
	repo.createErr = fmt.Errorf("database error")

	gateway := &mockDarajaGateway{
		resp: &daraja.STKPushResponse{
			MerchantRequestID: "merch-123",
			CheckoutRequestID: "checkout-123",
		},
	}

	uc := NewMpesaUsecase(repo, gateway)
	_, err := uc.InitiateSTKPush(context.Background(), "ref-1", "0700000000", 100.0, "test")

	if err == nil {
		t.Error("expected error from repository, got nil")
	}
	if err.Error() != "database error" {
		t.Errorf("expected 'database error', got '%v'", err)
	}
}

func TestProcessSTKCallback_DatabaseError(t *testing.T) {
	repo := newMockRepo()
	checkoutID := "checkout-123"
	repo.transactions["ref-1"] = &domain.Transaction{
		ExternalReference: "ref-1",
		CheckoutRequestID: &checkoutID,
		Status:            domain.StatusPending,
	}
	repo.updateErr = fmt.Errorf("database error")

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
	if err == nil {
		t.Error("expected error from repository, got nil")
	}
	if err.Error() != "database error" {
		t.Errorf("expected 'database error', got '%v'", err)
	}
}

func TestValidateC2B_MissingBillRefNumber(t *testing.T) {
	repo := newMockRepo()
	uc := NewMpesaUsecase(repo, nil)

	payload := &domain.C2BPayload{
		TransID:     "TRX123",
		TransAmount: "100.00",
		MSISDN:      "254700000000",
	}

	resp, err := uc.ValidateC2B(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResultCode != 1 {
		t.Errorf("expected ResultCode 1 for missing BillRefNumber, got %d", resp.ResultCode)
	}
	if !strings.Contains(resp.ResultDesc, "Account Reference is required") {
		t.Errorf("expected error message about Account Reference, got '%s'", resp.ResultDesc)
	}
}

func TestProcessSTKCallback_DynamicResultCode(t *testing.T) {
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
				ResultCode:        "GV50113",
				ResultDesc:        "The receiver party information is invalid",
			},
		},
	}

	err := uc.ProcessSTKCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx := repo.transactions["ref-1"]
	if tx.Status != domain.StatusFailed {
		t.Errorf("expected status FAILED, got %s", tx.Status)
	}

	if tx.ResultCode != -1 {
		t.Errorf("expected ResultCode to be -1 for non-numeric string error, got %d", tx.ResultCode)
	}
}

func TestRegisterC2BURLs_Success(t *testing.T) {
	repo := newMockRepo()
	gateway := &mockDarajaGateway{
		registerC2BResp: &daraja.C2BRegisterResponse{
			ResponseDescription: "Success",
		},
	}
	uc := NewMpesaUsecase(repo, gateway)

	err := uc.RegisterC2BURLs(context.Background(), "https://example.com/val", "https://example.com/conf", "v2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegisterC2BURLs_Error(t *testing.T) {
	repo := newMockRepo()
	gateway := &mockDarajaGateway{
		registerC2BErr: fmt.Errorf("daraja registration failed"),
	}
	uc := NewMpesaUsecase(repo, gateway)

	err := uc.RegisterC2BURLs(context.Background(), "https://example.com/val", "https://example.com/conf", "v1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "daraja registration failed") {
		t.Errorf("expected error message to contain 'daraja registration failed', got: %v", err)
	}
}

