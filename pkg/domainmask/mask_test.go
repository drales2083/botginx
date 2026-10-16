package domainmask

import "testing"

func TestMask(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{
			name:     "empty string",
			domain:   "",
			expected: "",
		},
		{
			name:     "standard domain",
			domain:   "example.com",
			expected: "ex⭑⭑⭑⭑e.com",
		},
		{
			name:     "long domain name",
			domain:   "benjaminrock.me.uk",
			expected: "be⭑⭑⭑⭑inrock.me.uk",
		},
		{
			name:     "short domain (4 chars)",
			domain:   "test.com",
			expected: "te⭑t.com",
		},
		{
			name:     "very short domain (3 chars)",
			domain:   "abc.com",
			expected: "⭑⭑⭑.com",
		},
		{
			name:     "two char domain",
			domain:   "ab.io",
			expected: "⭑⭑.io",
		},
		{
			name:     "multi-part TLD co.uk",
			domain:   "shop.co.uk",
			expected: "sh⭑p.co.uk",
		},
		{
			name:     "multi-part TLD com.au",
			domain:   "mybusiness.com.au",
			expected: "my⭑⭑⭑⭑ness.com.au",
		},
		{
			name:     "no TLD treats as name",
			domain:   "localhost",
			expected: "lo⭑⭑⭑⭑ost",
		},
		{
			name:     "medium domain (6 chars)",
			domain:   "domain.org",
			expected: "do⭑⭑⭑n.org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mask(tt.domain)
			if got != tt.expected {
				t.Errorf("Mask(%q) = %q, want %q", tt.domain, got, tt.expected)
			}
		})
	}
}

func TestSplitDomainTLD(t *testing.T) {
	tests := []struct {
		domain       string
		expectedName string
		expectedTLD  string
	}{
		{"example.com", "example", ".com"},
		{"test.co.uk", "test", ".co.uk"},
		{"shop.me.uk", "shop", ".me.uk"},
		{"site.com.au", "site", ".com.au"},
		{"localhost", "localhost", ""},
		{"sub.domain.org", "sub.domain", ".org"},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			name, tld := splitDomainTLD(tt.domain)
			if name != tt.expectedName || tld != tt.expectedTLD {
				t.Errorf("splitDomainTLD(%q) = (%q, %q), want (%q, %q)",
					tt.domain, name, tld, tt.expectedName, tt.expectedTLD)
			}
		})
	}
}
