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

import "math/rand/v2"

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

// Combinations reports how many distinct names the vocabulary can produce.
func Combinations() int {
	return len(adjectives) * len(nouns)
}

// Vocabulary reports the size of each word list.
func Vocabulary() (adjectiveCount, nounCount int) {
	return len(adjectives), len(nouns)
}
