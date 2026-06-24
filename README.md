# payment-payfort

[PayFort](https://paymentservices.amazon.com) driver for togo **payment**.

```bash
togo install togo-framework/payment
togo install togo-framework/payment-payfort
```
```env
PAYMENT_DRIVER=payfort
PAYFORT_ACCESS_CODE=...
```

Registers on the togo `payment.PaymentProvider` interface and is selected via
`PAYMENT_DRIVER=payfort`. Gateway API calls are scaffolded — see the PayFort docs.

MIT
