package core

// Classification is the verdict for one pixel fetch (email open).
type Classification int

const (
	ClassHuman    Classification = iota // a real human open ("verified")
	ClassPrefetch                       // client/proxy prefetch, not a human view
	ClassAppleMPP                       // Apple Mail Privacy Protection proxy fetch
	ClassProxied                        // image proxy (e.g. Gmail GoogleImageProxy)
	ClassBot                            // scanner / crawler / library
)

// String returns the stable wire/DB label for the classification.
func (c Classification) String() string {
	switch c {
	case ClassHuman:
		return "human"
	case ClassPrefetch:
		return "prefetch"
	case ClassAppleMPP:
		return "apple_mpp"
	case ClassProxied:
		return "proxied"
	case ClassBot:
		return "bot"
	default:
		return "unknown"
	}
}
