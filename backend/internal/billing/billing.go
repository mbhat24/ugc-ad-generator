// Package billing integrates Stripe subscriptions with the credit ledger.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

var ErrDisabled = errors.New("billing is not configured")

type Service struct {
	cfg config.Config
	st  *store.Store
	sc  *client.API
}

func New(cfg config.Config, st *store.Store) *Service {
	s := &Service{cfg: cfg, st: st}
	if cfg.BillingEnabled() {
		s.sc = &client.API{}
		s.sc.Init(cfg.StripeSecretKey, nil)
	}
	return s
}

func (s *Service) Enabled() bool { return s.sc != nil }

func (s *Service) planForPrice(priceID string) store.Plan {
	for _, p := range []store.Plan{store.PlanStarter, store.PlanGrowth, store.PlanScale} {
		if s.cfg.StripePrices[string(p)] == priceID && priceID != "" {
			return p
		}
	}
	return store.PlanFree
}

func (s *Service) customerFor(ctx context.Context, userID, email string) (string, error) {
	c, err := s.st.StripeCustomer(ctx, userID)
	if err != nil || c != "" {
		return c, err
	}
	cu, err := s.sc.Customers.New(&stripe.CustomerParams{Email: stripe.String(email), Params: stripe.Params{Context: ctx, Metadata: map[string]string{"userId": userID}}})
	if err != nil {
		return "", err
	}
	return cu.ID, s.st.SetStripeCustomer(ctx, userID, cu.ID)
}

func (s *Service) Checkout(ctx context.Context, userID, email string, plan store.Plan) (string, error) {
	if !s.Enabled() {
		return "", ErrDisabled
	}
	price := s.cfg.StripePrices[string(plan)]
	if price == "" {
		return "", fmt.Errorf("unknown plan %q", plan)
	}
	cust, err := s.customerFor(ctx, userID, email)
	if err != nil {
		return "", err
	}
	sess, err := s.sc.CheckoutSessions.New(&stripe.CheckoutSessionParams{
		Params:            stripe.Params{Context: ctx},
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:          stripe.String(cust),
		ClientReferenceID: stripe.String(userID),
		LineItems:         []*stripe.CheckoutSessionLineItemParams{{Price: stripe.String(price), Quantity: stripe.Int64(1)}},
		SuccessURL:        stripe.String(s.cfg.AppURL + "/dashboard?upgraded=1"),
		CancelURL:         stripe.String(s.cfg.AppURL + "/dashboard"),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (s *Service) Portal(ctx context.Context, userID string) (string, error) {
	if !s.Enabled() {
		return "", ErrDisabled
	}
	cust, err := s.st.StripeCustomer(ctx, userID)
	if err != nil {
		return "", err
	}
	if cust == "" {
		return "", errors.New("no billing account yet")
	}
	sess, err := s.sc.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{Params: stripe.Params{Context: ctx}, Customer: stripe.String(cust), ReturnURL: stripe.String(s.cfg.AppURL + "/dashboard")})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// CancelAll cancels every live subscription of the user's Stripe customer.
func (s *Service) CancelAll(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return nil
	}
	cust, err := s.st.StripeCustomer(ctx, userID)
	if err != nil || cust == "" {
		return err
	}
	it := s.sc.Subscriptions.List(&stripe.SubscriptionListParams{Customer: stripe.String(cust), Status: stripe.String("all"), ListParams: stripe.ListParams{Context: ctx, Limit: stripe.Int64(20)}})
	for it.Next() {
		sub := it.Subscription()
		if sub.Status == stripe.SubscriptionStatusCanceled || sub.Status == stripe.SubscriptionStatusIncompleteExpired {
			continue
		}
		if _, err := s.sc.Subscriptions.Cancel(sub.ID, &stripe.SubscriptionCancelParams{Params: stripe.Params{Context: ctx}}); err != nil {
			return err
		}
	}
	return it.Err()
}

var ErrBadSignature = errors.New("bad signature")

// HandleWebhook verifies and processes a Stripe event exactly once per event id.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	ev, err := webhook.ConstructEventWithOptions(payload, sigHeader, s.cfg.StripeWebhookSecret, webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return ErrBadSignature
	}
	fresh, err := s.st.RecordStripeEvent(ctx, ev.ID, string(ev.Type))
	if err != nil {
		return err
	}
	if !fresh {
		return nil
	}
	if err := s.handle(ctx, ev); err != nil {
		_ = s.st.ForgetStripeEvent(context.WithoutCancel(ctx), ev.ID)
		return err
	}
	slog.Info("stripe event handled", "type", ev.Type, "event_id", ev.ID)
	return nil
}

func (s *Service) applySubscription(ctx context.Context, sub *stripe.Subscription) (store.Plan, error) {
	plan := store.PlanFree
	switch sub.Status {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing, stripe.SubscriptionStatusPastDue:
		if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
			plan = s.planForPrice(sub.Items.Data[0].Price.ID)
		}
	}
	if sub.Customer == nil {
		return plan, errors.New("subscription without customer")
	}
	return plan, s.st.SetPlanByCustomer(ctx, sub.Customer.ID, plan)
}

func (s *Service) handle(ctx context.Context, ev stripe.Event) error {
	switch ev.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			return err
		}
		_, err := s.applySubscription(ctx, &sub)
		return err
	case "invoice.paid":
		var inv stripe.Invoice
		if err := json.Unmarshal(ev.Data.Raw, &inv); err != nil {
			return err
		}
		if inv.Parent == nil || inv.Parent.SubscriptionDetails == nil || inv.Parent.SubscriptionDetails.Subscription == nil || inv.Customer == nil {
			return nil
		}
		sub, err := s.sc.Subscriptions.Get(inv.Parent.SubscriptionDetails.Subscription.ID, &stripe.SubscriptionParams{Params: stripe.Params{Context: ctx}})
		if err != nil {
			return err
		}
		plan, err := s.applySubscription(ctx, sub)
		if err != nil || plan == store.PlanFree {
			return err
		}
		info, _ := store.PlanByID(plan)
		return s.st.GrantSubscriptionCredits(ctx, inv.Customer.ID, info.Videos, inv.ID)
	case "invoice.payment_failed":
		slog.Warn("invoice payment failed", "event_id", ev.ID)
	}
	return nil
}
