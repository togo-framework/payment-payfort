// Package payfort is an Amazon PayFort / Amazon Payment Services driver for togo
// payment. Blank-import it and set PAYMENT_DRIVER=payfort, PAYFORT_ACCESS_CODE,
// PAYFORT_MERCHANT_IDENTIFIER, PAYFORT_SHA_REQUEST, PAYFORT_SHA_RESPONSE
// (+ optional PAYFORT_LANGUAGE, PAYFORT_SANDBOX=1). Implements the signed
// PURCHASE (CreateCharge with a token_name), REFUND and webhook verification.
// See https://paymentservices-reference.payfort.com/docs/api/build/index.html.
package payfort

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/togo-framework/payment"
	"github.com/togo-framework/togo"
)

const (
	prodAPI = "https://paymentservices.payfort.com/FortAPI/paymentApi"
	testAPI = "https://sbpaymentservices.payfort.com/FortAPI/paymentApi"
)

func init() {
	payment.RegisterDriver("payfort", func(k *togo.Kernel) (payment.PaymentProvider, error) {
		p := &provider{
			access:   os.Getenv("PAYFORT_ACCESS_CODE"),
			merchant: os.Getenv("PAYFORT_MERCHANT_IDENTIFIER"),
			shaReq:   os.Getenv("PAYFORT_SHA_REQUEST"),
			shaResp:  os.Getenv("PAYFORT_SHA_RESPONSE"),
			lang:     firstNonEmpty(os.Getenv("PAYFORT_LANGUAGE"), "en"),
			api:      prodAPI,
			hc:       &http.Client{Timeout: 20 * time.Second},
		}
		if os.Getenv("PAYFORT_SANDBOX") != "" {
			p.api = testAPI
		}
		if p.access == "" || p.merchant == "" || p.shaReq == "" || p.shaResp == "" {
			return nil, errors.New("payment-payfort: set PAYFORT_ACCESS_CODE, PAYFORT_MERCHANT_IDENTIFIER, PAYFORT_SHA_REQUEST, PAYFORT_SHA_RESPONSE")
		}
		return p, nil
	})
}

type provider struct {
	access, merchant, shaReq, shaResp, lang, api string
	hc                                           *http.Client
}

// Signature computes the PayFort SHA-256 signature: sorted key=value pairs
// concatenated with no separator, wrapped in the SHA phrase.
func Signature(phrase string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(phrase)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(params[k])
	}
	b.WriteString(phrase)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func (p *provider) call(ctx context.Context, params map[string]string) (map[string]any, error) {
	params["access_code"] = p.access
	params["merchant_identifier"] = p.merchant
	params["language"] = p.lang
	params["signature"] = Signature(p.shaReq, params)
	buf, _ := json.Marshal(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.api, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if rc, _ := m["response_code"].(string); rc != "" && !strings.HasSuffix(rc, "000") {
		return m, fmt.Errorf("payment-payfort: %s: %v", rc, m["response_message"])
	}
	if resp.StatusCode >= 300 {
		return m, fmt.Errorf("payment-payfort: http %d: %s", resp.StatusCode, string(b))
	}
	return m, nil
}

// CreateCharge runs a PURCHASE. r.Token is the PayFort token_name from a prior
// tokenization (the hosted/merchant-page or SDK step).
func (p *provider) CreateCharge(ctx context.Context, r payment.ChargeRequest) (*payment.Charge, error) {
	if r.Token == "" {
		return nil, errors.New("payment-payfort: CreateCharge needs a token_name (tokenize via the PayFort merchant page/SDK first)")
	}
	params := map[string]string{
		"command":           "PURCHASE",
		"merchant_reference": fmt.Sprintf("ref-%d", time.Now().UnixNano()),
		"amount":            strconv.FormatInt(r.Amount.Amount, 10),
		"currency":          strings.ToUpper(r.Amount.Currency),
		"customer_email":    firstNonEmpty(r.Customer.Email, "noreply@example.com"),
		"token_name":        r.Token,
	}
	m, err := p.call(ctx, params)
	if err != nil {
		return nil, err
	}
	id, _ := m["fort_id"].(string)
	status := "pending"
	if s, _ := m["status"].(string); s == "14" {
		status = "succeeded"
	} else if s != "" {
		status = "failed"
	}
	return &payment.Charge{ID: id, Status: status, Amount: r.Amount, Provider: "payfort", Raw: m}, nil
}

func (p *provider) Refund(ctx context.Context, r payment.RefundRequest) error {
	params := map[string]string{
		"command":  "REFUND",
		"fort_id":  r.ChargeID,
		"currency": "USD",
		"amount":   "0",
	}
	if r.Amount != nil {
		params["amount"] = strconv.FormatInt(r.Amount.Amount, 10)
		params["currency"] = strings.ToUpper(r.Amount.Currency)
	}
	_, err := p.call(ctx, params)
	return err
}

func (p *provider) CreateCheckoutSession(context.Context, payment.CheckoutRequest) (*payment.CheckoutSession, error) {
	return nil, errors.New("payment-payfort: PayFort uses a POST redirection form (FortAPI/paymentPage) — build it with the exported Signature() helper; CreateCharge handles token_name purchases")
}

func (p *provider) CreateCustomer(context.Context, payment.Customer) (string, error) {
	return "", errors.New("payment-payfort: no customer object; reuse a token_name per charge")
}

func (p *provider) CreateSubscription(context.Context, payment.SubscriptionRequest) (*payment.Subscription, error) {
	return nil, errors.New("payment-payfort: recurring uses RECURRING command with a token_name — not exposed by this driver yet")
}

// HandleWebhook verifies the PayFort response signature (SHA response phrase) on
// the posted JSON notification and normalizes it.
func (p *provider) HandleWebhook(_ context.Context, _ map[string]string, body []byte) (*payment.WebhookEvent, error) {
	var ev map[string]any
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, err
	}
	got, _ := ev["signature"].(string)
	params := map[string]string{}
	for k, v := range ev {
		params[k] = fmt.Sprint(v)
	}
	if got != "" && !strings.EqualFold(got, Signature(p.shaResp, params)) {
		return nil, errors.New("payment-payfort: invalid response signature")
	}
	id, _ := ev["fort_id"].(string)
	typ := "payment.update"
	if s, _ := ev["status"].(string); s == "14" {
		typ = "charge.succeeded"
	} else if s != "" {
		typ = "charge.failed"
	}
	return &payment.WebhookEvent{Type: typ, ID: id, Provider: "payfort", Raw: ev}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
