package core

import (
	"encoding/hex"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"os"
	"testing"
	"time"
)

func TestRewritePDFLinksNeutralize(t *testing.T) {
	in, err := os.ReadFile("testdata/link.pdf")
	if err != nil {
		t.Skip("fixture missing; run go run testdata/gen.go")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := pdfURIs(ctx); len(got) != 1 || got[0] != "https://real.com/x" {
		t.Fatalf("fixture URIs = %v", got)
	}
	rep := &Report{}
	opts := Options{LinkMode: LinkNeutralize}
	if err := rewritePDFLinks(ctx, opts, rep); err != nil {
		t.Fatal(err)
	}
	if rep.LinksHandled != 1 {
		t.Fatalf("LinksHandled = %d, want 1", rep.LinksHandled)
	}
	for _, u := range pdfURIs(ctx) {
		if u == "https://real.com/x" {
			t.Fatalf("original URI survived: %q", u)
		}
	}
}

func TestRewritePDFLinksWrap(t *testing.T) {
	in, err := os.ReadFile("testdata/link.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	rep := &Report{}
	key := []byte("pdf-key-123456")
	opts := Options{LinkMode: LinkWrap, WrapBase: "https://app.example.com/optimizer/r/", SignKey: key, TokenTTL: time.Hour}
	if err := rewritePDFLinks(ctx, opts, rep); err != nil {
		t.Fatal(err)
	}
	uris := pdfURIs(ctx)
	if len(uris) != 1 {
		t.Fatalf("want 1 uri, got %d", len(uris))
	}
	path := uris[0]
	const prefix = "https://app.example.com/optimizer/r/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		t.Fatalf("not wrapped: %q", path)
	}
	got, err := VerifyToken(key, path[len(prefix):])
	if err != nil || got != "https://real.com/x" {
		t.Fatalf("token: url=%q err=%v", got, err)
	}
}

func TestRewritePDFLinksSurvivesWrite(t *testing.T) {
	in, err := os.ReadFile("testdata/link.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := rewritePDFLinks(ctx, Options{LinkMode: LinkNeutralize}, &Report{}); err != nil {
		t.Fatal(err)
	}
	want := pdfURIs(ctx)
	out, err := writePDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx2, err := readPDF(out)
	if err != nil {
		t.Fatal(err)
	}
	got := pdfURIs(ctx2)
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("after write+read: %v, want %v", got, want)
	}
}

// setFixtureURI replaces the /URI object of the fixture's single link action.
func setFixtureURI(t *testing.T, ctx *model.Context, v types.Object) {
	t.Helper()
	n := 0
	if err := eachURIAction(ctx, func(action types.Dict) { action["URI"] = v; n++ }); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 link action, got %d", n)
	}
}

func readLinkFixture(t *testing.T) *model.Context {
	t.Helper()
	in, err := os.ReadFile("testdata/link.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestRewritePDFLinksEscapedParens(t *testing.T) {
	const real = "https://x.com/a_(b)"
	// Neutralize: defanged form of the decoded URL, no stray backslashes.
	ctx := readLinkFixture(t)
	setFixtureURI(t, ctx, types.StringLiteral(`https://x.com/a_\(b\)`))
	if got := pdfURIs(ctx); len(got) != 1 || got[0] != real {
		t.Fatalf("decoded = %v, want %q", got, real)
	}
	if err := rewritePDFLinks(ctx, Options{LinkMode: LinkNeutralize}, &Report{}); err != nil {
		t.Fatal(err)
	}
	if got := pdfURIs(ctx); len(got) != 1 || got[0] != "hxxps://x.com/a_(b)" {
		t.Fatalf("neutralized = %v", got)
	}
	// Wrap: the token must decode to the real URL.
	ctx = readLinkFixture(t)
	setFixtureURI(t, ctx, types.StringLiteral(`https://x.com/a_\(b\)`))
	key := []byte("pdf-key-123456")
	opts := Options{LinkMode: LinkWrap, WrapBase: "https://app.example.com/optimizer/r/", SignKey: key, TokenTTL: time.Hour}
	if err := rewritePDFLinks(ctx, opts, &Report{}); err != nil {
		t.Fatal(err)
	}
	got := pdfURIs(ctx)
	if len(got) != 1 {
		t.Fatalf("uris = %v", got)
	}
	url, err := VerifyToken(key, strings.TrimPrefix(got[0], "https://app.example.com/optimizer/r/"))
	if err != nil || url != real {
		t.Fatalf("token url=%q err=%v, want %q", url, err, real)
	}
}

func TestRewritePDFLinksHexURI(t *testing.T) {
	ctx := readLinkFixture(t)
	setFixtureURI(t, ctx, types.HexLiteral(hex.EncodeToString([]byte("https://real.com/x"))))
	if got := pdfURIs(ctx); len(got) != 1 || got[0] != "https://real.com/x" {
		t.Fatalf("decoded = %v", got)
	}
	rep := &Report{}
	if err := rewritePDFLinks(ctx, Options{LinkMode: LinkNeutralize}, rep); err != nil {
		t.Fatal(err)
	}
	if got := pdfURIs(ctx); len(got) != 1 || got[0] != "hxxps://real.com/x" {
		t.Fatalf("neutralized = %v (link lost?)", got)
	}
}

func uriAct(u string) types.Dict {
	return types.Dict{"S": types.Name("URI"), "URI": types.StringLiteral(u)}
}

// wrapOpts returns wrap options and the verify helper for tokens.
func wrapOpts() (Options, []byte) {
	key := []byte("pdf-key-123456")
	return Options{LinkMode: LinkWrap, WrapBase: "https://app.example.com/optimizer/r/", SignKey: key, TokenTTL: time.Hour}, key
}

func checkWrapped(t *testing.T, key []byte, got, want string) {
	t.Helper()
	const prefix = "https://app.example.com/optimizer/r/"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("not wrapped: %q", got)
	}
	u, err := VerifyToken(key, strings.TrimPrefix(got, prefix))
	if err != nil || u != want {
		t.Fatalf("token url=%q err=%v, want %q", u, err, want)
	}
}

func TestRewritePDFLinksNestedLocations(t *testing.T) {
	build := func(t *testing.T) (*model.Context, types.Dict, types.Dict, types.Dict, types.Dict) {
		ctx := readLinkFixture(t)
		page, _, _, err := ctx.PageDict(1, false)
		if err != nil {
			t.Fatal(err)
		}
		// Link /A (GoTo) chained to a URI via /Next.
		next := uriAct("https://next.com/n")
		chained := linkAnnot(types.Dict{"S": types.Name("GoTo"), "D": types.Array{}, "Next": next})
		// Widget annotation whose /AA holds a URI.
		aaAct := uriAct("https://aa.com/a")
		widget := types.Dict{"Subtype": types.Name("Widget"), "AA": types.Dict{"E": aaAct}}
		// Form field with /A URI (not on any page).
		fieldAct := uriAct("https://field.com/f")
		field := types.Dict{"FT": types.Name("Btn"), "A": fieldAct}
		root, err := ctx.Catalog()
		if err != nil {
			t.Fatal(err)
		}
		root["AcroForm"] = types.Dict{"Fields": types.Array{field}}
		orig, _ := page.Find("Annots")
		page["Annots"] = append(orig.(types.Array), chained, widget)
		return ctx, next, aaAct, fieldAct, page
	}
	get := func(d types.Dict) string { return uriString(d) }

	t.Run("neutralize", func(t *testing.T) {
		ctx, next, aaAct, fieldAct, _ := build(t)
		rep := &Report{}
		if err := rewritePDFLinks(ctx, Options{LinkMode: LinkNeutralize}, rep); err != nil {
			t.Fatal(err)
		}
		for u, d := range map[string]types.Dict{"hxxps://next.com/n": next, "hxxps://aa.com/a": aaAct, "hxxps://field.com/f": fieldAct} {
			if got := get(d); got != u {
				t.Errorf("got %q, want %q", got, u)
			}
		}
		if rep.LinksHandled != 4 {
			t.Errorf("LinksHandled = %d, want 4", rep.LinksHandled)
		}
	})
	t.Run("wrap", func(t *testing.T) {
		ctx, next, aaAct, fieldAct, _ := build(t)
		opts, key := wrapOpts()
		if err := rewritePDFLinks(ctx, opts, &Report{}); err != nil {
			t.Fatal(err)
		}
		checkWrapped(t, key, get(next), "https://next.com/n")
		checkWrapped(t, key, get(aaAct), "https://aa.com/a")
		checkWrapped(t, key, get(fieldAct), "https://field.com/f")
	})
	t.Run("sanitize then rewrite", func(t *testing.T) {
		ctx, next, aaAct, fieldAct, _ := build(t)
		if err := sanitizePDF(ctx, &Report{}); err != nil {
			t.Fatal(err)
		}
		opts, key := wrapOpts()
		if err := rewritePDFLinks(ctx, opts, &Report{}); err != nil {
			t.Fatal(err)
		}
		checkWrapped(t, key, get(next), "https://next.com/n")
		checkWrapped(t, key, get(aaAct), "https://aa.com/a")
		checkWrapped(t, key, get(fieldAct), "https://field.com/f")
	})
}

func TestRewritePDFLinksSharedActionRewrittenOnce(t *testing.T) {
	ctx := readLinkFixture(t)
	page, _, _, _ := ctx.PageDict(1, false)
	shared := uriAct("https://shared.com/s")
	widget := types.Dict{"Subtype": types.Name("Widget"), "A": shared}
	root, _ := ctx.Catalog()
	root["AcroForm"] = types.Dict{"Fields": types.Array{widget}}
	orig, _ := page.Find("Annots")
	page["Annots"] = append(orig.(types.Array), widget)
	opts, key := wrapOpts()
	if err := rewritePDFLinks(ctx, opts, &Report{}); err != nil {
		t.Fatal(err)
	}
	checkWrapped(t, key, uriString(shared), "https://shared.com/s")
}

func TestRewritePDFLinksRelativeBaseNeutralizes(t *testing.T) {
	ctx := readLinkFixture(t)
	setFixtureURI(t, ctx, types.StringLiteral("https://x.com/a"))
	opts, _ := wrapOpts()
	opts.WrapBase = "/user/optimizer/r/"
	if err := rewritePDFLinks(ctx, opts, &Report{}); err != nil {
		t.Fatal(err)
	}
	got := pdfURIs(ctx)
	if len(got) != 1 || got[0] != "hxxps://x.com/a" {
		t.Fatalf("want neutralized, got %v", got)
	}
}
