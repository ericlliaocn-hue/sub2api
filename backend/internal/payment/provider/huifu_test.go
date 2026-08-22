package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestHuifuCreatePaymentSignsRequestAndUsesHostedCheckout(t *testing.T) {
	merchantPrivate := mustHuifuRSAKey(t)
	huifuPrivate := mustHuifuRSAKey(t)
	provider, err := NewHuifu("huifu-1", huifuTestConfig(t, merchantPrivate, &huifuPrivate.PublicKey))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, huifuPreorderPath, r.URL.Path)
		require.Equal(t, "sub2api", r.Header.Get("jpt-x-skill-source"))
		require.Equal(t, "6666000232494579", r.Header.Get("jpt-x-skill-huifu_id"))

		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		var envelope struct {
			SysID     string          `json:"sys_id"`
			ProductID string          `json:"product_id"`
			Data      json.RawMessage `json:"data"`
			Sign      string          `json:"sign"`
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		require.Equal(t, "6666000230207702", envelope.SysID)
		require.Equal(t, "product-1", envelope.ProductID)
		require.NoError(t, verifyHuifuSignature(&merchantPrivate.PublicKey, envelope.Data, envelope.Sign))

		var data map[string]any
		require.NoError(t, json.Unmarshal(envelope.Data, &data))
		require.Equal(t, "sub2_20260822_order1", data["req_seq_id"])
		require.Equal(t, "R", data["usage_type"])
		require.Equal(t, "12.34", data["trans_amt"])
		require.Equal(t, "https://pay.example.com/api/v1/payment/webhook/huifu", data["notify_url"])

		var hosting map[string]string
		require.NoError(t, json.Unmarshal([]byte(data["hosting_data"].(string)), &hosting))
		require.Equal(t, "PROJECTID2026082039375179", hosting["project_id"])
		require.Equal(t, "M", hosting["request_type"])
		require.Equal(t, "https://pay.example.com/payment/result", hosting["callback_url"])

		responseData, marshalErr := marshalHuifuJSON(map[string]any{
			"resp_code": "00000000",
			"resp_desc": "success",
			"jump_url":  "https://cashier.example.com/session/1",
		})
		require.NoError(t, marshalErr)
		responseSign, signErr := signHuifuPayload(huifuPrivate, responseData)
		require.NoError(t, signErr)
		responseBody, marshalErr := marshalHuifuJSON(map[string]any{"data": string(responseData), "sign": responseSign})
		require.NoError(t, marshalErr)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(server.Close)
	provider.config["apiBase"] = server.URL

	result, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:   "sub2_20260822_order1",
		Amount:    "12.34",
		Subject:   "余额充值",
		ReturnURL: "https://pay.example.com/payment/result",
		IsMobile:  true,
		OrderType: payment.OrderTypeBalance,
	})
	require.NoError(t, err)
	require.Equal(t, "https://cashier.example.com/session/1", result.PayURL)
	require.Equal(t, "CNY", result.Currency)
	require.Equal(t, "test", result.PaymentEnv)
}

func TestHuifuVerifyNotificationRequiresSignatureAndFinalStatus(t *testing.T) {
	merchantPrivate := mustHuifuRSAKey(t)
	huifuPrivate := mustHuifuRSAKey(t)
	provider, err := NewHuifu("huifu-1", huifuTestConfig(t, merchantPrivate, &huifuPrivate.PublicKey))
	require.NoError(t, err)

	respData := `{"req_seq_id":"sub2_20260822_order2","huifu_id":"6666000232494579","hf_seq_id":"hf-2","trans_amt":"8.88","trans_stat":"S"}`
	signature, err := signHuifuPayload(huifuPrivate, []byte(respData))
	require.NoError(t, err)
	form := url.Values{"resp_data": {respData}, "sign": {signature}}

	notification, err := provider.VerifyNotification(context.Background(), form.Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, "sub2_20260822_order2", notification.OrderID)
	require.Equal(t, "hf-2", notification.TradeNo)
	require.Equal(t, payment.NotificationStatusSuccess, notification.Status)
	require.Equal(t, 8.88, notification.Amount)
	require.Equal(t, "6666000232494579", notification.Metadata["huifu_id"])

	tampered := url.Values{"resp_data": {respData + " "}, "sign": {signature}}
	_, err = provider.VerifyNotification(context.Background(), tampered.Encode(), nil)
	require.ErrorContains(t, err, "invalid signature")

	pendingData := `{"req_seq_id":"sub2_20260822_order2","huifu_id":"6666000232494579","trans_amt":"8.88","trans_stat":"P"}`
	pendingSign, err := signHuifuPayload(huifuPrivate, []byte(pendingData))
	require.NoError(t, err)
	_, err = provider.VerifyNotification(context.Background(), url.Values{"resp_data": {pendingData}, "sign": {pendingSign}}.Encode(), nil)
	require.ErrorContains(t, err, "unsupported trans_stat")
}

func TestNewHuifuRejectsUnknownAPIHost(t *testing.T) {
	merchantPrivate := mustHuifuRSAKey(t)
	huifuPrivate := mustHuifuRSAKey(t)
	config := huifuTestConfig(t, merchantPrivate, &huifuPrivate.PublicKey)
	config["apiBase"] = "https://example.com"

	_, err := NewHuifu("huifu-1", config)
	require.ErrorContains(t, err, "apiBase host")
}

func TestNewHuifuRequiresServerSecretFiles(t *testing.T) {
	t.Setenv(huifuMerchantPrivateKeyFileEnv, "")
	t.Setenv(huifuPublicKeyFileEnv, "")

	_, err := NewHuifu("huifu-1", huifuConfigWithoutKeys())
	require.ErrorContains(t, err, huifuMerchantPrivateKeyFileEnv)
}

func TestNewHuifuRejectsBroadPrivateKeyPermissions(t *testing.T) {
	merchantPrivate := mustHuifuRSAKey(t)
	huifuPrivate := mustHuifuRSAKey(t)
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "merchant-private.pem")
	publicPath := filepath.Join(dir, "huifu-public.pem")
	require.NoError(t, os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(merchantPrivate)}), 0o644))
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustMarshalPKIX(&huifuPrivate.PublicKey)}), 0o644))
	t.Setenv(huifuMerchantPrivateKeyFileEnv, privatePath)
	t.Setenv(huifuPublicKeyFileEnv, publicPath)

	_, err := NewHuifu("huifu-1", huifuConfigWithoutKeys())
	require.ErrorContains(t, err, "permissions are too broad")
}

func huifuTestConfig(t *testing.T, merchantPrivate *rsa.PrivateKey, huifuPublic *rsa.PublicKey) map[string]string {
	t.Helper()
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "merchant-private.pem")
	publicPath := filepath.Join(dir, "huifu-public.pem")
	require.NoError(t, os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalPKCS8(merchantPrivate)}), 0o600))
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustMarshalPKIX(huifuPublic)}), 0o644))
	t.Setenv(huifuMerchantPrivateKeyFileEnv, privatePath)
	t.Setenv(huifuPublicKeyFileEnv, publicPath)
	return huifuConfigWithoutKeys()
}

func huifuConfigWithoutKeys() map[string]string {
	return map[string]string{
		"sysId":        "6666000230207702",
		"huifuId":      "6666000232494579",
		"productId":    "product-1",
		"projectId":    "PROJECTID2026082039375179",
		"apiBase":      huifuTestAPIBase,
		"notifyUrl":    "https://pay.example.com/api/v1/payment/webhook/huifu",
		"returnUrl":    "https://pay.example.com/payment/result",
		"projectTitle": "Anytoken",
	}
}

func mustHuifuRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func mustMarshalPKCS8(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return der
}

func mustMarshalPKIX(key *rsa.PublicKey) []byte {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		panic(err)
	}
	return der
}
