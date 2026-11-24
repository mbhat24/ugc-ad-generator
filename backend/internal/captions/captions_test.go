package captions

import (
	"strings"
	"testing"
)

func TestSRT(t *testing.T) {
	got := SRT("one two three four five six seven eight", 8, 4)
	want := "1\n00:00:00,000 --> 00:00:04,000\none two three four\n\n2\n00:00:04,000 --> 00:00:08,000\nfive six seven eight\n\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if SRT("", 10, 4) != "" || !strings.HasPrefix(SRT("a", 1, 4), "1\n") {
		t.Error("edge cases failed")
	}
}
