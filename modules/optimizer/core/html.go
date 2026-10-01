package core

import (
	"bytes"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// processHTML cleans an HTML document: removes scripts, tracking pixels and
// inline event handlers, and rewrites links per opts.LinkMode.
func processHTML(in []byte, opts Options, rep *Report) ([]byte, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}

	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		rep.ScriptsRemoved++
		s.Remove()
	})

	// Active / embedding / hijacking elements are removed outright.
	doc.Find("iframe, object, embed, frame, frameset, applet, base, noscript, style").
		Each(func(_ int, s *goquery.Selection) {
			rep.ActiveContentRemoved++
			s.Remove()
		})
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("http-equiv"); ok && strings.EqualFold(strings.TrimSpace(v), "refresh") {
			rep.ActiveContentRemoved++
			s.Remove()
		}
	})
	// External resource links (CSS, imports, prefetch/preload) can carry
	// beacons; icons and canonical/alternate links are left alone.
	doc.Find("link").Each(func(_ int, s *goquery.Selection) {
		rel, _ := s.Attr("rel")
		for _, tok := range strings.Fields(strings.ToLower(rel)) {
			switch tok {
			case "stylesheet", "import", "prefetch", "preload", "prerender", "dns-prefetch", "preconnect", "modulepreload":
				rep.ActiveContentRemoved++
				s.Remove()
				return
			}
		}
	})

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		w, _ := s.Attr("width")
		h, _ := s.Attr("height")
		if isTrackerSize(w) && isTrackerSize(h) {
			rep.TrackersRemoved++
			s.Remove()
		}
	})

	// Attribute pass over every element.
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		for _, node := range s.Nodes {
			tag := strings.ToLower(node.Data)
			kept := node.Attr[:0]
			for _, a := range node.Attr {
				key := strings.ToLower(a.Key)
				if len(key) > 2 && strings.HasPrefix(key, "on") || key == "srcdoc" {
					continue // drop onclick, onload, ..., srcdoc
				}
				if key == "style" {
					lv := strings.ToLower(a.Val)
					if strings.Contains(lv, "url(") || strings.Contains(lv, "expression(") {
						rep.ActiveContentRemoved++
						continue
					}
				}
				if key == "ping" && (tag == "a" || tag == "area") {
					rep.TrackersRemoved++
					continue
				}
				name := key
				name = strings.TrimPrefix(name, "xlink:")
				isXlink := name != key || strings.EqualFold(a.Namespace, "xlink")
				switch {
				case name == "formaction",
					name == "href" && (tag == "a" || tag == "area" || isXlink),
					name == "action" && tag == "form":
					if a.Val != "" {
						rep.LinksHandled++
						a.Val = rewriteLink(a.Val, opts)
					}
				case name == "href" || name == "src" || name == "action" || name == "data":
					if isDangerousScheme(a.Val) {
						a.Val = NeutralizeURL(a.Val)
					}
				}
				kept = append(kept, a)
			}
			node.Attr = kept
		}
	})

	out, err := doc.Html()
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func rewriteLink(raw string, opts Options) string {
	if opts.LinkMode == LinkWrap && IsHTTPURL(raw) && IsHTTPURL(opts.WrapBase) {
		tok, err := SignToken(opts.SignKey, raw, opts.TokenTTL)
		if err == nil {
			return WrapURL(opts.WrapBase, tok)
		}
	}
	return NeutralizeURL(raw)
}

func isTrackerSize(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "0", "1px", "0px":
		return true
	}
	return false
}

// isDangerousScheme reports whether v uses a scripting or HTML-carrying
// scheme. Whitespace and control characters browsers ignore are stripped first.
func isDangerousScheme(v string) bool {
	clean := strings.Map(func(r rune) rune {
		if r <= ' ' {
			return -1
		}
		return r
	}, strings.ToLower(v))
	return strings.HasPrefix(clean, "javascript:") || strings.HasPrefix(clean, "vbscript:") ||
		strings.HasPrefix(clean, "data:text/html") || strings.HasPrefix(clean, "data:application/")
}
