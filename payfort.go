// Package payfort is a PayFort driver for togo payment. Blank-import it and set
// PAYMENT_DRIVER=payfort plus PAYFORT_ACCESS_CODE. The driver registers and is env-configured; the
// gateway API calls are scaffolded (see PayFort docs: https://paymentservices.amazon.com) — the togo payment
// interface is satisfied. Contributions to flesh out the calls are welcome.
package payfort

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/togo-framework/payment"
	"github.com/togo-framework/togo"
)

func init() {
	payment.RegisterDriver("payfort", func(k *togo.Kernel) (payment.PaymentProvider, error) {
		key := os.Getenv("PAYFORT_ACCESS_CODE")
		if key == "" {
			return nil, errors.New("payment-payfort: PAYFORT_ACCESS_CODE not set")
		}
		return &provider{key: key, hc: &http.Client{Timeout: 20 * time.Second}}, nil
	})
}

type provider struct {
	key string
	hc  *http.Client
}

var errTODO = errors.New("payment-payfort: this operation is scaffolded — wire the PayFort API (https://paymentservices.amazon.com)")

func (p *provider) CreateCharge(context.Context, payment.ChargeRequest) (*payment.Charge, error) {
	return nil, errTODO
}
func (p *provider) Refund(context.Context, payment.RefundRequest) error { return errTODO }
func (p *provider) CreateCheckoutSession(context.Context, payment.CheckoutRequest) (*payment.CheckoutSession, error) {
	return nil, errTODO
}
func (p *provider) CreateCustomer(context.Context, payment.Customer) (string, error) { return "", errTODO }
func (p *provider) CreateSubscription(context.Context, payment.SubscriptionRequest) (*payment.Subscription, error) {
	return nil, errTODO
}
func (p *provider) HandleWebhook(context.Context, map[string]string, []byte) (*payment.WebhookEvent, error) {
	return nil, errTODO
}
