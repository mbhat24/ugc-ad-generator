// Package copywriter produces hooks, scripts and captions, with a deterministic template fallback.
package copywriter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/moderation"
	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

var avatars = []string{
	"young woman, casual home setting, friendly and energetic",
	"young man, bright bedroom, relaxed and authentic",
	"woman in her 30s, kitchen, warm and trustworthy",
	"man in his 30s, living room, confident and conversational",
}

type Writer struct {
	Key   string
	Model string
	HTTP  *http.Client
}

type copyT struct {
	Hook     string   `json:"hook"`
	Script   string   `json:"script"`
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func templates(p store.Product) []copyT {
	d := trunc(p.Description, 120)
	price := func(format string) string {
		if p.Price == "" {
			return ""
		}
		return fmt.Sprintf(format, p.Price)
	}
	return []copyT{
		{fmt.Sprintf("Stop scrolling, I can't believe this %s is real", p.Title), fmt.Sprintf("Okay so I just tried the %s and honestly? I was skeptical. %s Two weeks in and I use it every day. If you've been on the fence, this is your sign.", p.Title, d), fmt.Sprintf("Not sponsored, just obsessed with the %s. Link in bio.", p.Title), []string{"#tiktokmademebuyit", "#musthave", "#review"}},
		{fmt.Sprintf("POV: you finally found the %s everyone keeps talking about", p.Title), fmt.Sprintf("Everyone kept telling me about this, so I finally got it. %s %s Go grab one before it sells out.", d, price("And it's only %s.")), fmt.Sprintf("Found it. %s, link in bio.", p.Title), []string{"#fyp", "#smallbusiness", "#haul"}},
		{fmt.Sprintf("3 reasons I'm never going back after the %s", p.Title), fmt.Sprintf("One: it actually works. Two: it looks great. Three: %s. %s Seriously, try it.", map[bool]string{true: "the price is unreal", false: "it's " + p.Price}[p.Price == ""], trunc(p.Description, 100)), fmt.Sprintf("3 reasons you need the %s", p.Title), []string{"#gamechanger", "#productreview", "#shopsmall"}},
	}
}

func (w *Writer) llm(ctx context.Context, p store.Product, n int) []copyT {
	if w.Key == "" {
		return nil
	}
	sys := `You write high-converting 15-second TikTok/Reels UGC ad scripts spoken by a creator to camera. Casual, specific, no hype cliches, no false claims, no medical claims. Return JSON: {"ads":[{"hook":string (max 12 words, first 2 seconds),"script":string (35-45 spoken words),"caption":string,"hashtags":string[3-5]}]}`
	user := fmt.Sprintf("Write %d distinct ads with different angles for:\nProduct: %s\nPrice: %s\nDescription: %s", n, p.Title, orNA(p.Price), trunc(p.Description, 1200))
	body, _ := json.Marshal(map[string]any{"model": w.Model, "response_format": map[string]string{"type": "json_object"}, "messages": []map[string]string{{"role": "system", "content": sys}, {"role": "user", "content": user}}})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+w.Key)
	req.Header.Set("Content-Type", "application/json")
	c := w.HTTP
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Choices) == 0 {
		return nil
	}
	var parsed struct {
		Ads []copyT `json:"ads"`
	}
	if json.Unmarshal([]byte(out.Choices[0].Message.Content), &parsed) != nil {
		return nil
	}
	return parsed.Ads
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

// Generate returns n ads, each with a distinct avatar, scrubbed of prohibited claims.
func (w *Writer) Generate(ctx context.Context, p store.Product, n int) []store.NewAd {
	got := w.llm(ctx, p, n)
	fb := templates(p)
	out := make([]store.NewAd, n)
	for i := range out {
		c := fb[i%len(fb)]
		if i < len(got) && got[i].Hook != "" && got[i].Script != "" {
			c = got[i]
		}
		if len(c.Hashtags) == 0 {
			c.Hashtags = []string{"#fyp"}
		}
		out[i] = store.NewAd{Hook: moderation.Scrub(c.Hook), Script: moderation.Scrub(c.Script), Caption: moderation.Scrub(c.Caption), Hashtags: c.Hashtags, Avatar: avatars[i%len(avatars)]}
	}
	return out
}
