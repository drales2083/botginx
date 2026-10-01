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

// ObfuscationLevel selects how aggressively the HTML sanitizer strips content.
// Link handling is independent and still controlled by Options.LinkMode.
type ObfuscationLevel int

const (
	// LevelStandard (zero value, default) removes scripts, embeds, trackers,
	// event handlers and dangerous URL schemes, but preserves styling and
	// ordinary remote images.
	LevelStandard ObfuscationLevel = iota
	// LevelAggressive additionally removes <style> blocks, external
	// stylesheet/import/prefetch/preload links and inline styles containing
	// url( or expression(.
	LevelAggressive
	// LevelMaximum additionally removes remote external media (remote <img>,
	// srcset, poster, background, remote SVG image/use refs) so the output
	// cannot phone home. data: images are kept.
	LevelMaximum
)

// Options controls a single Optimize call.
type Options struct {
	Level    ObfuscationLevel // sanitizer strength; zero value is LevelStandard
	LinkMode LinkMode
	WrapBase string        // absolute http(s) base URL, e.g. "https://app.example.com/optimizer/r" (not a relative path); wrap is used only when LinkMode == LinkWrap and this is absolute, else links are neutralized
	SignKey  []byte        // HMAC key for signed tokens; required when LinkMode == LinkWrap
	TokenTTL time.Duration // how long a wrapped-link token stays valid
	MaxBytes int64         // reject inputs larger than this; 0 means no limit
}
