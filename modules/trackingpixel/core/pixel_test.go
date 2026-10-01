package core

import (
	"bytes"
	"image/gif"
	"image/png"
	"strings"
	"testing"
)

func TestTransparentImages(t *testing.T) {
	if len(TransparentGIF) < 6 || string(TransparentGIF[:6]) != "GIF89a" {
		t.Error("GIF missing GIF89a magic")
	}
	if len(TransparentPNG) < 8 || string(TransparentPNG[1:4]) != "PNG" {
		t.Error("PNG missing PNG magic")
	}
}

func TestTransparentGIFDecodes(t *testing.T) {
	img, err := gif.Decode(bytes.NewReader(TransparentGIF))
	if err != nil {
		t.Fatalf("gif decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("gif bounds = %v, want 1x1", b)
	}
}

func TestTransparentPNGDecodes(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(TransparentPNG))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Fatalf("png bounds = %v, want 1x1", b)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("png pixel alpha = %d, want 0", a)
	}
}

func TestContentTypeAndImageFor(t *testing.T) {
	if ContentType("png") != "image/png" || ContentType("gif") != "image/gif" {
		t.Fatal("content type mismatch")
	}
	if ContentType("weird") != "image/gif" {
		t.Fatal("default should be gif")
	}
	if &ImageFor("png")[0] != &TransparentPNG[0] {
		t.Error("ImageFor(png) should return the PNG bytes")
	}
}

func TestNewToken(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) < 16 || strings.ContainsAny(tok, "+/=") {
			t.Fatalf("token not url-safe/long enough: %q", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token: %q", tok)
		}
		seen[tok] = true
	}
}
