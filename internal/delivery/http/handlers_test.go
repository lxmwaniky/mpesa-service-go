package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
)

type mockMpesaUsecase struct {
	initiateFunc        func(ctx context.Context, extRef, phone string, amount float64, desc string) (*domain.Transaction, error)
	processFunc         func(ctx context.Context, payload *domain.STKCallbackPayload) error
	validateFunc        func(ctx context.Context, payload *domain.C2BPayload) (*domain.C2BValidationResponse, error)
	confirmFunc         func(ctx context.Context, payload *domain.C2BPayload) error
	statusFunc          func(ctx context.Context, extRef string) (*domain.TransactionStatusResponse, error)
	registerC2BURLsFunc func(ctx context.Context, validationURL, confirmationURL string, apiVersion string) error
	pingFunc            func(ctx context.Context) error
}

func (m *mockMpesaUsecase) InitiateSTKPush(ctx context.Context, extRef, phone string, amount float64, desc string) (*domain.Transaction, error) {
	if m.initiateFunc != nil {
		return m.initiateFunc(ctx, extRef, phone, amount, desc)
	}
	return nil, nil
}

func (m *mockMpesaUsecase) ProcessSTKCallback(ctx context.Context, payload *domain.STKCallbackPayload) error {
	if m.processFunc != nil {
		return m.processFunc(ctx, payload)
	}
	return nil
}

func (m *mockMpesaUsecase) ValidateC2B(ctx context.Context, payload *domain.C2BPayload) (*domain.C2BValidationResponse, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, payload)
	}
	return nil, nil
}

func (m *mockMpesaUsecase) ConfirmC2B(ctx context.Context, payload *domain.C2BPayload) error {
	if m.confirmFunc != nil {
		return m.confirmFunc(ctx, payload)
	}
	return nil
}

func (m *mockMpesaUsecase) GetTransactionStatus(ctx context.Context, extRef string) (*domain.TransactionStatusResponse, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx, extRef)
	}
	return nil, nil
}

func (m *mockMpesaUsecase) RegisterC2BURLs(ctx context.Context, validationURL, confirmationURL string, apiVersion string) error {
	if m.registerC2BURLsFunc != nil {
		return m.registerC2BURLsFunc(ctx, validationURL, confirmationURL, apiVersion)
	}
	return nil
}

func (m *mockMpesaUsecase) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func TestHandler_InitiateSTKPush_Success(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		initiateFunc: func(ctx context.Context, extRef, phone string, amount float64, desc string) (*domain.Transaction, error) {
			return &domain.Transaction{
				ID:                1,
				ExternalReference: extRef,
				PhoneNumber:       phone,
				Amount:            amount,
				Status:            domain.StatusPending,
			}, nil
		},
	}

	handler := NewHandler(mockUC)

	reqBody := STKInitiateRequest{
		ExternalReference: "ref-123",
		PhoneNumber:       "254712345678",
		Amount:            100.0,
		Description:       "test payment",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/mpesa/stk-push", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.InitiateSTKPush(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, rr.Code)
	}

	var resp domain.Transaction
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ExternalReference != "ref-123" {
		t.Errorf("expected external reference 'ref-123', got %s", resp.ExternalReference)
	}
}

func TestHandler_InitiateSTKPush_BadRequest(t *testing.T) {
	handler := NewHandler(&mockMpesaUsecase{})

	req := httptest.NewRequest("POST", "/api/v1/mpesa/stk-push", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	handler.InitiateSTKPush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandler_HandleSTKCallback_Success(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		processFunc: func(ctx context.Context, payload *domain.STKCallbackPayload) error {
			return nil
		},
	}

	handler := NewHandler(mockUC)

	payload := domain.STKCallbackPayload{
		Body: domain.STKCallbackBody{
			StkCallback: domain.STKCallback{
				CheckoutRequestID: "checkout-123",
				ResultCode:        0,
				ResultDesc:        "Success",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/mpesa/callbacks/stk", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.HandleSTKCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandler_GetStatus_Success(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		statusFunc: func(ctx context.Context, extRef string) (*domain.TransactionStatusResponse, error) {
			return &domain.TransactionStatusResponse{
				ID:                 1,
				ExternalReference:  extRef,
				Status:             domain.StatusSuccess,
				MpesaReceiptNumber: "REC123",
				UserMessage:        "Success",
				Amount:             10.0,
			}, nil
		},
	}

	handler := NewHandler(mockUC)

	req := httptest.NewRequest("GET", "/api/v1/mpesa/status/ref-123", nil)
	req.SetPathValue("extRef", "ref-123")
	rr := httptest.NewRecorder()

	handler.GetStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandler_Healthz_Success(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}

	handler := NewHandler(mockUC)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandler_Healthz_Failure(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		pingFunc: func(ctx context.Context) error {
			return errors.New("db error")
		},
	}

	handler := NewHandler(mockUC)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.Healthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestHandler_RegisterC2BURLs_Success(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		registerC2BURLsFunc: func(ctx context.Context, validationURL, confirmationURL string, apiVersion string) error {
			if apiVersion != "v2" {
				return errors.New("expected apiVersion 'v2'")
			}
			return nil
		},
	}

	handler := NewHandler(mockUC)

	payload := C2BRegisterRequest{
		ValidationURL:   "https://example.com/val",
		ConfirmationURL: "https://example.com/conf",
		APIVersion:      "v2",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/mpesa/c2b/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.RegisterC2BURLs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["message"] != "C2B URLs registered successfully" {
		t.Errorf("expected message 'C2B URLs registered successfully', got '%s'", resp["message"])
	}
}

func TestHandler_RegisterC2BURLs_InvalidPayload(t *testing.T) {
	handler := NewHandler(&mockMpesaUsecase{})

	req := httptest.NewRequest("POST", "/api/v1/mpesa/c2b/register", bytes.NewBufferString("{invalid json"))
	rr := httptest.NewRecorder()

	handler.RegisterC2BURLs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandler_RegisterC2BURLs_MissingFields(t *testing.T) {
	handler := NewHandler(&mockMpesaUsecase{})

	payload := C2BRegisterRequest{
		ValidationURL: "",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/mpesa/c2b/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.RegisterC2BURLs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandler_RegisterC2BURLs_Failure(t *testing.T) {
	mockUC := &mockMpesaUsecase{
		registerC2BURLsFunc: func(ctx context.Context, validationURL, confirmationURL string, apiVersion string) error {
			return errors.New("registration error")
		},
	}

	handler := NewHandler(mockUC)

	payload := C2BRegisterRequest{
		ValidationURL:   "https://example.com/val",
		ConfirmationURL: "https://example.com/conf",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/mpesa/c2b/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.RegisterC2BURLs(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

