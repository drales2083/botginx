// Command genwords builds the namegen word lists from WordNet index files.
//
// WordNet is used rather than a flat dictionary because its index files are
// split by part of speech, which is what lets generated names keep the
// readable "adjective-noun" shape (amber-canyon) instead of pairing two
// arbitrary words (abaculus-zymurgy).
//
// Usage:
//
//	curl -sL https://wordnetcode.princeton.edu/wn3.1.dict.tar.gz -o /tmp/wn.tar.gz
//	tar xzf /tmp/wn.tar.gz -C /tmp
//	go run ./tools/genwords -adj /tmp/dict/index.adj -noun /tmp/dict/index.noun
//
// Optionally layer a profanity blocklist -- strongly recommended, since these
// names become public hostnames:
//
//	curl -sL https://raw.githubusercontent.com/LDNOOBW/List-of-Dirty-Naughty-Obscene-and-Otherwise-Bad-Words/master/en -o /tmp/blocklist.txt
//	go run ./tools/genwords -adj ... -noun ... -blocklist /tmp/blocklist.txt
//
// The output overwrites pkg/namegen/words.go, so the package always compiles
// whether or not this has been run.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

func main() {
	var (
		adjPath    = flag.String("adj", "", "path to WordNet index.adj (required)")
		nounPath   = flag.String("noun", "", "path to WordNet index.noun (required)")
		blockPath  = flag.String("blocklist", "", "path to a newline-separated blocklist")
		outPath    = flag.String("out", "pkg/namegen/words.go", "output Go file")
		minLen     = flag.Int("min", 4, "minimum word length")
		maxLen     = flag.Int("max", 8, "maximum word length")
		substrings = flag.Bool("substring", false,
			"reject words containing a blocklist entry, not just exact matches "+
				"(catches variants, but also kills innocent words: 'ass' would drop 'class' and 'grass')")
	)
	flag.Parse()

	if *adjPath == "" || *nounPath == "" {
		flag.Usage()
		log.Fatal("both -adj and -noun are required")
	}
	if *minLen < 1 || *maxLen < *minLen {
		log.Fatalf("invalid length range %d..%d", *minLen, *maxLen)
	}

	blocked, err := loadBlocklist(*blockPath)
	if err != nil {
		log.Fatalf("blocklist: %v", err)
	}
	if *blockPath == "" {
		log.Print("warning: no -blocklist given; generated names are unfiltered for profanity")
	}

	filter := &filter{
		min:        *minLen,
		max:        *maxLen,
		blocked:    blocked,
		substrings: *substrings,
	}

	adjectives, err := readIndex(*adjPath, filter)
	if err != nil {
		log.Fatalf("reading %s: %v", *adjPath, err)
	}
	nouns, err := readIndex(*nounPath, filter)
	if err != nil {
		log.Fatalf("reading %s: %v", *nounPath, err)
	}

	if len(adjectives) == 0 || len(nouns) == 0 {
		log.Fatalf("refusing to write empty lists (%d adjectives, %d nouns) -- check the input files",
			len(adjectives), len(nouns))
	}

	src, err := render(adjectives, nouns)
	if err != nil {
		log.Fatalf("formatting output: %v", err)
	}
	if err := os.WriteFile(*outPath, src, 0o644); err != nil {
		log.Fatalf("writing %s: %v", *outPath, err)
	}

	log.Printf("wrote %s: %d adjectives x %d nouns = %d combinations",
		*outPath, len(adjectives), len(nouns), len(adjectives)*len(nouns))
}

// wordRe keeps only plain lowercase words. This drops WordNet's multi-word
// entries (written with underscores), hyphenated forms, and anything with
// digits or punctuation -- none of which belong in a DNS label.
var wordRe = regexp.MustCompile(`^[a-z]+$`)

type filter struct {
	min, max   int
	blocked    map[string]bool
	substrings bool
}

func (f *filter) keep(word string) bool {
	if len(word) < f.min || len(word) > f.max {
		return false
	}
	if !wordRe.MatchString(word) {
		return false
	}
	if f.blocked[word] {
		return false
	}
	if f.substrings {
		for bad := range f.blocked {
			if strings.Contains(word, bad) {
				return false
			}
		}
	}
	return true
}

// readIndex parses a WordNet index file. Lines in the licence header begin
// with a space; every other line starts with the lemma.
func readIndex(path string, f *filter) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	seen := make(map[string]bool)
	var words []string

	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}

		lemma, _, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if seen[lemma] || !f.keep(lemma) {
			continue
		}

		seen[lemma] = true
		words = append(words, lemma)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	sort.Strings(words)
	return words, nil
}

func loadBlocklist(path string) (map[string]bool, error) {
	blocked := make(map[string]bool)
	if path == "" {
		return blocked, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		word := strings.ToLower(strings.TrimSpace(sc.Text()))
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		blocked[word] = true
	}
	return blocked, sc.Err()
}

func render(adjectives, nouns []string) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(`// Code generated by tools/genwords. DO NOT EDIT.
//
// Source: WordNet index.adj and index.noun, filtered to plain lowercase words
// safe to use as DNS labels. Regenerate with:
//
//	go run ./tools/genwords -adj dict/index.adj -noun dict/index.noun -blocklist blocklist.txt

package namegen

`)

	writeSlice(&buf, "adjectives", adjectives)
	buf.WriteString("\n")
	writeSlice(&buf, "nouns", nouns)

	return format.Source(buf.Bytes())
}

func writeSlice(buf *bytes.Buffer, name string, words []string) {
	fmt.Fprintf(buf, "var %s = []string{\n", name)

	const perLine = 8
	for i, word := range words {
		if i%perLine == 0 {
			buf.WriteString("\t")
		}
		fmt.Fprintf(buf, "%q,", word)
		if i%perLine == perLine-1 || i == len(words)-1 {
			buf.WriteString("\n")
		} else {
			buf.WriteString(" ")
		}
	}

	buf.WriteString("}\n")
}
