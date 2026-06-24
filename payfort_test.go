package payfort

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togo-framework/payment"
)

func TestSignature(t *testing.T) {
	// PayFort docs reference example.
	params := map[string]string{
		"command":             "PURCHASE",
		"access_code":         "zx0IPmPy5jp1vAz8Kpg7",
		"merchant_identifier": "CycHZxVj",
		"merchant_reference":  "XYZ9239-yu898",
		"amount":              "10000",
		"currency":            "AED",
		"language":            "en",
		"customer_email":      "customer1@domain.com",
	}
	got := Signature("TESTSHAIN", params)
	if len(got) != 64 {
		t.Fatalf("signature length: got %d want 64", len(got))
	}
	// Deterministic: same inputs → same signature.
	if Signature("TESTSHAIN", params) != got {
		t.Fatal("signature not deterministic")
	}
	// The signature ignores any pre-set signature field.
	params["signature"] = "ignored"
	if Signature("TESTSHAIN", params) != got {
		t.Fatal("signature should ignore existing signature field")
	}
}

func TestHandleWebhookSignature(t *testing.T) {
	p := &provider{shaResp: "TESTSHAOUT"}
	fields := map[string]string{"fort_id": "1500000001", "status": "14", "response_code": "14000"}
	sig := Signature(p.shaResp, fields)
	payload := map[string]any{"fort_id": "1500000001", "status": "14", "response_code": "14000", "signature": sig}
	body, _ := json.Marshal(payload)

	ev, err := p.HandleWebhook(context.Background(), nil, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type != "charge.succeeded" || ev.ID != "1500000001" || ev.Provider != "payfort" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	payload["signature"] = "bad"
	bad, _ := json.Marshal(payload)
	if _, err := p.HandleWebhook(context.Background(), nil, bad); err == nil {
		t.Fatal("expected invalid-signature error")
	}
}

var _ payment.PaymentProvider = (*provider)(nil)
