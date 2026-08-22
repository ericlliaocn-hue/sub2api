package provider

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const (
	huifuProdAPIBase     = "https://api.huifu.com"
	huifuTestAPIBase     = "https://spin-test.cloudpnr.com"
	huifuPreorderPath    = "/v2/trade/hosting/payment/preorder"
	huifuQueryOrderPath  = "/v2/trade/hosting/payment/queryorderinfo"
	huifuRefundPath      = "/v2/trade/hosting/payment/htRefund"
	huifuQueryRefundPath = "/v2/trade/hosting/payment/queryRefundInfo"
	huifuClosePath       = "/v2/trade/hosting/payment/close"
	huifuHTTPTimeout     = 15 * time.Second
	huifuMaxResponseSize = 1 << 20
	huifuSuccessCode     = "00000000"
	huifuMaxKeyFileSize  = 64 << 10

	huifuMerchantPrivateKeyFileEnv = "HUIFU_MERCHANT_PRIVATE_KEY_FILE"
	huifuPublicKeyFileEnv          = "HUIFU_PUBLIC_KEY_FILE"
)

// Huifu integrates Huifu Dougong's H5/PC hosted checkout. It deliberately
// implements the small provider surface used by sub2api instead of relying on
// the official SDK's process-global, file-based configuration.
type Huifu struct {
	instanceID string
	config     map[string]string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	httpClient *http.Client
}

func NewHuifu(instanceID string, config map[string]string) (*Huifu, error) {
	for _, key := range []string{"sysId", "huifuId", "productId", "projectId", "apiBase", "notifyUrl", "returnUrl"} {
		if strings.TrimSpace(config[key]) == "" {
			return nil, fmt.Errorf("huifu config missing required key: %s", key)
		}
	}
	apiBase, err := normalizeHuifuAPIBase(config["apiBase"])
	if err != nil {
		return nil, err
	}
	if err := validateHuifuCallbackURL("notifyUrl", config["notifyUrl"], false); err != nil {
		return nil, err
	}
	if err := validateHuifuCallbackURL("returnUrl", config["returnUrl"], true); err != nil {
		return nil, err
	}
	privateKeyPEM, err := readHuifuKeyFile(huifuMerchantPrivateKeyFileEnv, true)
	if err != nil {
		return nil, err
	}
	publicKeyPEM, err := readHuifuKeyFile(huifuPublicKeyFileEnv, false)
	if err != nil {
		return nil, err
	}
	privateKey, err := parseHuifuPrivateKey(string(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("huifu merchant private key file contains an invalid RSA key: %w", err)
	}
	publicKey, err := parseHuifuPublicKey(string(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("huifu public key file contains an invalid RSA key: %w", err)
	}
	cfg := cloneStringMap(config)
	cfg["apiBase"] = apiBase
	if strings.TrimSpace(cfg["skillSource"]) == "" {
		cfg["skillSource"] = "sub2api"
	}
	return &Huifu{
		instanceID: instanceID,
		config:     cfg,
		privateKey: privateKey,
		publicKey:  publicKey,
		httpClient: &http.Client{Timeout: huifuHTTPTimeout},
	}, nil
}

func readHuifuKeyFile(envName string, private bool) ([]byte, error) {
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" {
		return nil, fmt.Errorf("huifu server secret file is not configured: set %s", envName)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("huifu server secret file is unavailable for %s: %w", envName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("huifu server secret file for %s must be a regular file", envName)
	}
	if info.Size() > huifuMaxKeyFileSize {
		return nil, fmt.Errorf("huifu server secret file for %s exceeds %d bytes", envName, huifuMaxKeyFileSize)
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("huifu merchant private key file permissions are too broad: require 0400 or 0600")
	}
	if !private && info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("huifu public key file must not be writable by group or others")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read huifu server secret file for %s: %w", envName, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("huifu server secret file for %s is empty", envName)
	}
	return data, nil
}

func normalizeHuifuAPIBase(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("huifu apiBase must be an HTTPS URL")
	}
	if parsed.Host != "api.huifu.com" && parsed.Host != "spin-test.cloudpnr.com" {
		return "", fmt.Errorf("huifu apiBase host must be api.huifu.com or spin-test.cloudpnr.com")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("huifu apiBase must not include a path")
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", "", ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateHuifuCallbackURL(name, raw string, allowQuery bool) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("huifu %s must be an HTTP(S) URL", name)
	}
	if !allowQuery && parsed.RawQuery != "" {
		return fmt.Errorf("huifu %s must not include query parameters", name)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("huifu %s must not include a fragment", name)
	}
	return nil
}

func (h *Huifu) Name() string        { return "汇付斗拱" }
func (h *Huifu) ProviderKey() string { return payment.TypeHuifu }
func (h *Huifu) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeHuifu}
}

func (h *Huifu) MerchantIdentityMetadata() map[string]string {
	if h == nil {
		return nil
	}
	return map[string]string{"huifu_id": strings.TrimSpace(h.config["huifuId"])}
}

func (h *Huifu) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThan(decimal.NewFromFloat(0.01)) {
		return nil, fmt.Errorf("huifu create payment: invalid amount %s", req.Amount)
	}
	if strings.TrimSpace(req.OrderID) == "" {
		return nil, fmt.Errorf("huifu create payment: missing order id")
	}

	returnURL := strings.TrimSpace(req.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(h.config["returnUrl"])
	}
	if err := validateHuifuCallbackURL("returnUrl", returnURL, true); err != nil {
		return nil, err
	}
	projectTitle := strings.TrimSpace(h.config["projectTitle"])
	if projectTitle == "" {
		projectTitle = strings.TrimSpace(req.Subject)
	}
	projectTitle = truncateHuifuRunes(projectTitle, 64)
	hostingData := map[string]string{
		"project_id":    strings.TrimSpace(h.config["projectId"]),
		"project_title": projectTitle,
		"callback_url":  returnURL,
	}
	if req.IsMobile {
		hostingData["request_type"] = "M"
	} else {
		hostingData["request_type"] = "P"
	}
	hostingJSON, err := marshalHuifuJSON(hostingData)
	if err != nil {
		return nil, fmt.Errorf("huifu create payment hosting_data: %w", err)
	}
	data := map[string]any{
		"req_date":       huifuOrderDate(req.OrderID),
		"req_seq_id":     req.OrderID,
		"huifu_id":       strings.TrimSpace(h.config["huifuId"]),
		"trans_amt":      amount.StringFixed(2),
		"goods_desc":     truncateHuifuRunes(strings.TrimSpace(req.Subject), 40),
		"pre_order_type": "1",
		"hosting_data":   string(hostingJSON),
		"notify_url":     strings.TrimSpace(h.config["notifyUrl"]),
		"usage_type":     huifuUsageType(req.OrderType),
	}
	var response huifuPaymentData
	if err := h.request(ctx, huifuPreorderPath, data, &response); err != nil {
		return nil, fmt.Errorf("huifu create payment: %w", err)
	}
	if strings.TrimSpace(response.JumpURL) == "" {
		return nil, fmt.Errorf("huifu create payment: response missing jump_url")
	}
	return &payment.CreatePaymentResponse{
		PayURL:     response.JumpURL,
		Currency:   payment.DefaultPaymentCurrency,
		PaymentEnv: h.environment(),
	}, nil
}

func huifuUsageType(orderType string) string {
	if strings.EqualFold(strings.TrimSpace(orderType), payment.OrderTypeBalance) {
		return "R"
	}
	return "P"
}

func (h *Huifu) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, fmt.Errorf("huifu query order: missing original request id")
	}
	data := map[string]any{
		"req_date":       time.Now().Format("20060102"),
		"req_seq_id":     huifuRequestID("Q", tradeNo),
		"huifu_id":       strings.TrimSpace(h.config["huifuId"]),
		"org_req_date":   huifuOrderDate(tradeNo),
		"org_req_seq_id": tradeNo,
	}
	var response huifuPaymentData
	if err := h.request(ctx, huifuQueryOrderPath, data, &response); err != nil {
		return nil, fmt.Errorf("huifu query order: %w", err)
	}
	amount, _ := decimal.NewFromString(strings.TrimSpace(response.TransAmt))
	return &payment.QueryOrderResponse{
		TradeNo:  firstNonEmpty(response.OrgHFSeqID, response.HFSeqID, tradeNo),
		Status:   huifuPaymentStatus(response.TransStat, response.OrderStat),
		Amount:   amount.InexactFloat64(),
		PaidAt:   parseHuifuTime(response.TransTime, response.EndTime),
		Metadata: h.metadata(response.TransStat),
	}, nil
}

func (h *Huifu) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("huifu parse notification: %w", err)
	}
	respData := values.Get("resp_data")
	signature := values.Get("sign")
	if strings.TrimSpace(respData) == "" || strings.TrimSpace(signature) == "" {
		return nil, fmt.Errorf("huifu notification missing resp_data or sign")
	}
	if err := verifyHuifuSignature(h.publicKey, []byte(respData), signature); err != nil {
		return nil, fmt.Errorf("huifu notification signature: %w", err)
	}
	var response huifuPaymentData
	if err := json.Unmarshal([]byte(respData), &response); err != nil {
		return nil, fmt.Errorf("huifu parse notification data: %w", err)
	}
	if strings.TrimSpace(response.ReqSeqID) == "" || strings.TrimSpace(response.HuifuID) == "" {
		return nil, fmt.Errorf("huifu notification missing req_seq_id or huifu_id")
	}
	if !strings.EqualFold(strings.TrimSpace(response.HuifuID), strings.TrimSpace(h.config["huifuId"])) {
		return nil, fmt.Errorf("huifu notification huifu_id mismatch")
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(response.TransAmt))
	if err != nil || amount.IsNegative() {
		return nil, fmt.Errorf("huifu notification invalid trans_amt")
	}
	var status string
	switch strings.ToUpper(strings.TrimSpace(response.TransStat)) {
	case "S":
		status = payment.NotificationStatusSuccess
	case "F":
		status = payment.ProviderStatusFailed
	default:
		return nil, fmt.Errorf("huifu notification unsupported trans_stat: %s", response.TransStat)
	}
	return &payment.PaymentNotification{
		TradeNo:  firstNonEmpty(response.HFSeqID, response.OrgHFSeqID),
		OrderID:  response.ReqSeqID,
		Amount:   amount.InexactFloat64(),
		Status:   status,
		RawData:  rawBody,
		Metadata: h.metadata(response.TransStat),
	}, nil
}

func (h *Huifu) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThan(decimal.NewFromFloat(0.01)) {
		return nil, fmt.Errorf("huifu refund: invalid amount %s", req.Amount)
	}
	orderID := strings.TrimSpace(req.OrderID)
	if orderID == "" {
		return nil, fmt.Errorf("huifu refund: missing original order id")
	}
	refundDate := time.Now().Format("20060102")
	refundID := huifuRefundID(refundDate, orderID, req.Amount)
	data := map[string]any{
		"req_date":       refundDate,
		"req_seq_id":     refundID,
		"huifu_id":       strings.TrimSpace(h.config["huifuId"]),
		"ord_amt":        amount.StringFixed(2),
		"org_req_date":   huifuOrderDate(orderID),
		"org_req_seq_id": orderID,
	}
	if reason := truncateHuifuRunes(strings.TrimSpace(req.Reason), 84); reason != "" {
		data["remark"] = reason
	}
	for _, key := range []string{"riskCheckData", "terminalDeviceData"} {
		if value := strings.TrimSpace(h.config[key]); value != "" {
			if !json.Valid([]byte(value)) {
				return nil, fmt.Errorf("huifu refund: %s must be valid JSON", key)
			}
			protocolKey := "risk_check_data"
			if key == "terminalDeviceData" {
				protocolKey = "terminal_device_data"
			}
			data[protocolKey] = value
		}
	}
	var response huifuPaymentData
	if err := h.request(ctx, huifuRefundPath, data, &response); err != nil {
		return nil, fmt.Errorf("huifu refund: %w", err)
	}
	status := huifuRefundStatus(response.TransStat)
	return &payment.RefundResponse{RefundID: refundID, Status: status}, nil
}

func (h *Huifu) QueryRefund(ctx context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	refundID := strings.TrimSpace(req.RefundID)
	refundDate, ok := huifuRefundDate(refundID)
	if !ok {
		return nil, fmt.Errorf("huifu query refund: invalid refund id")
	}
	data := map[string]any{
		"req_date":       time.Now().Format("20060102"),
		"req_seq_id":     huifuRequestID("RQ", refundID),
		"huifu_id":       strings.TrimSpace(h.config["huifuId"]),
		"org_req_date":   refundDate,
		"org_req_seq_id": refundID,
	}
	var response huifuPaymentData
	if err := h.request(ctx, huifuQueryRefundPath, data, &response); err != nil {
		return nil, fmt.Errorf("huifu query refund: %w", err)
	}
	return &payment.RefundResponse{RefundID: refundID, Status: huifuRefundStatus(response.TransStat)}, nil
}

func (h *Huifu) CancelPayment(ctx context.Context, tradeNo string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil
	}
	data := map[string]any{
		"req_date":       time.Now().Format("20060102"),
		"req_seq_id":     huifuRequestID("C", tradeNo),
		"huifu_id":       strings.TrimSpace(h.config["huifuId"]),
		"org_req_date":   huifuOrderDate(tradeNo),
		"org_req_seq_id": tradeNo,
	}
	var response huifuPaymentData
	if err := h.request(ctx, huifuClosePath, data, &response); err != nil {
		return fmt.Errorf("huifu cancel payment: %w", err)
	}
	return nil
}

func (h *Huifu) request(ctx context.Context, path string, data map[string]any, out any) error {
	dataJSON, err := marshalHuifuJSON(data)
	if err != nil {
		return err
	}
	signature, err := signHuifuPayload(h.privateKey, dataJSON)
	if err != nil {
		return err
	}
	body, err := marshalHuifuJSON(map[string]any{
		"sys_id":     strings.TrimSpace(h.config["sysId"]),
		"product_id": strings.TrimSpace(h.config["productId"]),
		"data":       json.RawMessage(dataJSON),
		"sign":       signature,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.config["apiBase"]+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("jpt-x-skill-source", h.config["skillSource"])
	req.Header.Set("jpt-x-skill-huifu_id", h.config["huifuId"])
	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: huifuHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, huifuMaxResponseSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, huifuResponseSummary(respBody))
	}
	var envelope huifuEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	dataBody, signedBody, err := huifuEnvelopeData(envelope.Data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Sign) == "" {
		return fmt.Errorf("response missing signature")
	}
	if err := verifyHuifuSignature(h.publicKey, signedBody, envelope.Sign); err != nil {
		return fmt.Errorf("response signature: %w", err)
	}
	var header huifuResponseHeader
	if err := json.Unmarshal(dataBody, &header); err != nil {
		return fmt.Errorf("parse response data: %w", err)
	}
	if header.RespCode != huifuSuccessCode {
		return fmt.Errorf("response code %s: %s", header.RespCode, header.RespDesc)
	}
	if out != nil {
		if err := json.Unmarshal(dataBody, out); err != nil {
			return fmt.Errorf("parse response data: %w", err)
		}
	}
	return nil
}

func huifuEnvelopeData(raw json.RawMessage) ([]byte, []byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("response missing data")
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, nil, fmt.Errorf("parse response data string: %w", err)
		}
		return []byte(value), []byte(value), nil
	}
	return raw, raw, nil
}

func signHuifuPayload(privateKey *rsa.PrivateKey, payload []byte) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("private key is not configured")
	}
	hash := sha256.Sum256(payload)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyHuifuSignature(publicKey *rsa.PublicKey, payload []byte, signature string) error {
	if publicKey == nil {
		return fmt.Errorf("public key is not configured")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	hash := sha256.Sum256(payload)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], decoded); err != nil {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func parseHuifuPrivateKey(raw string) (*rsa.PrivateKey, error) {
	der, blockType, err := decodeHuifuPEM(raw, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	if blockType == "RSA PRIVATE KEY" {
		if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			return key, nil
		}
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if key, ok := parsed.(*rsa.PrivateKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("unsupported RSA private key format")
}

func parseHuifuPublicKey(raw string) (*rsa.PublicKey, error) {
	der, _, err := decodeHuifuPEM(raw, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	if parsed, err := x509.ParsePKIXPublicKey(der); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	if cert, err := x509.ParseCertificate(der); err == nil {
		if key, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	return nil, fmt.Errorf("unsupported RSA public key format")
}

func decodeHuifuPEM(raw, label string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("key is empty")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		return block.Bytes, block.Type, nil
	}
	compact := strings.Join(strings.Fields(raw), "")
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, "", fmt.Errorf("invalid PEM/base64 key")
	}
	return der, label, nil
}

func marshalHuifuJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func huifuPaymentStatus(transStat, orderStat string) string {
	switch strings.ToUpper(strings.TrimSpace(transStat)) {
	case "S":
		return payment.ProviderStatusPaid
	case "F":
		return payment.ProviderStatusFailed
	}
	switch strings.TrimSpace(orderStat) {
	case "1":
		return payment.ProviderStatusPaid
	case "3":
		return payment.ProviderStatusRefunded
	case "5":
		return payment.ProviderStatusFailed
	default:
		return payment.ProviderStatusPending
	}
}

func huifuRefundStatus(transStat string) string {
	switch strings.ToUpper(strings.TrimSpace(transStat)) {
	case "S":
		return payment.ProviderStatusSuccess
	case "F":
		return payment.ProviderStatusFailed
	default:
		return payment.ProviderStatusPending
	}
}

func huifuOrderDate(orderID string) string {
	orderID = strings.TrimSpace(orderID)
	if strings.HasPrefix(orderID, "sub2_") && len(orderID) >= len("sub2_")+8 {
		candidate := orderID[len("sub2_") : len("sub2_")+8]
		if _, err := time.Parse("20060102", candidate); err == nil {
			return candidate
		}
	}
	return time.Now().Format("20060102")
}

func huifuRequestID(prefix, source string) string {
	date := time.Now().Format("20060102")
	hash := sha256.Sum256([]byte(prefix + "\x00" + source + "\x00" + date))
	return prefix + date + hex.EncodeToString(hash[:20])
}

func huifuRefundID(date, orderID, amount string) string {
	hash := sha256.Sum256([]byte(orderID + "\x00" + amount))
	return "HFR" + date + hex.EncodeToString(hash[:20])
}

func huifuRefundDate(refundID string) (string, bool) {
	if !strings.HasPrefix(refundID, "HFR") || len(refundID) < 11 {
		return "", false
	}
	date := refundID[3:11]
	_, err := time.Parse("20060102", date)
	return date, err == nil
}

func parseHuifuTime(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) != 14 {
			continue
		}
		if parsed, err := time.ParseInLocation("20060102150405", value, time.Local); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return ""
}

func truncateHuifuRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *Huifu) metadata(transStat string) map[string]string {
	return map[string]string{
		"huifu_id":   strings.TrimSpace(h.config["huifuId"]),
		"trans_stat": strings.ToUpper(strings.TrimSpace(transStat)),
	}
}

func (h *Huifu) environment() string {
	if strings.EqualFold(strings.TrimSpace(h.config["apiBase"]), huifuProdAPIBase) {
		return "prod"
	}
	return "test"
}

func huifuResponseSummary(body []byte) string {
	summary := strings.Join(strings.Fields(string(body)), " ")
	if len(summary) > 512 {
		return summary[:512] + "..."
	}
	return summary
}

type huifuEnvelope struct {
	Sign string          `json:"sign"`
	Data json.RawMessage `json:"data"`
}

type huifuResponseHeader struct {
	RespCode string `json:"resp_code"`
	RespDesc string `json:"resp_desc"`
}

type huifuPaymentData struct {
	RespCode   string `json:"resp_code"`
	RespDesc   string `json:"resp_desc"`
	ReqDate    string `json:"req_date"`
	ReqSeqID   string `json:"req_seq_id"`
	HuifuID    string `json:"huifu_id"`
	HFSeqID    string `json:"hf_seq_id"`
	OrgHFSeqID string `json:"org_hf_seq_id"`
	JumpURL    string `json:"jump_url"`
	TransAmt   string `json:"trans_amt"`
	OrdAmt     string `json:"ord_amt"`
	TransStat  string `json:"trans_stat"`
	OrderStat  string `json:"order_stat"`
	TransTime  string `json:"trans_time"`
	EndTime    string `json:"end_time"`
}
