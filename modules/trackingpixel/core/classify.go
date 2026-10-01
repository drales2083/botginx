package core

import "strings"

// Classify decides what a pixel fetch represents, from the User-Agent and HTTP
// method. Precedence is fixed (see the plan's Global Constraints).
func Classify(userAgent, method string) Classification {
	if strings.EqualFold(method, "HEAD") {
		return ClassPrefetch
	}
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return ClassBot // no UA is never a real human mail client
	}
	if containsAny(ua, proxySignatures) {
		return ClassProxied
	}
	if containsAny(ua, appleMPPSignatures) {
		return ClassAppleMPP
	}
	if containsAny(ua, botSignatures) {
		return ClassBot
	}
	if containsAny(ua, prefetchSignatures) {
		return ClassPrefetch
	}
	return ClassHuman
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
