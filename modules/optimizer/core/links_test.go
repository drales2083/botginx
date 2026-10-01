package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	key := []byte("test-key-0123456789")
	url := "https://example.com/a?b=c"
	tok, err := SignToken(key, url, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyToken(key, tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != url {
		t.Fatalf("got %q want %q", got, url)
	}
}

func TestTokenTampered(t *testing.T) {
	key := []byte("test-key-0123456789")
	tok, _ := SignToken(key, "https://example.com", time.Minute)
	dot := strings.Index(tok, ".")
	c := byte('A')
	if tok[dot+1] == 'A' {
		c = 'B'
	}
	bad := tok[:dot+1] + string(c) + tok[dot+2:]
	if bad == tok {
		t.Fatal("mutation must change the token")
	}
	if _, err := VerifyToken(key, bad); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestTokenExpired(t *testing.T) {
	key := []byte("test-key-0123456789")
	tok, _ := SignToken(key, "https://example.com", -time.Second)
	if _, err := VerifyToken(key, tok); err != ErrTokenExpired {
		t.Fatalf("got %v want ErrTokenExpired", err)
	}
}

func TestSignTokenRejectsNonHTTP(t *testing.T) {
	key := []byte("test-key-0123456789")
	if _, err := SignToken(key, "javascript:alert(1)", time.Minute); err == nil {
		t.Fatal("expected error for non-http URL")
	}
}

func TestNeutralizeURL(t *testing.T) {
	if got := NeutralizeURL("http://x.com"); !strings.HasPrefix(got, "hxxp://") {
		t.Fatalf("http not defanged: %q", got)
	}
	if got := NeutralizeURL("https://x.com"); !strings.HasPrefix(got, "hxxps://") {
		t.Fatalf("https not defanged: %q", got)
	}
	if got := NeutralizeURL("javascript:alert(1)"); got != "#" {
		t.Fatalf("dangerous scheme not blocked: %q", got)
	}
}

func TestSignTokenRejectsEmptyKey(t *testing.T) {
	for _, k := range [][]byte{nil, {}} {
		if _, err := SignToken(k, "https://example.com", time.Minute); err != ErrTokenInvalid {
			t.Fatalf("got %v want ErrTokenInvalid", err)
		}
	}
}

func TestSignTokenTrimsURL(t *testing.T) {
	key := []byte("test-key-0123456789")
	tok, err := SignToken(key, "  https://example.com/x \n", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyToken(key, tok)
	if err != nil || got != "https://example.com/x" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestVerifyTokenEmptyKey(t *testing.T) {
	tok, err := SignToken([]byte("some-key-123"), "https://a.com/x", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyToken(nil, tok); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("empty key: err=%v", err)
	}
	if _, err := VerifyToken([]byte{}, tok); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("empty key slice: err=%v", err)
	}
}
