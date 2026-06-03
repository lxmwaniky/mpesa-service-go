package http

import (
	"net/http"

	"github.com/lxmwaniky/mpesa-service-go/config"
	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
)

func NewRouter(usecase domain.MpesaUsecase, cfg *config.Config) http.Handler {
	mux := http.NewServeMux()
	handler := NewHandler(usecase)

	authMiddleware := APIKeyAuth(cfg)
	stkPushHandler := http.HandlerFunc(handler.InitiateSTKPush)
	getStatusHandler := http.HandlerFunc(handler.GetStatus)

	mux.Handle("POST /api/v1/mpesa/stk-push", authMiddleware(stkPushHandler))
	mux.Handle("GET /api/v1/mpesa/status/{extRef}", authMiddleware(getStatusHandler))

	mux.HandleFunc("POST /api/v1/mpesa/callbacks/stk", handler.HandleSTKCallback)
	mux.HandleFunc("POST /api/v1/mpesa/callbacks/c2b/validation", handler.HandleC2BValidation)
	mux.HandleFunc("POST /api/v1/mpesa/callbacks/c2b/confirmation", handler.HandleC2BConfirmation)

	wrapped := RateLimit(mux)
	wrapped = Logger(wrapped)
	return EnableCORS(wrapped)
}