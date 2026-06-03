package domain

import (
	"context"
	"time"
)

type TransactionStatus string

const (
	StatusPending TransactionStatus = "PENDING"
	StatusSuccess TransactionStatus = "SUCCESS"
	StatusFailed  TransactionStatus = "FAILED"
)

type Transaction struct {
	ID                 int64             `json:"id"`
	ExternalReference  string            `json:"external_reference"`
	MerchantRequestID  *string           `json:"merchant_request_id,omitempty"`
	CheckoutRequestID  *string           `json:"checkout_request_id,omitempty"`
	PhoneNumber        string            `json:"phone_number"`
	Amount             float64           `json:"amount"`
	MpesaReceiptNumber *string           `json:"mpesa_receipt_number,omitempty"`
	Status             TransactionStatus `json:"status"`
	ResultCode         int               `json:"result_code"`
	ResultDesc         string            `json:"result_desc,omitempty"`
	TransactionType    string            `json:"transaction_type"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type TransactionStatusResponse struct {
	ID                 int64             `json:"id"`
	ExternalReference  string            `json:"external_reference"`
	Status             TransactionStatus `json:"status"`
	MpesaReceiptNumber string            `json:"mpesa_receipt_number,omitempty"`
	UserMessage        string            `json:"user_message"`
	Amount             float64           `json:"amount"`
}

type STKCallbackMetadataItem struct {
	Name  string      `json:"Name"`
	Value interface{} `json:"Value,omitempty"`
}

type STKCallbackMetadata struct {
	Item []STKCallbackMetadataItem `json:"Item"`
}

type STKCallback struct {
	MerchantRequestID string               `json:"MerchantRequestID"`
	CheckoutRequestID string               `json:"CheckoutRequestID"`
	ResultCode        interface{}          `json:"ResultCode"`
	ResultDesc        string               `json:"ResultDesc"`
	CallbackMetadata  *STKCallbackMetadata `json:"CallbackMetadata,omitempty"`
}

type STKCallbackBody struct {
	StkCallback STKCallback `json:"stkCallback"`
}

type STKCallbackPayload struct {
	Body STKCallbackBody `json:"Body"`
}

type C2BPayload struct {
	TransactionType   string `json:"TransactionType"`
	TransID           string `json:"TransID"`
	TransTime         string `json:"TransTime"`
	TransAmount       string `json:"TransAmount"`
	BusinessShortCode string `json:"BusinessShortCode"`
	BillRefNumber     string `json:"BillRefNumber"`
	InvoiceNumber     string `json:"InvoiceNumber"`
	OrgAccountBalance string `json:"OrgAccountBalance"`
	ThirdPartyTransID string `json:"ThirdPartyTransID"`
	MSISDN            string `json:"MSISDN"`
	FirstName         string `json:"FirstName"`
	MiddleName        string `json:"MiddleName"`
	LastName          string `json:"LastName"`
}

type C2BValidationResponse struct {
	ResultCode int    `json:"ResultCode"`
	ResultDesc string `json:"ResultDesc"`
}

type MpesaUsecase interface {
	InitiateSTKPush(ctx context.Context, extRef, phone string, amount float64, desc string) (*Transaction, error)
	ProcessSTKCallback(ctx context.Context, payload *STKCallbackPayload) error
	ValidateC2B(ctx context.Context, payload *C2BPayload) (*C2BValidationResponse, error)
	ConfirmC2B(ctx context.Context, payload *C2BPayload) error
	GetTransactionStatus(ctx context.Context, extRef string) (*TransactionStatusResponse, error)
	RegisterC2BURLs(ctx context.Context, validationURL, confirmationURL string) error
	Ping(ctx context.Context) error
}