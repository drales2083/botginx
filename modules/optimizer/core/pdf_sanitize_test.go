package core

import (
	"os"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func plainCtx(t *testing.T) (*model.Context, types.Dict, types.Dict) {
	t.Helper()
	in, err := os.ReadFile("testdata/plain.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	root, err := ctx.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	page, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, root, page
}

func linkAnnot(action types.Dict) types.Dict {
	return types.Dict{"Type": types.Name("Annot"), "Subtype": types.Name("Link"), "A": action}
}

func pageAnnots(t *testing.T, page types.Dict) types.Array {
	t.Helper()
	a, ok := page.Find("Annots")
	if !ok {
		return nil
	}
	return a.(types.Array)
}

func TestSanitizePDFAnnotationActions(t *testing.T) {
	unsafe := []string{"JavaScript", "Launch", "SubmitForm", "ImportData", "GoToR", "GoToE", "Named", "Rendition", "Movie", "SetOCGState", "Trans"}
	for _, s := range unsafe {
		t.Run(s, func(t *testing.T) {
			ctx, _, page := plainCtx(t)
			annot := linkAnnot(types.Dict{"S": types.Name(s)})
			page["Annots"] = types.Array{annot}
			var rep Report
			if err := sanitizePDF(ctx, &rep); err != nil {
				t.Fatal(err)
			}
			if _, ok := annot.Find("A"); ok {
				t.Errorf("/%s action not removed", s)
			}
			if rep.PDFJSRemoved != 1 {
				t.Errorf("PDFJSRemoved = %d, want 1", rep.PDFJSRemoved)
			}
		})
	}
}

func TestSanitizePDFPreservesURIAndGoTo(t *testing.T) {
	ctx, _, page := plainCtx(t)
	uri := linkAnnot(types.Dict{"S": types.Name("URI"), "URI": types.StringLiteral("https://a.com")})
	goTo := linkAnnot(types.Dict{"S": types.Name("GoTo"), "D": types.Array{}})
	page["Annots"] = types.Array{uri, goTo}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if _, ok := uri.Find("A"); !ok {
		t.Error("URI action removed")
	}
	if _, ok := goTo.Find("A"); !ok {
		t.Error("GoTo action removed")
	}
	if rep.PDFJSRemoved != 0 {
		t.Errorf("PDFJSRemoved = %d, want 0", rep.PDFJSRemoved)
	}
}

func TestSanitizePDFNextChainToJS(t *testing.T) {
	ctx, _, page := plainCtx(t)
	annot := linkAnnot(types.Dict{
		"S": types.Name("GoTo"), "D": types.Array{},
		"Next": types.Dict{"S": types.Name("JavaScript"), "JS": types.StringLiteral("x")},
	})
	page["Annots"] = types.Array{annot}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if _, ok := annot.Find("A"); ok {
		t.Error("action chaining to JavaScript not removed")
	}
}

func TestSanitizePDFAnnotationAA(t *testing.T) {
	ctx, _, page := plainCtx(t)
	aa := types.Dict{
		"E": types.Dict{"S": types.Name("JavaScript"), "JS": types.StringLiteral("x")},
		"X": types.Dict{"S": types.Name("URI"), "URI": types.StringLiteral("https://a.com")},
	}
	annot := types.Dict{"Subtype": types.Name("Widget"), "AA": aa}
	page["Annots"] = types.Array{annot}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if _, ok := aa.Find("E"); ok {
		t.Error("JS entry in annotation /AA not removed")
	}
	if _, ok := aa.Find("X"); !ok {
		t.Error("URI entry in annotation /AA removed")
	}
}

func TestSanitizePDFXFAAndFieldAA(t *testing.T) {
	ctx, root, _ := plainCtx(t)
	field := types.Dict{"FT": types.Name("Tx"), "AA": types.Dict{"K": types.Dict{"S": types.Name("JavaScript")}}}
	kid := types.Dict{"AA": types.Dict{}}
	field["Kids"] = types.Array{kid}
	form := types.Dict{"XFA": types.Array{}, "Fields": types.Array{field}}
	root["AcroForm"] = form
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if _, ok := form.Find("XFA"); ok {
		t.Error("XFA not removed")
	}
	if _, ok := field.Find("AA"); ok {
		t.Error("field /AA not removed")
	}
	if _, ok := kid.Find("AA"); ok {
		t.Error("kid field /AA not removed")
	}
	if rep.PDFJSRemoved != 3 {
		t.Errorf("PDFJSRemoved = %d, want 3", rep.PDFJSRemoved)
	}
}

func TestSanitizePDFFileAttachmentAndAF(t *testing.T) {
	ctx, root, page := plainCtx(t)
	att := types.Dict{"Subtype": types.Name("FileAttachment")}
	link := linkAnnot(types.Dict{"S": types.Name("URI"), "URI": types.StringLiteral("https://a.com")})
	page["Annots"] = types.Array{att, link}
	root["AF"] = types.Array{}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.PDFEmbeddedRemoved != 2 {
		t.Errorf("PDFEmbeddedRemoved = %d, want 2", rep.PDFEmbeddedRemoved)
	}
	if _, ok := root.Find("AF"); ok {
		t.Error("/AF not removed")
	}
	annots := pageAnnots(t, page)
	if len(annots) != 1 {
		t.Fatalf("annots = %d, want 1 (link kept)", len(annots))
	}
}

func TestSanitizePDFPropagatesDerefError(t *testing.T) {
	ctx, _, page := plainCtx(t)
	// An annotation entry that is not a dictionary cannot be inspected.
	page["Annots"] = types.Array{types.Name("bogus")}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err == nil {
		t.Fatal("expected error for uninspectable annotation, got nil")
	}
	ctx, root, _ := plainCtx(t)
	root["Names"] = types.Name("bogus")
	if err := sanitizePDF(ctx, &rep); err == nil {
		t.Fatal("expected error for uninspectable /Names, got nil")
	}
}

func TestSanitizePDFPageAF(t *testing.T) {
	ctx, _, page := plainCtx(t)
	page["AF"] = types.Array{}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if _, ok := page.Find("AF"); ok {
		t.Error("page /AF not removed")
	}
	if rep.PDFEmbeddedRemoved != 1 {
		t.Errorf("PDFEmbeddedRemoved = %d, want 1", rep.PDFEmbeddedRemoved)
	}
}

func TestSanitizePDFDropsMediaAnnots(t *testing.T) {
	for _, st := range []string{"Screen", "RichMedia", "3D", "Sound", "Movie"} {
		t.Run(st, func(t *testing.T) {
			ctx, _, page := plainCtx(t)
			keep := types.Dict{"Subtype": types.Name("Text")}
			page["Annots"] = types.Array{types.Dict{"Subtype": types.Name(st)}, keep}
			var rep Report
			if err := sanitizePDF(ctx, &rep); err != nil {
				t.Fatal(err)
			}
			if a := pageAnnots(t, page); len(a) != 1 {
				t.Fatalf("annots = %d, want 1", len(a))
			}
			if rep.PDFEmbeddedRemoved != 1 {
				t.Errorf("PDFEmbeddedRemoved = %d, want 1", rep.PDFEmbeddedRemoved)
			}
		})
	}
}
