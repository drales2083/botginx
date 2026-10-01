package core

import (
	"strings"
	"testing"
	"time"
)

func htmlOpts(mode LinkMode) (Options, *Report) {
	return Options{
		LinkMode: mode,
		WrapBase: "https://app.example.com/optimizer/r/",
		SignKey:  []byte("k-k-k-k-k-k-k-k"),
		TokenTTL: time.Hour,
	}, &Report{}
}

func TestHTMLRemovesScriptAndTracker(t *testing.T) {
	in := []byte(`<html><body>
<script>track()</script>
<img src="https://a.com/p.gif" width="1" height="1">
<p onclick="x()">hi</p>
<a href="http://real.com">link</a>
</body></html>`)
	opts, rep := htmlOpts(LinkNeutralize)
	out, err := processHTML(in, opts, rep)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<script") {
		t.Error("script not removed")
	}
	if strings.Contains(s, "p.gif") {
		t.Error("tracker image not removed")
	}
	if strings.Contains(s, "onclick") {
		t.Error("event handler not removed")
	}
	if !strings.Contains(s, "hxxp://real.com") {
		t.Error("link not neutralized")
	}
	if rep.ScriptsRemoved != 1 || rep.TrackersRemoved != 1 || rep.LinksHandled != 1 {
		t.Errorf("counts: scripts=%d trackers=%d links=%d",
			rep.ScriptsRemoved, rep.TrackersRemoved, rep.LinksHandled)
	}
}

func TestHTMLWrapLinks(t *testing.T) {
	in := []byte(`<a href="https://real.com/x">link</a>`)
	opts, rep := htmlOpts(LinkWrap)
	out, err := processHTML(in, opts, rep)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `href="https://app.example.com/optimizer/r/`) {
		t.Fatalf("link not wrapped: %s", s)
	}
	// extract token and verify it decodes back to the original URL
	i := strings.Index(s, "https://app.example.com/optimizer/r/")
	rest := s[i+len("https://app.example.com/optimizer/r/"):]
	tok := rest[:strings.IndexByte(rest, '"')]
	got, err := VerifyToken(opts.SignKey, tok)
	if err != nil || got != "https://real.com/x" {
		t.Fatalf("token verify: url=%q err=%v", got, err)
	}
}

func hardenRun(t *testing.T, in string) (string, *Report) {
	t.Helper()
	opts, rep := htmlOpts(LinkNeutralize)
	out, err := processHTML([]byte(in), opts, rep)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), rep
}

func TestHTMLHardeningRemovals(t *testing.T) {
	cases := []struct{ name, in, gone string }{
		{"iframe", `<iframe src="https://e.com"></iframe><p>x</p>`, "<iframe"},
		{"object", `<object data="https://e.com/a.swf"></object>`, "<object"},
		{"embed", `<embed src="https://e.com/a.swf">`, "<embed"},
		{"applet", `<applet code="A"></applet>`, "<applet"},
		{"frameset", `<frameset><frame src="https://e.com"></frameset>`, "<frame"},
		{"meta refresh", `<meta HTTP-EQUIV="Refresh" content="0;url=https://e.com">`, "e.com"},
		{"base", `<base href="https://e.com/">`, "<base"},
		{"link stylesheet", `<link rel="stylesheet" href="https://e.com/a.css">`, "a.css"},
		{"link preload", `<link rel="preload" href="https://e.com/a.js">`, "a.js"},
		{"link prefetch", `<link rel="prefetch" href="https://e.com/a">`, "e.com"},
		{"style", `<style>body{background:url(https://e.com/b.gif)}</style>`, "b.gif"},
		{"noscript", `<noscript><img src="https://e.com/n.gif"></noscript>`, "n.gif"},
		{"svg script", `<svg><script>alert(1)</script></svg>`, "alert(1)"},
		{"srcdoc", `<div srcdoc="<b>x</b>">y</div>`, "srcdoc"},
		{"1px tracker", `<img src="https://e.com/t.gif" width="1px" height="0px">`, "t.gif"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, rep := hardenRun(t, `<html><head></head><body>`+c.in+`</body></html>`)
			if strings.Contains(out, c.gone) {
				t.Errorf("%q still present in %s", c.gone, out)
			}
			if rep.ScriptsRemoved+rep.TrackersRemoved+rep.ActiveContentRemoved == 0 && c.name != "srcdoc" && c.name != "frameset" {
				t.Error("nothing counted")
			}
		})
	}
}

func TestHTMLHardeningKeepsLegitContent(t *testing.T) {
	out, _ := hardenRun(t, `<img src="https://e.com/photo.jpg" width="200" height="100"><link rel="icon" href="https://e.com/f.ico">`)
	if !strings.Contains(out, "photo.jpg") {
		t.Error("legit image removed")
	}
}

func TestHTMLHardeningURLAttrs(t *testing.T) {
	out, rep := hardenRun(t, `<map><area href="http://a.com/x"></map>
<button formaction="http://b.com/y">b</button><input formaction="https://c.com/z">
<a href="javascript:alert(1)">j</a><a href="  JaVaScRiPt:alert(1)">j2</a>
<img src="data:text/html,<script>1</script>"><a href="vbscript:x">v</a>
<svg><a xlink:href="javascript:alert(1)"><text>t</text></a><a xlink:href="http://d.com/q"><text>u</text></a></svg>`)
	for _, bad := range []string{"javascript:", "vbscript:", "data:text/html", "http://a.com", "http://b.com", "https://c.com", "http://d.com"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Errorf("%q survived: %s", bad, out)
		}
	}
	for _, good := range []string{"hxxp://a.com/x", "hxxp://b.com/y", "hxxps://c.com/z", "hxxp://d.com/q"} {
		if !strings.Contains(out, good) {
			t.Errorf("missing %q: %s", good, out)
		}
	}
	if rep.LinksHandled < 6 {
		t.Errorf("LinksHandled=%d", rep.LinksHandled)
	}
}

func TestHTMLInlineStyleBeacons(t *testing.T) {
	out, rep := hardenRun(t, `<body><div style="background:URL(https://e.com/b.gif)">a</div>
<p style="width:expression(alert(1))">b</p><span style="color:red">c</span></body>`)
	if strings.Contains(out, "b.gif") || strings.Contains(out, "expression(") {
		t.Errorf("style beacon survived: %s", out)
	}
	if !strings.Contains(out, `style="color:red"`) {
		t.Errorf("legit style removed: %s", out)
	}
	if rep.ActiveContentRemoved != 2 {
		t.Errorf("ActiveContentRemoved=%d", rep.ActiveContentRemoved)
	}
}

func TestHTMLPingStripped(t *testing.T) {
	out, rep := hardenRun(t, `<body><a href="http://a.com" ping="https://t.com/p">x</a><map><area href="http://b.com" ping="https://t.com/q"></map></body>`)
	if strings.Contains(out, "ping") || strings.Contains(out, "t.com") {
		t.Errorf("ping survived: %s", out)
	}
	if rep.TrackersRemoved != 2 {
		t.Errorf("TrackersRemoved=%d", rep.TrackersRemoved)
	}
}

func TestHTMLSVGDangerousRefs(t *testing.T) {
	out, _ := hardenRun(t, `<body><svg><image href="javascript:alert(1)"/><use xlink:href="data:text/html,x"/><image href="https://e.com/ok.png"/></svg></body>`)
	if strings.Contains(out, "javascript:") || strings.Contains(out, "data:text/html") {
		t.Errorf("dangerous svg ref survived: %s", out)
	}
	if !strings.Contains(out, "ok.png") {
		t.Errorf("normal svg image removed: %s", out)
	}
}

func TestHTMLWrapRelativeBaseNeutralizes(t *testing.T) {
	for _, base := range []string{"/user/optimizer/r/", ""} {
		opts, rep := htmlOpts(LinkWrap)
		opts.WrapBase = base
		out, err := processHTML([]byte(`<a href="https://real.com/x">l</a>`), opts, rep)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, `href="hxxps://real.com/x"`) || strings.Contains(s, "optimizer") {
			t.Fatalf("base %q: want neutralized, got %s", base, s)
		}
	}
}

func TestHTMLSVGPlainHrefDangerousNeutralized(t *testing.T) {
	out, _ := hardenRun(t, `<svg><use href="javascript:alert(1)"/><image href="data:text/html,x"/></svg>`)
	if strings.Contains(out, "javascript:") || strings.Contains(out, "data:text/html") {
		t.Fatalf("dangerous href survived: %s", out)
	}
}
