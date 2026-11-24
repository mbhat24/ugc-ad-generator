package moderation

import (
	"testing"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

func TestCheckProduct(t *testing.T) {
	cases := map[string]bool{
		"Tactical rifle scope": true, "Premium CBD gummies": true, "Miracle cure serum": true, "Online casino bonus": true,
		"Merino wool running shoes": false, "Stainless steel water bottle": false, "Weed barrier fabric": true,
	}
	for title, blocked := range cases {
		if got := CheckProduct(store.Product{Title: title}) != ""; got != blocked {
			t.Errorf("%q blocked=%v want %v", title, got, blocked)
		}
	}
}

func TestScrub(t *testing.T) {
	got := Scrub("This cures acne. Guaranteed results, FDA approved and clinically proven!")
	for _, bad := range []string{"cures", "Guaranteed results", "FDA approved", "clinically proven"} {
		if contains(got, bad) {
			t.Errorf("still contains %q: %q", bad, got)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && (len(s) >= len(sub)) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
