package copywriter

import (
	"context"
	"strings"
	"testing"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

func TestGenerateFallbackIsDistinctAndClean(t *testing.T) {
	w := &Writer{}
	ads := w.Generate(context.Background(), store.Product{Title: "Runner", Description: "Light shoe that cures sore feet", Price: "$95"}, 3)
	if len(ads) != 3 {
		t.Fatalf("got %d ads", len(ads))
	}
	seenHook, seenAvatar := map[string]bool{}, map[string]bool{}
	for _, a := range ads {
		seenHook[a.Hook], seenAvatar[a.Avatar] = true, true
		if a.Hook == "" || a.Script == "" || len(a.Hashtags) == 0 || strings.Contains(strings.ToLower(a.Script), "cures") {
			t.Errorf("bad ad: %+v", a)
		}
	}
	if len(seenHook) != 3 || len(seenAvatar) != 3 {
		t.Errorf("ads are not distinct: hooks=%d avatars=%d", len(seenHook), len(seenAvatar))
	}
}
