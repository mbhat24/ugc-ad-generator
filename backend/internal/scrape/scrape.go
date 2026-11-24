// Package scrape fetches product details from public store pages with SSRF protection.
package scrape

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/store"
)

var ErrBlocked = errors.New("URL must be a public website")

var blockedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, c := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/3",
		"::/128", "::1/128", "64:ff9b::/96", "100::/64", "2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
	} {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}()

// IsPublicIP reports whether ip is a globally routable unicast address.
func IsPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// Client fetches pages. Every connection is validated at dial time, so DNS rebinding and redirects cannot reach internal addresses.
type Client struct{ http *http.Client }

func NewClient() *Client { return newClient(false) }

// NewClientForTest allows loopback targets so tests can use httptest servers. Never use it in production code.
func NewClientForTest() *Client { return newClient(true) }

func newClient(allowPrivate bool) *Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, ErrBlocked
			}
			var ok []netip.Addr
			for _, ip := range ips {
				if !allowPrivate && !IsPublicIP(ip) {
					return nil, ErrBlocked
				}
				ok = append(ok, ip)
			}
			if len(ok) == 0 {
				return nil, ErrBlocked
			}
			var last error
			for _, ip := range ok {
				c, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return c, nil
				}
				last = err
			}
			return nil, last
		},
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Client{http: &http.Client{
		Transport: tr,
		Timeout:   12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many redirects")
			}
			return checkScheme(req.URL)
		},
	}}
}

func checkScheme(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL must be http(s)")
	}
	if u.Hostname() == "" {
		return ErrBlocked
	}
	return nil
}

func (c *Client) get(ctx context.Context, u *url.URL, maxBytes int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AdReelBot/1.0)")
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.5")
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) || strings.Contains(err.Error(), ErrBlocked.Error()) {
			return nil, 0, ErrBlocked
		}
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	return b, resp.StatusCode, err
}

var (
	tagRe   = regexp.MustCompile(`<[^>]+>`)
	spaceRe = regexp.MustCompile(`\s+`)
	metaRe  = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	attrRe  = regexp.MustCompile(`(?is)([a-zA-Z:-]+)\s*=\s*("([^"]*)"|'([^']*)')`)
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func clean(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(tagRe.ReplaceAllString(html.UnescapeString(s), " "), " "))
}

func metaTags(doc string) map[string]string {
	out := map[string]string{}
	for _, m := range metaRe.FindAllString(doc, -1) {
		attrs := map[string]string{}
		for _, a := range attrRe.FindAllStringSubmatch(m, -1) {
			v := a[3]
			if v == "" {
				v = a[4]
			}
			attrs[strings.ToLower(a[1])] = v
		}
		key := attrs["property"]
		if key == "" {
			key = attrs["name"]
		}
		if key != "" && attrs["content"] != "" {
			if _, ok := out[strings.ToLower(key)]; !ok {
				out[strings.ToLower(key)] = clean(attrs["content"])
			}
		}
	}
	return out
}

func price(amount, cur string) string {
	if amount == "" {
		return ""
	}
	if cur == "" || cur == "USD" {
		return "$" + amount
	}
	return cur + " " + amount
}

// Product extracts product details from a store page URL.
func (c *Client) Product(ctx context.Context, raw string) (store.Product, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || checkScheme(u) != nil {
		return store.Product{}, errors.New("Enter a valid http(s) product URL")
	}
	if strings.Contains(u.Path, "/products/") {
		ju := *u
		ju.Path = strings.TrimRight(ju.Path, "/") + ".json"
		ju.RawQuery = ""
		if b, code, err := c.get(ctx, &ju, 1<<20); err == nil && code == 200 {
			var v struct {
				Product struct {
					Title    string `json:"title"`
					BodyHTML string `json:"body_html"`
					Images   []struct {
						Src string `json:"src"`
					} `json:"images"`
					Variants []struct {
						Price string `json:"price"`
					} `json:"variants"`
				} `json:"product"`
			}
			if json.Unmarshal(b, &v) == nil && v.Product.Title != "" {
				p := store.Product{Title: v.Product.Title, Description: clean(v.Product.BodyHTML), Images: []string{}}
				for i, im := range v.Product.Images {
					if i < 5 {
						p.Images = append(p.Images, im.Src)
					}
				}
				if len(p.Images) > 0 {
					p.Image = p.Images[0]
				}
				if len(v.Product.Variants) > 0 {
					p.Price = price(v.Product.Variants[0].Price, "")
				}
				return p, nil
			}
		} else if errors.Is(err, ErrBlocked) {
			return store.Product{}, err
		}
	}
	b, code, err := c.get(ctx, u, 1_500_000)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return store.Product{}, err
		}
		return store.Product{}, errors.New("Could not load that page")
	}
	if code != 200 {
		return store.Product{}, fmt.Errorf("Could not load that page (HTTP %d)", code)
	}
	doc := string(b)
	m := metaTags(doc)
	title := m["og:title"]
	if title == "" {
		if t := titleRe.FindStringSubmatch(doc); t != nil {
			title = clean(t[1])
		}
	}
	if title == "" {
		return store.Product{}, errors.New("Could not find a product on that page")
	}
	desc := m["og:description"]
	if desc == "" {
		desc = m["description"]
	}
	amount := m["product:price:amount"]
	if amount == "" {
		amount = m["og:price:amount"]
	}
	cur := m["product:price:currency"]
	if cur == "" {
		cur = m["og:price:currency"]
	}
	p := store.Product{Title: title, Description: desc, Price: price(amount, cur), Images: []string{}}
	if im := m["og:image"]; im != "" {
		if iu, err := u.Parse(im); err == nil {
			p.Image = iu.String()
			p.Images = []string{p.Image}
		}
	}
	return p, nil
}
