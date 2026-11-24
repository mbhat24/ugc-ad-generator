package billing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/testutil"
)

func TestWebhookSignatureDedupeAndPlanSync(t *testing.T) {
	st := testutil.Store(t)
	cfg := config.Config{StripeSecretKey: "sk_test_x", StripeWebhookSecret: "whsec_test", StripePrices: map[string]string{"starter": "price_s", "growth": "price_g", "scale": "price_x"}}
	svc := New(cfg, st)
	ctx := context.Background()

	id, _ := testutil.User(t, st)
	cust := "cus_" + uuid.NewString()
	require.NoError(t, st.SetStripeCustomer(ctx, id, cust))

	event := func(typ, status string) []byte {
		b, _ := json.Marshal(map[string]any{
			"id": "evt_" + uuid.NewString(), "object": "event", "type": typ, "api_version": "2020-01-01",
			"data": map[string]any{"object": map[string]any{"id": "sub_1", "object": "subscription", "customer": cust, "status": status, "items": map[string]any{"data": []any{map[string]any{"price": map[string]any{"id": "price_g"}}}}}},
		})
		return b
	}
	sign := func(p []byte) string {
		return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: p, Secret: "whsec_test", Timestamp: time.Now()}).Header
	}
	plan := func() store.Plan {
		var p store.Plan
		require.NoError(t, st.Pool.QueryRow(ctx, "select plan from profiles where id = $1", id).Scan(&p))
		return p
	}

	require.ErrorIs(t, svc.HandleWebhook(ctx, event("customer.subscription.updated", "active"), "t=1,v1=bad"), ErrBadSignature)

	up := event("customer.subscription.updated", "active")
	require.NoError(t, svc.HandleWebhook(ctx, up, sign(up)))
	require.Equal(t, store.PlanGrowth, plan())
	require.NoError(t, svc.HandleWebhook(ctx, up, sign(up)), "replayed event is acknowledged")

	del := event("customer.subscription.deleted", "canceled")
	require.NoError(t, svc.HandleWebhook(ctx, del, sign(del)))
	require.Equal(t, store.PlanFree, plan())
}

func TestGrantSubscriptionCreditsIsIdempotent(t *testing.T) {
	st := testutil.Store(t)
	ctx := context.Background()
	id, _ := testutil.User(t, st)
	cust := "cus_" + uuid.NewString()
	require.NoError(t, st.SetStripeCustomer(ctx, id, cust))
	for range 3 {
		require.NoError(t, st.GrantSubscriptionCredits(ctx, cust, 40, "in_123_"+id))
	}
	bal, _ := st.Balance(ctx, id)
	require.Equal(t, 41, bal)
}
