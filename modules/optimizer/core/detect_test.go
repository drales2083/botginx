package core

import "testing"

func TestDetectType(t *testing.T) {
	cases := []struct {
		name     string
		in       []byte
		declared string
		want     string
	}{
		{"pdf magic", []byte("%PDF-1.7\n..."), "", "application/pdf"},
		{"html doctype", []byte("<!DOCTYPE html><html></html>"), "", "text/html"},
		{"html tag", []byte("  <html><body>hi</body></html>"), "", "text/html"},
		{"declared pdf wins on ambiguous", []byte("plain text"), "application/pdf", ""},
		{"unsupported", []byte("GIF89a..."), "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectType(c.in, c.declared); got != c.want {
				t.Fatalf("DetectType = %q, want %q", got, c.want)
			}
		})
	}
}
