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
	doc.Find("iframe, object, embed, frame, frameset, applet, base, noscript").
		Each(func(_ int, s *goquery.Selection) {
			rep.ActiveContentRemoved++
			s.Remove()
		})
	if opts.Level >= LevelAggressive {
		doc.Find("style").Each(func(_ int, s *goquery.Selection) {
			rep.ActiveContentRemoved++
			s.Remove()
		})
	}
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("http-equiv"); ok && strings.EqualFold(strings.TrimSpace(v), "refresh") {
			rep.ActiveContentRemoved++
			s.Remove()
		}
	})
	// External resource links (CSS, imports, prefetch/preload) can carry
	// beacons; icons and canonical/alternate links are left alone.
	doc.Find("link").Each(func(_ int, s *goquery.Selection) {
		if opts.Level < LevelAggressive {
			return
		}
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

	if opts.Level == LevelMaximum {
		doc.Find("video, audio, source, track").Each(func(_ int, s *goquery.Selection) {
			if src, _ := s.Attr("src"); isRemoteURL(src) {
				rep.ActiveContentRemoved++
				s.Remove()
			}
		})
		doc.Find("input").Each(func(_ int, s *goquery.Selection) {
			typ, _ := s.Attr("type")
			if src, ok := s.Attr("src"); ok && strings.EqualFold(strings.TrimSpace(typ), "image") && isRemoteURL(src) {
				rep.ActiveContentRemoved++
				s.RemoveAttr("src")
			}
		})
		// Favicon fetches are beacons too.
		doc.Find("link").Each(func(_ int, s *goquery.Selection) {
			rel, _ := s.Attr("rel")
			for _, tok := range strings.Fields(strings.ToLower(rel)) {
				if strings.HasSuffix(tok, "icon") || strings.HasSuffix(tok, "icon-precomposed") {
					rep.ActiveContentRemoved++
					s.Remove()
					return
				}
			}
		})
		doc.Find("img").Each(func(_ int, s *goquery.Selection) {
			if src, _ := s.Attr("src"); isRemoteURL(src) {
				rep.TrackersRemoved++
				s.Remove()
			}
		})
	}

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
				if opts.Level == LevelMaximum {
					switch {
					case (key == "srcset" || key == "poster" || key == "background") && isRemoteURL(a.Val),
						key == "srcset" && strings.Contains(a.Val, "//"):
						rep.ActiveContentRemoved++
						continue
					case (tag == "image" || tag == "use") &&
						(key == "href" || key == "xlink:href" || strings.EqualFold(a.Namespace, "xlink")) &&
						isRemoteURL(a.Val):
						rep.ActiveContentRemoved++
						a.Val = "#"
						kept = append(kept, a)
						continue
					}
				}
				if key == "style" && opts.Level >= LevelAggressive {
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

// normalizeURL mimics browser URL preprocessing: embedded whitespace and
// control characters (tab, CR, LF, ...) are dropped, case is folded, and
// backslashes count as slashes.
func normalizeURL(v string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r <= ' ':
			return -1
		case r == '\\':
			return '/'
		}
		return r
	}, strings.ToLower(v))
}

// isDangerousScheme reports whether v uses a scripting or HTML-carrying scheme.
func isDangerousScheme(v string) bool {
	clean := normalizeURL(v)
	return strings.HasPrefix(clean, "javascript:") || strings.HasPrefix(clean, "vbscript:") ||
		strings.HasPrefix(clean, "data:text/html") || strings.HasPrefix(clean, "data:application/")
}

// isRemoteURL reports whether v is an http(s) or protocol-relative URL after
// browser-style normalization.
func isRemoteURL(v string) bool {
	t := normalizeURL(v)
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") || strings.HasPrefix(t, "//")
}
