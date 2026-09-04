package http

import "net/http"

func (h *Handler) Docs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service":     "mpesa-service-go",
		"description": "M-Pesa Daraja backend for STK Push, C2B callbacks, and transaction status.",
		"auth": map[string]string{
			"type":   "api_key",
			"header": "X-API-Key",
		},
		"links": map[string]string{
			"openapi": "/openapi.json",
			"health":  "/health",
		},
		"endpoints": []map[string]interface{}{
			{
				"method":        "POST",
				"path":          "/api/v1/mpesa/stk-push",
				"auth_required": true,
				"description":   "Initiate an STK Push payment request.",
			},
			{
				"method":        "GET",
				"path":          "/api/v1/mpesa/status/{extRef}",
				"auth_required": true,
				"description":   "Fetch transaction status by external reference.",
			},
			{
				"method":        "POST",
				"path":          "/api/v1/mpesa/c2b/register",
				"auth_required": true,
				"description":   "Register C2B validation and confirmation callback URLs.",
			},
			{
				"method":        "POST",
				"path":          "/api/v1/mpesa/callbacks/stk",
				"auth_required": false,
				"description":   "Receive Daraja STK callback notifications.",
			},
			{
				"method":        "POST",
				"path":          "/api/v1/mpesa/callbacks/c2b/validation",
				"auth_required": false,
				"description":   "Receive Daraja C2B validation callbacks.",
			},
			{
				"method":        "POST",
				"path":          "/api/v1/mpesa/callbacks/c2b/confirmation",
				"auth_required": false,
				"description":   "Receive Daraja C2B confirmation callbacks.",
			},
			{
				"method":        "GET",
				"path":          "/health",
				"auth_required": false,
				"description":   "Check database connectivity.",
			},
		},
		"environment": map[string][]string{
			"secrets": {
				"DATABASE_URL",
				"MPESA_CONSUMER_KEY",
				"MPESA_CONSUMER_SECRET",
				"MPESA_PASSKEY",
				"API_KEY",
			},
			"variables": {
				"PORT",
				"MPESA_ENV",
				"MPESA_SHORTCODE",
				"MPESA_PARTY_B",
				"MPESA_TRANSACTION_TYPE",
				"MPESA_CALLBACK_URL",
			},
		},
	})
}

func (h *Handler) OpenAPI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openAPIDocument())
}

func openAPIDocument() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]string{
			"title":       "M-Pesa Service API",
			"version":     "1.0.0",
			"description": "M-Pesa Daraja backend for STK Push, C2B callbacks, and transaction status.",
		},
		"servers": []map[string]string{
			{"url": "http://localhost:8080"},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"apiKeyAuth": map[string]string{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
			},
		},
		"paths": map[string]interface{}{
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Health check",
					"responses": map[string]interface{}{
						"200": response("Service is healthy"),
						"503": response("Service is unavailable"),
					},
				},
			},
			"/api/v1/mpesa/stk-push": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Initiate STK Push",
					"security": apiKeySecurity(),
					"requestBody": jsonBody(map[string]interface{}{
						"type":     "object",
						"required": []string{"external_reference", "phone_number", "amount"},
						"properties": map[string]interface{}{
							"external_reference": schema("string", "Your stable payment reference"),
							"phone_number":       schema("string", "Customer phone number in 2547XXXXXXXX format"),
							"amount":             schema("number", "Payment amount"),
							"description":        schema("string", "Payment description"),
						},
					}),
					"responses": map[string]interface{}{
						"202": response("STK Push accepted"),
						"400": response("Invalid request"),
						"401": response("Unauthorized"),
						"500": response("STK Push failed"),
					},
				},
			},
			"/api/v1/mpesa/status/{extRef}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Get transaction status",
					"security": apiKeySecurity(),
					"parameters": []map[string]interface{}{
						{
							"name":        "extRef",
							"in":          "path",
							"required":    true,
							"description": "External transaction reference",
							"schema":      map[string]string{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": response("Transaction status"),
						"401": response("Unauthorized"),
						"404": response("Transaction not found"),
						"500": response("Status lookup failed"),
					},
				},
			},
			"/api/v1/mpesa/c2b/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":  "Register C2B URLs",
					"security": apiKeySecurity(),
					"requestBody": jsonBody(map[string]interface{}{
						"type":     "object",
						"required": []string{"validation_url", "confirmation_url"},
						"properties": map[string]interface{}{
							"validation_url":   schema("string", "HTTPS validation callback URL"),
							"confirmation_url": schema("string", "HTTPS confirmation callback URL"),
							"api_version":      schema("string", "Daraja API version"),
						},
					}),
					"responses": map[string]interface{}{
						"200": response("C2B URLs registered"),
						"400": response("Invalid request"),
						"401": response("Unauthorized"),
						"500": response("Registration failed"),
					},
				},
			},
			"/api/v1/mpesa/callbacks/stk":              callbackPath("Receive STK callback"),
			"/api/v1/mpesa/callbacks/c2b/validation":   callbackPath("Receive C2B validation callback"),
			"/api/v1/mpesa/callbacks/c2b/confirmation": callbackPath("Receive C2B confirmation callback"),
		},
	}
}

func apiKeySecurity() []map[string][]string {
	return []map[string][]string{{"apiKeyAuth": {}}}
}

func jsonBody(schema map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": schema,
			},
		},
	}
}

func callbackPath(summary string) map[string]interface{} {
	return map[string]interface{}{
		"post": map[string]interface{}{
			"summary": summary,
			"responses": map[string]interface{}{
				"200": response("Callback processed"),
				"400": response("Invalid callback payload"),
				"500": response("Callback processing failed"),
			},
		},
	}
}

func response(description string) map[string]string {
	return map[string]string{"description": description}
}

func schema(typ, description string) map[string]string {
	return map[string]string{
		"type":        typ,
		"description": description,
	}
}
