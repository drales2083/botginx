package core

import "time"

// LinkMode selects how links found in a document are handled.
type LinkMode int

const (
	// LinkNeutralize defangs links in place so they are not navigable.
	LinkNeutralize LinkMode = iota
	// LinkWrap replaces links with a signed redirector URL.
	LinkWrap
)

// Options controls a single Optimize call.
type Options struct {
	LinkMode LinkMode
	WrapBase string        // absolute http(s) base URL, e.g. "https://app.example.com/optimizer/r" (not a relative path); wrap is used only when LinkMode == LinkWrap and this is absolute, else links are neutralized
	SignKey  []byte        // HMAC key for signed tokens; required when LinkMode == LinkWrap
	TokenTTL time.Duration // how long a wrapped-link token stays valid
	MaxBytes int64         // reject inputs larger than this; 0 means no limit
}
