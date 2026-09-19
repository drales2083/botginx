// Package namegen generates readable two-word names such as "amber-canyon",
// used to suggest subdomains.
//
// The vocabulary lives in words.go and is generated from WordNet by
// tools/genwords. WordNet is the source rather than a flat dictionary because
// its index files are split by part of speech, which is what keeps names in the
// readable adjective-noun shape instead of pairing two arbitrary words.
// Every word is lowercase a-z, so any generated name is a valid DNS label.
package namegen

//go:generate go run ../../tools/genwords -adj dict/index.adj -noun dict/index.noun

import (
	"math/rand/v2"
	"strings"
)

// Generate returns an adjective-noun name joined by sep, e.g. "amber-canyon".
func Generate(sep string) string {
	adjective := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]

	// Some words are both an adjective and a noun; "light-light" reads like a bug.
	for adjective == noun {
		noun = nouns[rand.IntN(len(nouns))]
	}

	return adjective + sep + noun
}

// Subdomain returns a name safe to use as a DNS label.
func Subdomain() string {
	return Generate("-")
}

// Path returns a single random noun for use as a URL path.
func Path() string {
	return nouns[rand.IntN(len(nouns))]
}

// ShortWord returns a random short word (max 5 chars) for compact subdomains.
func ShortWord() string {
	return shortWords[rand.IntN(len(shortWords))]
}

// Combinations reports how many distinct names the vocabulary can produce.
func Combinations() int {
	return len(adjectives) * len(nouns)
}

// Vocabulary reports the size of each word list.
func Vocabulary() (adjectiveCount, nounCount int) {
	return len(adjectives), len(nouns)
}

// Character sets for tracking-style URLs
const (
	lowercase    = "abcdefghijklmnopqrstuvwxyz"
	alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"
	base64url    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

// randomString generates a random string of given length from charset
func randomString(length int, charset string) string {
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		sb.WriteByte(charset[rand.IntN(len(charset))])
	}
	return sb.String()
}

// TrackingPath generates a tracking-style URL path that mimics enterprise email links.
// Example: ss/c/u001.wWhzj7xDlZj-03YaXV1yHF/4tq/CxP-a-gDSfui04/h1/h001.6Or6ZDA0Luw
func TrackingPath() string {
	var sb strings.Builder

	// Segment 1: double letter (ss, ll, mm, etc.)
	letter := lowercase[rand.IntN(len(lowercase))]
	sb.WriteByte(letter)
	sb.WriteByte(letter)
	sb.WriteByte('/')

	// Segment 2: single letter
	sb.WriteByte(lowercase[rand.IntN(len(lowercase))])
	sb.WriteByte('/')

	// Segment 3: prefix.hash (u001.wWhzj7xDlZj-03YaXV1yHF)
	sb.WriteByte(lowercase[rand.IntN(len(lowercase))])
	sb.WriteString("001.")
	sb.WriteString(randomString(20+rand.IntN(10), base64url))
	sb.WriteByte('/')

	// Segment 4: 3-char alphanumeric (4tq)
	sb.WriteString(randomString(3, alphanumeric))
	sb.WriteByte('/')

	// Segment 5: hash (CxP-a-gDSfui04)
	sb.WriteString(randomString(10+rand.IntN(5), base64url))
	sb.WriteByte('/')

	// Segment 6: letter + digit (h1)
	sb.WriteByte(lowercase[rand.IntN(len(lowercase))])
	sb.WriteByte('0' + byte(rand.IntN(10)))
	sb.WriteByte('/')

	// Segment 7: prefix.hash (h001.6Or6ZDA0Luw)
	sb.WriteByte(lowercase[rand.IntN(len(lowercase))])
	sb.WriteString("001.")
	sb.WriteString(randomString(10+rand.IntN(5), base64url))

	return sb.String()
}
