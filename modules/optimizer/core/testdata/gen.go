//go:build ignore

// Run with: cd core && go run testdata/gen.go
// Produces testdata/*.pdf used by pdf_test.go.
package main

import (
	"bytes"
	"fmt"
	"os"
)

// minimalPDF builds a one-page A4 PDF with a correct xref table.
func minimalPDF() []byte {
	return buildPDF("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>")
}

// linkPDF builds a one-page PDF whose page has a single URI link annotation.
func linkPDF(uri string) []byte {
	return buildPDF(
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Annots [4 0 R] >>",
		"<< /Type /Annot /Subtype /Link /Rect [100 100 200 120] /Border [0 0 0] /A << /S /URI /URI ("+uri+") >> >>",
	)
}

// buildPDF assembles catalog, page tree, the page (object 3) and any extra
// objects (4, 5, ...) into a PDF with a correct xref table.
func buildPDF(page string, extra ...string) []byte {
	objs := append([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		page,
	}, extra...)
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offs {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}

func main() {
	// A minimal valid single-page PDF (hand-written; classic xref table).
	if err := os.WriteFile("testdata/plain.pdf", minimalPDF(), 0o644); err != nil {
		panic(err)
	}
	// A one-page PDF with one URI link annotation.
	if err := os.WriteFile("testdata/link.pdf", linkPDF("https://real.com/x"), 0o644); err != nil {
		panic(err)
	}
	// A deliberately broken file to exercise fail-safe.
	if err := os.WriteFile("testdata/broken.pdf", []byte("%PDF-1.7\nnot a real pdf"), 0o644); err != nil {
		panic(err)
	}
}
