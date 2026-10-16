package namegen

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	name := Generate("-")

	if name == "" {
		t.Error("Generate returned empty string")
	}

	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Errorf("expected 2 parts separated by -, got %d: %q", len(parts), name)
	}

	// Verify adjective and noun are different
	if parts[0] == parts[1] {
		t.Errorf("adjective and noun should be different: %q", name)
	}
}

func TestGenerate_CustomSeparator(t *testing.T) {
	name := Generate("_")

	if !strings.Contains(name, "_") {
		t.Errorf("expected underscore separator, got %q", name)
	}
}

func TestSubdomain(t *testing.T) {
	subdomain := Subdomain()

	if subdomain == "" {
		t.Error("Subdomain returned empty string")
	}

	// Must be valid DNS label
	if !regexp.MustCompile(`^[a-z]+-[a-z]+$`).MatchString(subdomain) {
		t.Errorf("subdomain not valid DNS label: %q", subdomain)
	}
}

func TestPath(t *testing.T) {
	path := Path()

	if path == "" {
		t.Error("Path returned empty string")
	}

	// Must be lowercase letters only
	if !regexp.MustCompile(`^[a-z]+$`).MatchString(path) {
		t.Errorf("path should be lowercase letters only: %q", path)
	}
}

func TestShortWord(t *testing.T) {
	word := ShortWord()

	if word == "" {
		t.Error("ShortWord returned empty string")
	}

	if len(word) > 5 {
		t.Errorf("ShortWord should be max 5 chars, got %d: %q", len(word), word)
	}
}

func TestCombinations(t *testing.T) {
	count := Combinations()

	// Should have a reasonable number of combinations
	if count < 1000 {
		t.Errorf("expected at least 1000 combinations, got %d", count)
	}
}

func TestVocabulary(t *testing.T) {
	adjCount, nounCount := Vocabulary()

	if adjCount < 10 {
		t.Errorf("expected at least 10 adjectives, got %d", adjCount)
	}

	if nounCount < 10 {
		t.Errorf("expected at least 10 nouns, got %d", nounCount)
	}
}

func TestTrackingPath(t *testing.T) {
	path := TrackingPath()

	if path == "" {
		t.Error("TrackingPath returned empty string")
	}

	// Should have multiple segments separated by /
	segments := strings.Split(path, "/")
	if len(segments) < 5 {
		t.Errorf("expected at least 5 segments, got %d: %q", len(segments), path)
	}

	// First segment should be double letter
	if len(segments[0]) != 2 || segments[0][0] != segments[0][1] {
		t.Errorf("first segment should be double letter, got %q", segments[0])
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	duplicates := 0

	// Generate 100 names and check for reasonable uniqueness
	for i := 0; i < 100; i++ {
		name := Generate("-")
		if seen[name] {
			duplicates++
		}
		seen[name] = true
	}

	// Allow some duplicates due to random chance, but not many
	if duplicates > 10 {
		t.Errorf("too many duplicates: %d out of 100", duplicates)
	}
}
