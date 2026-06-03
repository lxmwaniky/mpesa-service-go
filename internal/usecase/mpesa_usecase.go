package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/internal/domain"
	"github.com/lxmwaniky/mpesa-service-go/internal/infrastructure/daraja"
)

type DarajaGateway interface {
	SendSTKPush(ctx context.Context, phone string, amount float64, ref, desc string) (*daraja.STKPushResponse, error)
}

type mpesaUsecase struct {
	repo    domain.TransactionRepository
	gateway DarajaGateway
}

func NewMpesaUsecase(repo domain.TransactionRepository, gateway DarajaGateway) domain.MpesaUsecase {
	return &mpesaUsecase{
		repo:    repo,
		gateway: gateway,
	}
}

func (u *mpesaUsecase) InitiateSTKPush(ctx context.Context, extRef, phone string, amount float64, desc string) (*domain.Transaction, error) {
	existing, err := u.repo.GetByExternalReference(ctx, extRef)
	if err != nil {
		return nil, err
	}

	if existing != nil && (existing.Status == domain.StatusPending || existing.Status == domain.StatusSuccess) {
		return nil, fmt.Errorf("transaction with reference %s already exists and is %s", extRef, existing.Status)
	}

	tx := &domain.Transaction{
		ExternalReference: extRef,
		PhoneNumber:       phone,
		Amount:            amount,
		Status:            domain.StatusPending,
		TransactionType:   "STK_PUSH",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	stkResp, err := u.gateway.SendSTKPush(ctx, phone, amount, extRef, desc)
	if err != nil {
		tx.Status = domain.StatusFailed
		tx.ResultCode = -1
		tx.ResultDesc = err.Error()
		_ = u.repo.Create(ctx, tx)
		return nil, err
	}

	tx.MerchantRequestID = &stkResp.MerchantRequestID
	tx.CheckoutRequestID = &stkResp.CheckoutRequestID

	if err := u.repo.Create(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (u *mpesaUsecase) ProcessSTKCallback(ctx context.Context, payload *domain.STKCallbackPayload) error {
	checkoutID := payload.Body.StkCallback.CheckoutRequestID
	tx, err := u.repo.GetByCheckoutRequestID(ctx, checkoutID)
	if err != nil {
		return err
	}
	if tx == nil {
		return fmt.Errorf("transaction not found for checkout request id: %s", checkoutID)
	}

	if tx.Status != domain.StatusPending {
		return nil
	}

	tx.ResultCode = payload.Body.StkCallback.ResultCode
	tx.ResultDesc = payload.Body.StkCallback.ResultDesc
	tx.UpdatedAt = time.Now()

	if tx.ResultCode == 0 && payload.Body.StkCallback.CallbackMetadata != nil {
		tx.Status = domain.StatusSuccess
		receipt, _ := extractSTKMetadata(payload.Body.StkCallback.CallbackMetadata)
		if receipt != "" {
			tx.MpesaReceiptNumber = &receipt
		}
	} else {
		tx.Status = domain.StatusFailed
	}

	return u.repo.Update(ctx, tx)
}

func (u *mpesaUsecase) ValidateC2B(ctx context.Context, payload *domain.C2BPayload) (*domain.C2BValidationResponse, error) {
	if payload.BillRefNumber == "" {
		return &domain.C2BValidationResponse{
			ResultCode: 1,
			ResultDesc: "Rejected: Account Reference is required",
		}, nil
	}

	return &domain.C2BValidationResponse{
		ResultCode: 0,
		ResultDesc: "Accepted",
	}, nil
}

func (u *mpesaUsecase) ConfirmC2B(ctx context.Context, payload *domain.C2BPayload) error {
	amount, err := strconv.ParseFloat(payload.TransAmount, 64)
	if err != nil {
		return err
	}

	tx := &domain.Transaction{
		ExternalReference:  payload.BillRefNumber,
		PhoneNumber:        payload.MSISDN,
		Amount:             amount,
		MpesaReceiptNumber: &payload.TransID,
		Status:             domain.StatusSuccess,
		TransactionType:    "C2B",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	return u.repo.Create(ctx, tx)
}

func (u *mpesaUsecase) GetTransactionStatus(ctx context.Context, extRef string) (*domain.TransactionStatusResponse, error) {
	tx, err := u.repo.GetByExternalReference(ctx, extRef)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, nil
	}

	resp := &domain.TransactionStatusResponse{
		ID:                tx.ID,
		ExternalReference: tx.ExternalReference,
		Status:            tx.Status,
		Amount:            tx.Amount,
		UserMessage:       translateResultCode(tx.Status, tx.ResultCode, tx.ResultDesc),
	}

	if tx.MpesaReceiptNumber != nil {
		resp.MpesaReceiptNumber = *tx.MpesaReceiptNumber
	}

	return resp, nil
}

func translateResultCode(status domain.TransactionStatus, code int, rawDesc string) string {
	if status == domain.StatusPending {
		return "Payment prompt sent to phone. Please enter your PIN to complete."
	}
	if status == domain.StatusSuccess {
		return "Payment completed successfully."
	}

	switch code {
	case 1:
		return "Insufficient funds. Please top up your M-Pesa account."
	case 1032:
		return "You cancelled the payment prompt."
	case 2001:
		return "Incorrect M-Pesa PIN entered. Please try again."
	case 1037:
		return "Payment timed out. Please keep your phone unlocked."
	default:
		if rawDesc != "" {
			return rawDesc
		}
		return "Payment failed. Please try again."
	}
}

func extractSTKMetadata(metadata *domain.STKCallbackMetadata) (string, float64) {
	var receipt string
	var amount float64
	for _, item := range metadata.Item {
		switch item.Name {
		case "MpesaReceiptNumber":
			if val, ok := item.Value.(string); ok {
				receipt = val
			}
		case "Amount":
			if val, ok := item.Value.(float64); ok {
				amount = val
			}
		}
	}
	return receipt, amount
}
