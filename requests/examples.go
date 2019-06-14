package requests

import (
	"encoding/json"
	"fmt"

	"github.com/stripe/stripe-cli/profile"
)

func parseResponse(response []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal(response, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// WebhookEndpointList contains the list of webhook endpoints for the account
type WebhookEndpointList struct {
	Data []WebhookEndpoint `json:"data"`
}

// WebhookEndpoint contains the data for each webhook endpoint
type WebhookEndpoint struct {
	URL           string   `json:"url"`
	EnabledEvents []string `json:"enabled_events"`
}

// Examples stores possible webhook test events to trigger for the CLI
type Examples struct {
	Profile    profile.Profile
	APIUrl string
	APIVersion string
	SecretKey  string
}

func (ex *Examples) buildRequest(method string, data []string) *Base {
	return &Base{
		Profile:        ex.Profile,
		Method:         method,
		SuppressOutput: true,
		Version:        ex.APIVersion,
		Data:           data,
	}
}

func (ex *Examples) performStripeRequest(req *Base, endpoint string) (map[string]interface{}, error) {
	resp, err := req.MakeRequest(endpoint, ex.APIUrl, ex.SecretKey)
	if err != nil {
		return nil, err
	}
	return parseResponse(resp)
}

func (ex *Examples) tokenCreated(card string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", []string{
		fmt.Sprintf("card[number]=%s", card),
		"card[exp_month]=12",
		"card[exp_year]=2020",
		"card[cvc]=123",
	})
	return ex.performStripeRequest(req, "/v1/tokens")
}

func (ex *Examples) chargeCreated(card string, data[]string) (map[string]interface{}, error) {
	paymentToken, err := ex.tokenCreated(card)
	if err != nil {
		return nil, err
	}
	paymentSource := fmt.Sprintf("source=%s", paymentToken["id"])

	req := ex.buildRequest("POST", append(data, paymentSource))
	return ex.performStripeRequest(req, "/v1/charges")
}

// ChargeCaptured first creates a charge that is not captured, then
// sends another request to specifically capture it to trigger the
// captured event
func (ex *Examples) ChargeCaptured() error {
	charge, err := ex.chargeCreated("4242424242424242", []string{
		"amount=2000",
		"currency=usd",
		"capture=false",
	})
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{})
	reqURL := fmt.Sprintf("/v1/charges/%s/capture", charge["id"])

	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// ChargeFailed fails to create a charge
func (ex *Examples) ChargeFailed() error {
	_, err := ex.chargeCreated("4000000000000002", []string{
		"amount=2000",
		"currency=usd",
	})
	return err
}

// ChargeSucceeded successfully creates a charge
func (ex *Examples) ChargeSucceeded() error {
	_, err := ex.chargeCreated("4242424242424242", []string{
		"amount=2000",
		"currency=usd",
	})
	return err
}

func (ex *Examples) customerCreated(data []string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", data)
	return ex.performStripeRequest(req, "/v1/customers")
}

// CustomerCreated creates a new customer
func (ex *Examples) CustomerCreated() error {
	_, err := ex.customerCreated([]string{})
	return err
}

// CustomerUpdated creates a new customer and adds metadata to
// trigger an update event
func (ex *Examples) CustomerUpdated() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}
	req := ex.buildRequest("POST", []string{
		"metadata[foo]=bar",
	})
	reqURL := fmt.Sprintf("/v1/customers/%s", customer["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// CustomerSourceCreated creates a customer and a token then attaches
// the card to the customer
func (ex *Examples) CustomerSourceCreated() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	token, err := ex.tokenCreated("4242424242424242")
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{
		fmt.Sprintf("source=%s", token["id"]),
	})

	reqURL := fmt.Sprintf("/v1/customers/%s/sources", customer["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// CustomerSourceUpdated creates a customer, adds a card,
// adds metadata to the card to trigger an update
func (ex *Examples) CustomerSourceUpdated() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	token, err := ex.tokenCreated("4242424242424242")
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{
		fmt.Sprintf("source=%s", token["id"]),
	})

	reqURL := fmt.Sprintf("/v1/customers/%s/sources", customer["id"])
	card, err := ex.performStripeRequest(req, reqURL)
	if err != nil {
		return err
	}

	req = ex.buildRequest("POST", []string{
		"metadata[foo]=bar",
	})
	reqURL = fmt.Sprintf("/v1/customers/%s/sources/%s", customer["id"], card["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// CustomerSubscriptionUpdated creates a customer with a card, creates a plan,
// adds the customer to the plan, then updates the new subscription
func (ex *Examples) CustomerSubscriptionUpdated() error {
	token, err := ex.tokenCreated("4242424242424242")
	if err != nil {
		return err
	}

	customer, err := ex.customerCreated([]string{
		fmt.Sprintf("source=%s", token["id"]),
	})
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{
		"currency=usd",
		"interval=month",
		"amount=2000",
		"product[name]=myproduct",
	})
	plan, err := ex.performStripeRequest(req, "/v1/plans")
	if err != nil {
		return err
	}

	req = ex.buildRequest("POST", []string{
		fmt.Sprintf("items[0][plan]=%s", plan["id"]),
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	subscription, err := ex.performStripeRequest(req, "/v1/subscriptions")
	if err != nil {
		return err
	}

	req = ex.buildRequest("POST", []string{
		"metadata[foo]=bar",
	})
	reqURL := fmt.Sprintf("/v1/subscriptions/%s", subscription["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

func (ex *Examples) createInvoiceItem(data []string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", data)
	return ex.performStripeRequest(req, "/v1/invoiceitems")
}

func (ex *Examples) invoiceCreated(data []string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", data)
	return ex.performStripeRequest(req, "/v1/invoices")
}

// InvoiceCreated first creates a customer, adds an invoice item,
// then creates an invoice.
func (ex *Examples) InvoiceCreated() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	_, err = ex.createInvoiceItem([]string{
		"currency=usd",
		fmt.Sprintf("customer=%s", customer["id"]),
		"amount=2000",
	})
	if err != nil {
		return err
	}

	_, err = ex.invoiceCreated([]string{
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	if err != nil {
		return err
	}

	return err
}

// InvoiceFinalized first creates a customer, adds an invoice item,
// creates an invoice, and then finalizes the invoice.
func (ex *Examples) InvoiceFinalized() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	_, err = ex.createInvoiceItem([]string{
		"currency=usd",
		fmt.Sprintf("customer=%s", customer["id"]),
		"amount=2000",
	})
	if err != nil {
		return err
	}

	invoice, err := ex.invoiceCreated([]string{
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{})
	reqURL := fmt.Sprintf("/v1/invoices/%s/finalize", invoice["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// InvoicePaymentSucceeded first creates a customer, adds an invoice item,
// creates the invoice, and then pays the invoice
func (ex *Examples) InvoicePaymentSucceeded() error {
	token, err := ex.tokenCreated("4242424242424242")
	if err != nil {
		return err
	}

	customer, err := ex.customerCreated([]string{
		fmt.Sprintf("source=%s", token["id"]),
	})
	if err != nil {
		return err
	}

	_, err = ex.createInvoiceItem([]string{
		"currency=usd",
		fmt.Sprintf("customer=%s", customer["id"]),
		"amount=2000",
	})
	if err != nil {
		return err
	}

	invoice, err := ex.invoiceCreated([]string{
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{})
	reqURL := fmt.Sprintf("/v1/invoices/%s/pay", invoice["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// InvoiceUpdated first creates a customer, adds an invoice item,
// creates the invoice, then adds metadata to the invoice to trigger an update
func (ex *Examples) InvoiceUpdated() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	_, err = ex.createInvoiceItem([]string{
		"currency=usd",
		fmt.Sprintf("customer=%s", customer["id"]),
		"amount=2000",
	})
	if err != nil {
		return err
	}

	invoice, err := ex.invoiceCreated([]string{
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{
		"metadata[foo]=bar",
	})

	reqURL := fmt.Sprintf("/v1/invoices/%s", invoice["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

func (ex *Examples) paymentIntentCreated(data []string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", data)
	return ex.performStripeRequest(req, "/v1/payment_intents")
}

// PaymentIntentCreated creates a payment intent. Requires the data to be assigned
// to the payment intent
func (ex *Examples) PaymentIntentCreated() error {
	_, err := ex.paymentIntentCreated([]string{
		"amount=2000",
		"currency=usd",
		"payment_method_types[]=card",
	})
	return err
}

// PaymentIntentSucceeded creates a successful payment intent
func (ex *Examples) PaymentIntentSucceeded() error {
	paymentMethod, err := ex.paymentMethodCreated("4242424242424242")
	if err != nil {
		return err
	}
	paymentMethodID := fmt.Sprintf("payment_method=%s", paymentMethod["id"])

	_, err = ex.paymentIntentCreated([]string{
		"amount=2000",
		"currency=usd",
		"payment_method_types[]=card",
		"confirm=true",
		paymentMethodID,
	})

	return err
}

// PaymentIntentFailed creates a failed payment intent
func (ex *Examples) PaymentIntentFailed() error {
	paymentMethod, err := ex.paymentMethodCreated("4000000000000002")
	if err != nil {
		return err
	}

	_, err = ex.paymentIntentCreated([]string{
		"amount=2000",
		"currency=usd",
		"payment_method_types[]=card",
		"confirm=true",
		fmt.Sprintf("payment_method=%s", paymentMethod["id"]),
	})

	return err
}

func (ex *Examples) paymentMethodCreated(card string) (map[string]interface{}, error) {
	req := ex.buildRequest("POST", []string{
		"type=card",
		fmt.Sprintf("card[number]=%s", card),
		"card[exp_month]=12",
		"card[exp_year]=2020",
		"card[cvc]=123",
	})
	return ex.performStripeRequest(req, "/v1/payment_methods")
}

// PaymentMethodAttached creates a customer and payment method,
// then attaches the customer to the payment method
func (ex *Examples) PaymentMethodAttached() error {
	customer, err := ex.customerCreated([]string{})
	if err != nil {
		return err
	}

	paymentMethod, err := ex.paymentMethodCreated("4242424242424242")
	if err != nil {
		return err
	}

	req := ex.buildRequest("POST", []string{
		fmt.Sprintf("customer=%s", customer["id"]),
	})
	reqURL := fmt.Sprintf("/v1/payment_methods/%s/attach", paymentMethod["id"])
	_, err = ex.performStripeRequest(req, reqURL)
	return err
}

// WebhookEndpointsList returns all the webhook endpoints on a users' account
func (ex *Examples) WebhookEndpointsList() WebhookEndpointList {
	base := &Base{
		Profile:        ex.Profile,
		Method:         "GET",
		SuppressOutput: true,
		Version:        ex.APIVersion,
		Data:           []string{"limit=30"},
	}
	resp, _ := base.MakeRequest("/webhook_endpoints", "https://api.stripe.com/v1", ex.SecretKey)
	data := WebhookEndpointList{}
	json.Unmarshal([]byte(resp), &data)

	return data
}
