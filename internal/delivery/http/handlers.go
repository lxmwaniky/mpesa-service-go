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

type Handler struct {
	usecase domain.MpesaUsecase
}

func NewHandler(usecase domain.MpesaUsecase) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) InitiateSTKPush(w http.ResponseWriter, r *http.Request) {
	var req STKInitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	if req.ExternalReference == "" || req.PhoneNumber == "" || req.Amount <= 0 {
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

	writeJSON(w, http.StatusAccepted, tx)
}

func (h *Handler) HandleSTKCallback(w http.ResponseWriter, r *http.Request) {
	var payload domain.STKCallbackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid callback structure"})
		return
	}

	err := h.usecase.ProcessSTKCallback(r.Context(), &payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ResultCode": 0,
		"ResultDesc": "Success",
	})
}

func (h *Handler) HandleC2BValidation(w http.ResponseWriter, r *http.Request) {
	var payload domain.C2BPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid validation structure"})
		return
	}

	resp, err := h.usecase.ValidateC2B(r.Context(), &payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleC2BConfirmation(w http.ResponseWriter, r *http.Request) {
	var payload domain.C2BPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid confirmation structure"})
		return
	}

	err := h.usecase.ConfirmC2B(r.Context(), &payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing transaction reference"})
		return
	}

	status, err := h.usecase.GetTransactionStatus(r.Context(), extRef)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if status == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Transaction not found"})
		return
	}

	writeJSON(w, http.StatusOK, status)
}