package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/stripe/stripe-go/v81"
	billingportal "github.com/stripe/stripe-go/v81/billingportal/session"
	checkout "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/webhook"
)

// Card data never touches Pulse: Stripe hosts the payment page and the billing
// portal. That keeps us out of PCI scope entirely, which is the main reason to
// use Checkout rather than building a payment form.

type checkoutReq struct {
	PlanCode string `json:"plan_code"`
}

func (s *Server) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	var in checkoutReq
	if err := decode(r, &in); err != nil || in.PlanCode == "" {
		writeErr(w, 400, "plan_code required")
		return
	}

	var priceID *string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT stripe_price_id FROM plans WHERE code=$1`, in.PlanCode).Scan(&priceID); err != nil {
		writeErr(w, 404, "unknown plan")
		return
	}
	if priceID == nil || *priceID == "" {
		writeErr(w, 400, "that plan is not purchasable")
		return
	}

	var tenantName, ownerEmail string
	var customerID *string
	if err := s.DB.QueryRow(r.Context(), `
		SELECT t.name, t.stripe_customer_id,
		       (SELECT email FROM users WHERE tenant_id=t.id AND role='owner' LIMIT 1)
		FROM tenants t WHERE t.id=$1`, c.TenantID).
		Scan(&tenantName, &customerID, &ownerEmail); err != nil {
		writeErr(w, 500, "db")
		return
	}

	// One Stripe customer per tenant, created on first checkout and reused
	// after. Creating a new one each time would fragment billing history.
	if customerID == nil || *customerID == "" {
		cust, err := customer.New(&stripe.CustomerParams{
			Name:  stripe.String(tenantName),
			Email: stripe.String(ownerEmail),
			Metadata: map[string]string{
				"pulse_tenant_id": c.TenantID,
			},
		})
		if err != nil {
			slog.Error("create stripe customer", "err", err)
			writeErr(w, 502, "could not reach billing provider")
			return
		}
		if _, err := s.DB.Exec(r.Context(),
			`UPDATE tenants SET stripe_customer_id=$2 WHERE id=$1`, c.TenantID, cust.ID); err != nil {
			slog.Error("save customer id", "err", err)
		}
		customerID = &cust.ID
	}

	sess, err := checkout.New(&stripe.CheckoutSessionParams{
		Customer: customerID,
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: priceID, Quantity: stripe.Int64(1)},
		},
		// Metadata rides along to the webhook, so the handler knows which
		// tenant and plan a completed session belongs to without a lookup
		// table of its own.
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"pulse_tenant_id": c.TenantID,
				"pulse_plan":      in.PlanCode,
			},
		},
		Metadata: map[string]string{
			"pulse_tenant_id": c.TenantID,
			"pulse_plan":      in.PlanCode,
		},
		SuccessURL: stripe.String(s.Cfg.DashboardURL + "/?billing=success"),
		CancelURL:  stripe.String(s.Cfg.DashboardURL + "/?billing=cancelled"),
	})
	if err != nil {
		slog.Error("create checkout session", "err", err)
		writeErr(w, 502, "could not start checkout")
		return
	}

	writeJSON(w, 200, map[string]string{"url": sess.URL})
}

// CreatePortal opens Stripe's hosted billing portal, where a customer can
// change plan, update their card, see invoices, or cancel. Building all of
// that ourselves would be weeks of work and a compliance burden.
func (s *Server) CreatePortal(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var customerID *string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT stripe_customer_id FROM tenants WHERE id=$1`, c.TenantID).Scan(&customerID); err != nil {
		writeErr(w, 500, "db")
		return
	}
	if customerID == nil || *customerID == "" {
		writeErr(w, 400, "no billing account yet — choose a plan first")
		return
	}

	sess, err := billingportal.New(&stripe.BillingPortalSessionParams{
		Customer:  customerID,
		ReturnURL: stripe.String(s.Cfg.DashboardURL + "/"),
	})
	if err != nil {
		slog.Error("create portal session", "err", err)
		writeErr(w, 502, "could not open billing portal")
		return
	}
	writeJSON(w, 200, map[string]string{"url": sess.URL})
}

// StripeWebhook is the only endpoint Stripe calls. It is unauthenticated in the
// usual sense — there is no user session — but every request must carry a valid
// signature computed with the webhook secret, which is what proves it came from
// Stripe rather than from anyone who discovered the URL.
func (s *Server) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBody = 1 << 20
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		writeErr(w, 400, "could not read body")
		return
	}

	// The account's API version is ahead of this SDK's. We read only long-stable
	// fields (customer, subscription, price, status, metadata), so the mismatch
	// is acceptable — but it is a deliberate choice, not an oversight, and the
	// real fix is keeping the SDK current.
	event, err := webhook.ConstructEventWithOptions(payload,
		r.Header.Get("Stripe-Signature"), s.Cfg.StripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		slog.Warn("stripe webhook signature rejected", "err", err)
		writeErr(w, 400, "invalid signature")
		return
	}

	// Idempotency: Stripe retries on any non-2xx and may deliver the same event
	// more than once. Recording the event id first means a duplicate is a
	// cheap no-op rather than a second plan change.
	tag, err := s.DB.Exec(r.Context(),
		`INSERT INTO stripe_events (id, type, payload) VALUES ($1,$2,$3)
		 ON CONFLICT (id) DO NOTHING`, event.ID, string(event.Type), payload)
	if err != nil {
		// Returning 500 makes Stripe retry, which is what we want: better a
		// duplicate delivery than a silently dropped subscription change.
		slog.Error("record stripe event", "err", err)
		writeErr(w, 500, "db")
		return
	}
	if tag.RowsAffected() == 0 {
		slog.Info("stripe event already processed", "id", event.ID, "type", event.Type)
		w.WriteHeader(200)
		return
	}

	if err := s.handleStripeEvent(r, event); err != nil {
		slog.Error("handle stripe event", "id", event.ID, "type", event.Type, "err", err)
		// Remove the record so the retry is actually reprocessed rather than
		// short-circuited by the idempotency check above.
		_, _ = s.DB.Exec(r.Context(), `DELETE FROM stripe_events WHERE id=$1`, event.ID)
		writeErr(w, 500, "could not process event")
		return
	}

	slog.Info("stripe event processed", "id", event.ID, "type", event.Type)
	w.WriteHeader(200)
}

func (s *Server) handleStripeEvent(r *http.Request, event stripe.Event) error {
	switch event.Type {

	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return err
		}
		tenantID := sess.Metadata["pulse_tenant_id"]
		planCode := sess.Metadata["pulse_plan"]
		if tenantID == "" || planCode == "" {
			slog.Warn("checkout session without pulse metadata", "session", sess.ID)
			return nil
		}
		var subID string
		if sess.Subscription != nil {
			subID = sess.Subscription.ID
		}
		_, err := s.DB.Exec(r.Context(), `
			UPDATE tenants SET plan_code=$2, stripe_subscription_id=NULLIF($3,''),
			                   subscription_status='active'
			WHERE id=$1`, tenantID, planCode, subID)
		return err

	case "customer.subscription.updated", "customer.subscription.created":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.applySubscription(r, sub)

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		// Only act if the deleted subscription is the one we track. A customer
		// can have several subscriptions in Stripe (a stray Checkout, a
		// duplicate); cancelling one of those must not revoke a plan the
		// customer is still paying for.
		_, err := s.DB.Exec(r.Context(), `
			UPDATE tenants SET plan_code='free', subscription_status='canceled',
			                   stripe_subscription_id=NULL,
			                   cancel_at_period_end=false
			WHERE stripe_customer_id=$1 AND stripe_subscription_id=$2`,
			sub.Customer.ID, sub.ID)
		return err

	case "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return err
		}
		// Don't downgrade on a single failure: Stripe retries over several days
		// and most failures resolve. Record the state so the UI can warn.
		_, err := s.DB.Exec(r.Context(), `
			UPDATE tenants SET subscription_status='past_due'
			WHERE stripe_customer_id=$1`, inv.Customer.ID)
		return err

	default:
		// Unhandled types are acknowledged, not retried: Stripe sends many
		// event types we have no interest in.
		return nil
	}
}

// applySubscription maps a Stripe subscription onto the tenant's plan. The
// price ID is the link: whatever price the subscription carries determines
// which plan the tenant is on, so a change made in Stripe's portal flows back
// here without the app having to interpret the portal's UI.
func (s *Server) applySubscription(r *http.Request, sub stripe.Subscription) error {
	if len(sub.Items.Data) == 0 {
		return nil
	}
	priceID := sub.Items.Data[0].Price.ID

	var planCode string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT code FROM plans WHERE stripe_price_id=$1`, priceID).Scan(&planCode); err != nil {
		slog.Warn("subscription price has no matching plan", "price", priceID)
		return nil
	}

	// A subscription that is not active or trialing gets no plan benefits.
	status := string(sub.Status)
	effective := planCode
	if status != "active" && status != "trialing" {
		effective = "free"
	}

	_, err := s.DB.Exec(r.Context(), `
		UPDATE tenants SET plan_code=$2, subscription_status=$3,
		                   stripe_subscription_id=$4,
		                   cancel_at_period_end=$6,
		                   current_period_end=to_timestamp($5)
		WHERE stripe_customer_id=$1
		  -- Accept an event for the tracked subscription, or adopt one if we are
		  -- tracking none yet (first checkout). Ignore events about other
		  -- subscriptions belonging to the same customer.
		  AND (stripe_subscription_id IS NULL OR stripe_subscription_id=$4)`,
		sub.Customer.ID, effective, status, sub.ID, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd)
	return err
}
