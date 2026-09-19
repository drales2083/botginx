// Package domainmask provides utilities for partially hiding domain names.
package domainmask

import (
	"strings"
)

// Mask partially hides a domain name, showing first 2 chars and end, with stars in middle.
// Example: "benjaminrock.me.uk" -> "be⭑⭑⭑⭑rock.me.uk"
func Mask(domain string) string {
	if domain == "" {
		return ""
	}

	// Split domain name from TLD
	name, tld := splitDomainTLD(domain)
	if name == "" {
		return domain // Can't parse, return as-is
	}

	// Very short names: mask most of it
	if len(name) <= 3 {
		return strings.Repeat("⭑", len(name)) + tld
	}

	// Short names (4-6 chars): show first 2, mask middle, show last 1
	if len(name) <= 6 {
		masked := name[:2] + strings.Repeat("⭑", len(name)-3) + name[len(name)-1:]
		return masked + tld
	}

	// Medium/long names: show first 2, mask 4 chars, show rest
	// "benjaminrock" -> "be⭑⭑⭑⭑rock"
	maskLen := 4
	endStart := 2 + maskLen
	if endStart >= len(name) {
		endStart = len(name) - 2
		if endStart < 2 {
			endStart = 2
		}
		maskLen = endStart - 2
	}

	masked := name[:2] + strings.Repeat("⭑", maskLen) + name[endStart:]
	return masked + tld
}

// splitDomainTLD separates the domain name from its TLD.
// Handles multi-part TLDs like .co.uk, .me.uk, .org.uk
func splitDomainTLD(domain string) (name, tld string) {
	// Common multi-part TLDs
	multiPartTLDs := []string{
		".co.uk", ".me.uk", ".org.uk", ".ac.uk", ".gov.uk", ".net.uk",
		".co.nz", ".co.za", ".com.au", ".net.au", ".org.au",
		".com.br", ".co.jp", ".co.kr", ".co.in",
	}

	lowerDomain := strings.ToLower(domain)

	// Check for multi-part TLDs first
	for _, mtld := range multiPartTLDs {
		if strings.HasSuffix(lowerDomain, mtld) {
			idx := len(domain) - len(mtld)
			return domain[:idx], domain[idx:]
		}
	}

	// Single-part TLD: find last dot
	lastDot := strings.LastIndex(domain, ".")
	if lastDot == -1 {
		return domain, "" // No TLD found
	}

	return domain[:lastDot], domain[lastDot:]
}
