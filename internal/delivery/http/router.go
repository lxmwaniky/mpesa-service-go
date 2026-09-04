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
	registerC2BHandler := http.HandlerFunc(handler.RegisterC2BURLs)

	mux.HandleFunc("GET /docs", handler.Docs)
	mux.HandleFunc("GET /openapi.json", handler.OpenAPI)
	mux.Handle("POST /api/v1/mpesa/stk-push", authMiddleware(stkPushHandler))
	mux.Handle("GET /api/v1/mpesa/status/{extRef}", authMiddleware(getStatusHandler))
	mux.Handle("POST /api/v1/mpesa/c2b/register", authMiddleware(registerC2BHandler))
	mux.HandleFunc("GET /health", handler.health)

	mux.HandleFunc("POST /api/v1/mpesa/callbacks/stk", handler.HandleSTKCallback)
	mux.HandleFunc("POST /api/v1/mpesa/callbacks/c2b/validation", handler.HandleC2BValidation)
	mux.HandleFunc("POST /api/v1/mpesa/callbacks/c2b/confirmation", handler.HandleC2BConfirmation)

	wrapped := RateLimit(mux)
	wrapped = Logger(wrapped)
	return EnableCORS(wrapped)
}
