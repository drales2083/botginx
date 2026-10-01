package core

import (
	"bytes"
	"strings"
)

// DetectType sniffs the document type from its bytes. It returns "text/html",
// "application/pdf", or "" when the type is unsupported. The declared type is
// not trusted; it is ignored unless sniffing is inconclusive and even then only
// confirms a supported type the bytes already look like.
func DetectType(in []byte, declared string) string {
	if bytes.HasPrefix(in, []byte("%PDF-")) {
		return "application/pdf"
	}
	head := strings.ToLower(strings.TrimSpace(string(firstN(in, 512))))
	if strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html") {
		return "text/html"
	}
	if strings.Contains(head, "<html") || strings.Contains(head, "<body") {
		return "text/html"
	}
	return ""
}

func firstN(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
