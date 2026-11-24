package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.9:5555"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
	require.Equal(t, "203.0.113.7", ClientIP(r, 1), "the entry appended by our proxy, not the spoofable left-most one")
	require.Equal(t, "6.6.6.6", ClientIP(r, 2))
	require.Equal(t, "10.0.0.9", ClientIP(r, 0), "no trusted proxy: use the socket address")
	r.Header.Del("X-Forwarded-For")
	require.Equal(t, "10.0.0.9", ClientIP(r, 1))
}

func TestDecodeJSONLimit(t *testing.T) {
	var v struct{ A string }
	require.NoError(t, DecodeJSON(httptest.NewRequest("POST", "/", strings.NewReader(`{"A":"x"}`)), &v, 100))
	require.ErrorIs(t, DecodeJSON(httptest.NewRequest("POST", "/", strings.NewReader(`{"A":"`+strings.Repeat("x", 200)+`"}`)), &v, 100), ErrBodyTooLarge)
}
