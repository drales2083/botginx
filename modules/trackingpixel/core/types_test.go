package core

import "testing"

func TestClassificationString(t *testing.T) {
	cases := map[Classification]string{
		ClassHuman: "human", ClassPrefetch: "prefetch", ClassAppleMPP: "apple_mpp",
		ClassProxied: "proxied", ClassBot: "bot", Classification(99): "unknown",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", c, got, want)
		}
	}
}
