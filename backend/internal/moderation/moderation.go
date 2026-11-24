// Package moderation blocks restricted product categories and scrubs prohibited claims from generated copy.
package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

type rule struct {
	re    *regexp.Regexp
	label string
}

func r(p, label string) rule { return rule{regexp.MustCompile(`(?i)` + p), label} }

var blocked = []rule{
	r(`\b(firearms?|rifles?|pistols?|ammunition|ammo|silencers?|suppressors?|switchblades?)\b`, "weapons"),
	r(`\b(porn|xxx|escort|erotic|adult toys?|sex toys?|fetish)\b`, "adult content"),
	r(`\b(cocaine|heroin|meth(amphetamine)?|lsd|mdma|cannabis|marijuana|weed|thc|cbd|kratom)\b`, "drugs"),
	r(`\b(casino|sports ?betting|poker|slots|gambling)\b`, "gambling"),
	r(`\b(cigarettes?|vapes?|vaping|e-?liquids?|tobacco|nicotine)\b`, "tobacco and vaping"),
	r(`\b(counterfeit|replica|knock-?off|fake id|fake passport)\b`, "counterfeit goods"),
	r(`\b(payday loan|get rich quick|crypto signals|forex signals|ponzi)\b`, "deceptive financial offers"),
	r(`\b(miracle cure|cures? (cancer|diabetes|covid|aids)|lose \d+ ?(lbs|pounds|kg) in)\b`, "unsubstantiated health claims"),
}

var claims = regexp.MustCompile(`(?i)\b(cures?|guaranteed (results|income|weight loss)|clinically proven|doctor recommended|fda approved|no side effects)\b`)
var spaces = regexp.MustCompile(`\s{2,}`)

// CheckProduct returns a user-facing reason if the product category is not allowed.
func CheckProduct(p store.Product) string {
	text := p.Title + " " + p.Description
	for _, x := range blocked {
		if x.re.MatchString(text) {
			return "We can't create ads for this product category (" + x.label + ")."
		}
	}
	return ""
}

func Scrub(s string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(claims.ReplaceAllString(s, ""), " "))
}

// OpenAI runs the hosted moderation endpoint when an API key is configured; failures are non-blocking.
func OpenAI(ctx context.Context, key string, p store.Product) string {
	if key == "" {
		return ""
	}
	input := p.Title + "\n" + p.Description
	if len(input) > 4000 {
		input = input[:4000]
	}
	body, _ := json.Marshal(map[string]any{"model": "omni-moderation-latest", "input": input})
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/moderations", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		Results []struct {
			Flagged bool `json:"flagged"`
		} `json:"results"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Results) == 0 {
		return ""
	}
	if out.Results[0].Flagged {
		return "This product page was flagged by content moderation."
	}
	return ""
}
