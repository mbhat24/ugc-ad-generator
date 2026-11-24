package scrape

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "1.1.1.1": true, "2606:4700:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "172.16.0.1": false, "192.168.1.1": false, "169.254.169.254": false,
		"100.64.0.1": false, "0.0.0.0": false, "224.0.0.1": false, "::1": false, "fe80::1": false, "fc00::1": false,
		"::ffff:127.0.0.1": false, "::ffff:7f00:1": false, "::ffff:10.0.0.1": false, "64:ff9b::7f00:1": false, "2002:7f00:1::": false,
	} {
		if got := IsPublicIP(netip.MustParseAddr(ip)); got != want {
			t.Errorf("IsPublicIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestClientBlocksInternalTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<title>secret</title>")) }))
	defer srv.Close()
	c := NewClient()
	for _, u := range []string{srv.URL, "http://localhost:1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data", "ftp://example.com/x"} {
		if _, err := c.Product(context.Background(), u); err == nil {
			t.Errorf("%s: expected error", u)
		}
	}
	if _, err := c.Product(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Errorf("loopback server: got %v, want ErrBlocked", err)
	}
}

func TestClientBlocksRedirectToInternal(t *testing.T) {
	// the redirect target is private even though the first hop is allowed in test mode
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<title>x</title>")) }))
	defer internal.Close()
	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, internal.URL, http.StatusFound) }))
	defer hop.Close()
	if _, err := NewClient().Product(context.Background(), hop.URL); err == nil {
		t.Fatal("expected redirect to loopback to be blocked")
	}
}

func TestProductParsing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products/shoe", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/p/page", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Fallback</title><meta property="og:title" content="Merino &amp; Wool Runner"><meta content="Soft &lt;b&gt;and&lt;/b&gt; light" property="og:description"><meta property="og:image" content="/img/a.jpg"><meta property="product:price:amount" content="95.00"><meta property="product:price:currency" content="USD"></head></html>`))
	})
	mux.HandleFunc("/products/json.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"product":{"title":"JSON Shoe","body_html":"<p>Great <b>shoe</b></p>","images":[{"src":"https://cdn/x.jpg"}],"variants":[{"price":"120.00"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := NewClientForTest()

	p, err := c.Product(context.Background(), srv.URL+"/p/page")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Merino & Wool Runner" || p.Price != "$95.00" || p.Description != "Soft and light" || p.Image != srv.URL+"/img/a.jpg" {
		t.Errorf("unexpected product: %+v", p)
	}
	p, err = c.Product(context.Background(), srv.URL+"/products/json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "JSON Shoe" || p.Price != "$120.00" || p.Description != "Great shoe" || len(p.Images) != 1 {
		t.Errorf("unexpected shopify product: %+v", p)
	}
}
