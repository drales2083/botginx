package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// TransparentGIF is a 1×1 fully transparent GIF89a (43 bytes).
var TransparentGIF = mustHex("47494638396101000100800000000000ffffff21f90401000000002c00000000010001000002024401003b")

// TransparentPNG is a 1×1 transparent PNG (74 bytes, generated via image/png).
var TransparentPNG = mustHex("89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000001149444154789c626260606000040000ffff000f0003fe8febcf0000000049454e44ae426082")

func mustHex(s string) []byte {
	b, err := hex.DecodeString(stripSpaces(s))
	if err != nil {
		panic("tracking-pixel: bad embedded image hex: " + err.Error())
	}
	return b
}

func stripSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// ContentType maps a format to its MIME type; unknown formats default to gif.
func ContentType(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/gif"
}

// ImageFor returns the 1×1 image bytes for a format; unknown defaults to gif.
func ImageFor(format string) []byte {
	if format == "png" {
		return TransparentPNG
	}
	return TransparentGIF
}

// NewToken returns a URL-safe random token for a pixel URL.
func NewToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
