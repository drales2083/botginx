package signedurl

import (
	"net/url"
	"strings"
	"testing"
)

func TestSignURL(t *testing.T) {
	tests := []struct {
		name     string
		destURL  string
		linkID   string
		expiry   int
		wantErr  bool
	}{
		{
			name:    "simple URL",
			destURL: "https://example.com/page",
			linkID:  "abc123",
			expiry:  300,
		},
		{
			name:    "URL with existing params",
			destURL: "https://example.com/page?ref=test&utm=campaign",
			linkID:  "def456",
			expiry:  300,
		},
		{
			name:    "URL with special characters",
			destURL: "https://example.com/page?name=John%20Doe",
			linkID:  "ghi789",
			expiry:  300,
		},
		{
			name:    "default expiry",
			destURL: "https://example.com/page",
			linkID:  "jkl012",
			expiry:  0, // Should default to 300
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signedURL, err := SignURL(tt.destURL, tt.linkID, tt.expiry)
			if (err != nil) != tt.wantErr {
				t.Errorf("SignURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Parse signed URL
			u, err := url.Parse(signedURL)
			if err != nil {
				t.Errorf("failed to parse signed URL: %v", err)
				return
			}

			q := u.Query()

			// Check _bl (link ID) is present
			if q.Get("_bl") != tt.linkID {
				t.Errorf("_bl = %q, want %q", q.Get("_bl"), tt.linkID)
			}

			// Check _bt (timestamp) is present and recent
			bt := q.Get("_bt")
			if bt == "" {
				t.Error("_bt is missing")
			}

			// Check _bs (signature) is present
			bs := q.Get("_bs")
			if bs == "" {
				t.Error("_bs is missing")
			}
			// Ed25519 signatures are 64 bytes, base64url encoded = ~86 chars
			if len(bs) < 80 {
				t.Errorf("_bs seems too short: %d chars", len(bs))
			}
		})
	}
}

func TestVerifyURL(t *testing.T) {
	// Sign a URL
	destURL := "https://example.com/page?ref=test"
	linkID := "test123"
	expiry := 300

	signedURL, err := SignURL(destURL, linkID, expiry)
	if err != nil {
		t.Fatalf("SignURL failed: %v", err)
	}

	// Valid signature should verify
	if !VerifyURL(signedURL, expiry) {
		t.Error("VerifyURL returned false for valid signature")
	}

	// Tampered URL should fail
	tamperedURL := strings.Replace(signedURL, "ref=test", "ref=hacked", 1)
	if VerifyURL(tamperedURL, expiry) {
		t.Error("VerifyURL returned true for tampered URL")
	}

	// Tampered signature should fail
	badSigURL := strings.Replace(signedURL, "_bs=", "_bs=XXXXX", 1)
	if VerifyURL(badSigURL, expiry) {
		t.Error("VerifyURL returned true for bad signature")
	}

	// Missing signature should fail
	u, _ := url.Parse(signedURL)
	q := u.Query()
	q.Del("_bs")
	u.RawQuery = q.Encode()
	if VerifyURL(u.String(), expiry) {
		t.Error("VerifyURL returned true for missing signature")
	}
}

func TestVerifyURL_Expiry(t *testing.T) {
	destURL := "https://example.com/page"
	linkID := "expiry123"

	signedURL, err := SignURL(destURL, linkID, 300)
	if err != nil {
		t.Fatalf("SignURL failed: %v", err)
	}

	// Should be valid with 300 second window
	if !VerifyURL(signedURL, 300) {
		t.Error("VerifyURL should be valid within expiry window")
	}

	// Simulate checking with very small expiry (effectively expired)
	// We can't easily test true expiry without manipulating time
	// but we can test with 0 second tolerance which should still pass
	// since we just created it
	if !VerifyURL(signedURL, 1) {
		t.Error("VerifyURL should be valid for freshly signed URL")
	}
}

func TestSortedQueryString(t *testing.T) {
	tests := []struct {
		name   string
		input  url.Values
		want   string
	}{
		{
			name:   "empty",
			input:  url.Values{},
			want:   "",
		},
		{
			name:   "single param",
			input:  url.Values{"a": {"1"}},
			want:   "a=1",
		},
		{
			name:   "multiple params sorted",
			input:  url.Values{"z": {"3"}, "a": {"1"}, "m": {"2"}},
			want:   "a=1&m=2&z=3",
		},
		{
			name:   "special characters",
			input:  url.Values{"name": {"John Doe"}, "email": {"test@example.com"}},
			want:   "email=test%40example.com&name=John+Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedQueryString(tt.input)
			if got != tt.want {
				t.Errorf("sortedQueryString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPublicKey(t *testing.T) {
	pubKey := GetPublicKey()
	if pubKey == "" {
		t.Error("GetPublicKey returned empty string")
	}
	// Ed25519 public keys are 32 bytes, base64 encoded = 44 chars
	if len(pubKey) != 44 {
		t.Errorf("public key length = %d, want 44", len(pubKey))
	}
}

func TestIsConfigured(t *testing.T) {
	// Should be configured (either from env or auto-generated)
	if !IsConfigured() {
		t.Error("IsConfigured returned false")
	}
}

func BenchmarkSignURL(b *testing.B) {
	destURL := "https://example.com/page?ref=test&utm=campaign"
	linkID := "bench123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SignURL(destURL, linkID, 300)
	}
}

func BenchmarkVerifyURL(b *testing.B) {
	destURL := "https://example.com/page?ref=test&utm=campaign"
	linkID := "bench123"
	signedURL, _ := SignURL(destURL, linkID, 300)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyURL(signedURL, 300)
	}
}
