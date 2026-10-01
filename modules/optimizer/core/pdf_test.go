package core

import (
	"os"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestReadWritePDFRoundTrip(t *testing.T) {
	in, err := os.ReadFile("testdata/plain.pdf")
	if err != nil {
		t.Skip("fixture missing; run go run testdata/gen.go")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatalf("readPDF: %v", err)
	}
	out, err := writePDF(ctx)
	if err != nil {
		t.Fatalf("writePDF: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
	// Output must re-parse.
	if _, err := readPDF(out); err != nil {
		t.Fatalf("output not re-parseable: %v", err)
	}
}

func TestSanitizePDFNoPanicOnPlain(t *testing.T) {
	in, err := os.ReadFile("testdata/plain.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatalf("sanitizePDF: %v", err)
	}
	// plain.pdf has no JS/embedded; counts stay zero, no error.
	if rep.PDFJSRemoved != 0 || rep.PDFEmbeddedRemoved != 0 {
		t.Errorf("unexpected removals: js=%d emb=%d", rep.PDFJSRemoved, rep.PDFEmbeddedRemoved)
	}
}

func TestSanitizePDFRemovesJSAndEmbedded(t *testing.T) {
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
	root["OpenAction"] = types.Dict{"S": types.Name("JavaScript"), "JS": types.StringLiteral("app.alert(1)")}
	root["Names"] = types.Dict{
		"JavaScript":    types.Dict{},
		"EmbeddedFiles": types.Dict{},
	}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.PDFJSRemoved != 2 || rep.PDFEmbeddedRemoved != 1 {
		t.Errorf("js=%d emb=%d, want 2/1", rep.PDFJSRemoved, rep.PDFEmbeddedRemoved)
	}
	if _, ok := root.Find("OpenAction"); ok {
		t.Error("OpenAction still present")
	}
}

func TestReadPDFBrokenFails(t *testing.T) {
	in, err := os.ReadFile("testdata/broken.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	if _, err := readPDF(in); err == nil {
		t.Fatal("expected error for broken pdf")
	}
}

func TestSanitizePDFRemovesPageAA(t *testing.T) {
	in, err := os.ReadFile("testdata/plain.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	ctx, err := readPDF(in)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PageCount < 1 {
		t.Fatalf("PageCount = %d, want >= 1", ctx.PageCount)
	}
	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatal(err)
	}
	pageDict["AA"] = types.Dict{"O": types.Dict{"S": types.Name("JavaScript"), "JS": types.StringLiteral("app.alert(1)")}}
	var rep Report
	if err := sanitizePDF(ctx, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.PDFJSRemoved != 1 {
		t.Errorf("PDFJSRemoved = %d, want 1", rep.PDFJSRemoved)
	}
	if _, ok := pageDict.Find("AA"); ok {
		t.Error("page /AA still present")
	}
}
