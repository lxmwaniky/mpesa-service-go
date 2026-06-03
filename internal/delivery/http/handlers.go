package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
)

type STKInitiateRequest struct {
	ExternalReference string  `json:"external_reference"`
	PhoneNumber       string  `json:"phone_number"`
	Amount            float64 `json:"amount"`
	Description       string  `json:"description"`
}

type C2BRegisterRequest struct {
	ValidationURL   string `json:"validation_url"`
	ConfirmationURL string `json:"confirmation_url"`
	APIVersion      string `json:"api_version"`
}

type Handler struct {
	usecase domain.MpesaUsecase
}

func NewHandler(usecase domain.MpesaUsecase) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) InitiateSTKPush(w http.ResponseWriter, r *http.Request) {
	var req STKInitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request payload for STK push", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	if req.ExternalReference == "" || req.PhoneNumber == "" || req.Amount <= 0 {
		slog.Warn("missing required fields for STK push",
			"external_reference", req.ExternalReference,
			"phone_number", req.PhoneNumber,
			"amount", req.Amount)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing required fields"})
		return
	}

	tx, err := h.usecase.InitiateSTKPush(r.Context(), req.ExternalReference, req.PhoneNumber, req.Amount, req.Description)
	if err != nil {
		slog.Error("stk push initiation failed",
			"phone", req.PhoneNumber,
			"reference", req.ExternalReference,
			"err", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("STK push initiated successfully",
		"phone", req.PhoneNumber,
		"reference", req.ExternalReference,
		"transaction_id", tx.ID)
	writeJSON(w, http.StatusAccepted, tx)
}

func (h *Handler) HandleSTKCallback(w http.ResponseWriter, r *http.Request) {
	var payload domain.STKCallbackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("invalid STK callback structure", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid callback structure"})
		return
	}

	err := h.usecase.ProcessSTKCallback(r.Context(), &payload)
	if err != nil {
		slog.Error("failed to process STK callback", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("STK callback processed successfully")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ResultCode": 0,
		"ResultDesc": "Success",
	})
}

func (h *Handler) HandleC2BValidation(w http.ResponseWriter, r *http.Request) {
	var payload domain.C2BPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("invalid C2B validation structure", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid validation structure"})
		return
	}

	resp, err := h.usecase.ValidateC2B(r.Context(), &payload)
	if err != nil {
		slog.Error("C2B validation failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("C2B validation processed")
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleC2BConfirmation(w http.ResponseWriter, r *http.Request) {
	var payload domain.C2BPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("invalid C2B confirmation structure", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid confirmation structure"})
		return
	}

	err := h.usecase.ConfirmC2B(r.Context(), &payload)
	if err != nil {
		slog.Error("C2B confirmation failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("C2B confirmation processed")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ResultCode": 0,
		"ResultDesc": "Success",
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	extRef := r.PathValue("extRef")
	if extRef == "" {
		slog.Warn("missing transaction reference in GET status request")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing transaction reference"})
		return
	}

	status, err := h.usecase.GetTransactionStatus(r.Context(), extRef)
	if err != nil {
		slog.Error("failed to get transaction status", "external_reference", extRef, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if status == nil {
		slog.Info("transaction not found", "external_reference", extRef)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Transaction not found"})
		return
	}

	slog.Info("transaction status retrieved", "external_reference", extRef, "status", status.Status)
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.Ping(r.Context()); err != nil {
		slog.Error("health check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "DOWN", "error": err.Error()})
		return
	}
	slog.Debug("health check passed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

func (h *Handler) RegisterC2BURLs(w http.ResponseWriter, r *http.Request) {
	var req C2BRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request payload for C2B register urls", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	if req.ValidationURL == "" || req.ConfirmationURL == "" {
		slog.Warn("missing required fields for C2B register urls",
			"validation_url", req.ValidationURL,
			"confirmation_url", req.ConfirmationURL)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing required fields"})
		return
	}

	err := h.usecase.RegisterC2BURLs(r.Context(), req.ValidationURL, req.ConfirmationURL, req.APIVersion)
	if err != nil {
		slog.Error("c2b register urls failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("C2B URLs registered successfully")
	writeJSON(w, http.StatusOK, map[string]string{"message": "C2B URLs registered successfully"})
}