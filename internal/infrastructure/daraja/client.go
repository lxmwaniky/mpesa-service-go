package daraja

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/config"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

type STKPushRequest struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	TransactionType   string `json:"TransactionType"`
	Amount            string `json:"Amount"`
	PartyA            string `json:"PartyA"`
	PartyB            string `json:"PartyB"`
	PhoneNumber       string `json:"PhoneNumber"`
	CallBackURL       string `json:"CallBackURL"`
	AccountReference  string `json:"AccountReference"`
	TransactionDesc   string `json:"TransactionDesc"`
}

type STKPushResponse struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
}

type Client struct {
	cfg         *config.Config
	httpClient  *http.Client
	tokenMutex  sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) getBaseURL() string {
	if c.cfg.MpesaEnv == "production" {
		return "https://api.safaricom.co.ke"
	}
	return "https://sandbox.safaricom.co.ke"
}

func (c *Client) GetToken() (string, error) {
	c.tokenMutex.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.tokenMutex.RUnlock()
		return token, nil
	}
	c.tokenMutex.RUnlock()

	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.cfg.MpesaConsumerKey + ":" + c.cfg.MpesaConsumerSecret))
	reqURL := fmt.Sprintf("%s/oauth/v1/generate?grant_type=client_credentials", c.getBaseURL())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Basic "+auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth failed with status: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(50 * time.Minute)

	return c.accessToken, nil
}

func (c *Client) SendSTKPush(ctx context.Context, phoneNumber string, amount float64, reference, description string) (*STKPushResponse, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	passMaterial := c.cfg.MpesaShortcode + c.cfg.MpesaPasskey + timestamp
	password := base64.StdEncoding.EncodeToString([]byte(passMaterial))

	formattedPhone := normalizePhone(phoneNumber)

	payload := STKPushRequest{
		BusinessShortCode: c.cfg.MpesaShortcode,
		Password:          password,
		Timestamp:         timestamp,
		TransactionType:   c.cfg.MpesaTransactionType,
		Amount:            fmt.Sprintf("%.2f", amount),
		PartyA:            formattedPhone,
		PartyB:            c.cfg.MpesaPartyB,
		PhoneNumber:       formattedPhone,
		CallBackURL:       c.cfg.MpesaCallbackURL,
		AccountReference:  reference,
		TransactionDesc:   description,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/mpesa/stkpush/v1/processrequest", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody bytes.Buffer
		_, _ = errorBody.ReadFrom(resp.Body)
		return nil, fmt.Errorf("stk push failed with status: %d, response: %s", resp.StatusCode, errorBody.String())
	}

	var stkResp STKPushResponse
	if err := json.NewDecoder(resp.Body).Decode(&stkResp); err != nil {
		return nil, err
	}

	return &stkResp, nil
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, "+", "")
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return "254" + phone[1:]
	}
	if strings.HasPrefix(phone, "7") && len(phone) == 9 {
		return "254" + phone
	}
	if strings.HasPrefix(phone, "1") && len(phone) == 9 {
		return "254" + phone
	}
	return phone
}

type C2BRegisterRequest struct {
	ShortCode       string `json:"ShortCode"`
	ResponseType    string `json:"ResponseType"`
	ConfirmationURL string `json:"ConfirmationURL"`
	ValidationURL   string `json:"ValidationURL"`
}

type C2BRegisterResponse struct {
	ConversationID           string `json:"ConversationID"`
	OriginatorConversationID string `json:"OriginatorConversationID"`
	ResponseDescription      string `json:"ResponseDescription"`
}

func (c *Client) RegisterC2BURLs(ctx context.Context, validationURL, confirmationURL string, apiVersion string) (*C2BRegisterResponse, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	payload := C2BRegisterRequest{
		ShortCode:       c.cfg.MpesaShortcode,
		ResponseType:    "Completed",
		ConfirmationURL: confirmationURL,
		ValidationURL:   validationURL,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	version := "v1"
	if apiVersion == "v2" {
		version = "v2"
	}

	reqURL := fmt.Sprintf("%s/mpesa/c2b/%s/registerurl", c.getBaseURL(), version)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("c2b url registration failed with status: %d", resp.StatusCode)
	}

	var regResp C2BRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, err
	}

	return &regResp, nil
}

type STKQueryRequest struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	CheckoutRequestID string `json:"CheckoutRequestID"`
}

type STKQueryResponse struct {
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResultCode          int    `json:"ResultCode"`
	ResultDesc          string `json:"ResultDesc"`
}

func (c *Client) QuerySTKPush(ctx context.Context, checkoutRequestID string) (*STKQueryResponse, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	passMaterial := c.cfg.MpesaShortcode + c.cfg.MpesaPasskey + timestamp
	password := base64.StdEncoding.EncodeToString([]byte(passMaterial))

	payload := STKQueryRequest{
		BusinessShortCode: c.cfg.MpesaShortcode,
		Password:          password,
		Timestamp:         timestamp,
		CheckoutRequestID: checkoutRequestID,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/mpesa/stkpushquery/v1/query", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody bytes.Buffer
		_, _ = errorBody.ReadFrom(resp.Body)
		return nil, fmt.Errorf("stk query failed with status: %d, response: %s", resp.StatusCode, errorBody.String())
	}

	var queryResp STKQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, err
	}

	return &queryResp, nil
}
