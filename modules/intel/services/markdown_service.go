package services

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type TOCItem struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

type MarkdownService struct {
	policy *bluemonday.Policy
}

func NewMarkdownService() *MarkdownService {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class").OnElements("code", "pre", "span", "div")
	policy.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	policy.AllowElements("iframe")
	policy.AllowAttrs("src", "width", "height", "frameborder", "allowfullscreen").OnElements("iframe")
	return &MarkdownService{policy: policy}
}

func (s *MarkdownService) Render(markdown string) string {
	html := blackfriday.Run([]byte(markdown),
		blackfriday.WithExtensions(
			blackfriday.CommonExtensions|
				blackfriday.AutoHeadingIDs|
				blackfriday.Footnotes,
		),
	)

	safe := s.policy.SanitizeBytes(html)
	result := s.embedYouTube(string(safe))

	return result
}

func (s *MarkdownService) ExtractTOC(markdown string) []TOCItem {
	var items []TOCItem
	re := regexp.MustCompile(`(?m)^(#{2,3})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(markdown, -1)

	for _, match := range matches {
		level := len(match[1])
		text := strings.TrimSpace(match[2])
		id := slugify(text)
		items = append(items, TOCItem{Level: level, ID: id, Text: text})
	}

	return items
}

func (s *MarkdownService) embedYouTube(html string) string {
	re := regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`)
	return re.ReplaceAllString(html, `<div class="ratio ratio-16x9 my-3"><iframe src="https://www.youtube.com/embed/$1" allowfullscreen></iframe></div>`)
}

func slugify(text string) string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	text = re.ReplaceAllString(text, "-")
	return strings.Trim(text, "-")
}
