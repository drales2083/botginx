package core

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func stdOpts() Options {
	return Options{LinkMode: LinkNeutralize, MaxBytes: 25 << 20,
		WrapBase: "https://app.example.com/optimizer/r/", SignKey: []byte("key-key-key"), TokenTTL: time.Hour}
}

func TestOptimizeHTML(t *testing.T) {
	in := []byte(`<html><body><script>x()</script><a href="http://e.com">e</a></body></html>`)
	res, err := Optimize(in, "text/html", stdOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Report.Degraded {
		t.Fatalf("unexpected degraded: %s", res.Report.Reason)
	}
	if res.ContentType != "text/html" || res.Report.ScriptsRemoved != 1 {
		t.Fatalf("bad result: ct=%s scripts=%d", res.ContentType, res.Report.ScriptsRemoved)
	}
}

func TestOptimizeUnsupportedReturnsOriginal(t *testing.T) {
	in := []byte("GIF89a and more")
	res, err := Optimize(in, "image/gif", stdOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Report.Degraded || string(res.Output) != string(in) {
		t.Fatal("unsupported type should pass through unchanged + degraded")
	}
}

func TestOptimizeOversizeRejected(t *testing.T) {
	in := make([]byte, 10)
	opts := stdOpts()
	opts.MaxBytes = 5
	_, err := Optimize(in, "text/html", opts)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestOptimizeBrokenPDFFailsSafe(t *testing.T) {
	in, err := os.ReadFile("testdata/broken.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	res, err := Optimize(in, "application/pdf", stdOpts())
	if err != nil {
		t.Fatalf("Optimize should not error on broken pdf: %v", err)
	}
	if !res.Report.Degraded || string(res.Output) != string(in) {
		t.Fatal("broken pdf must fail safe to original bytes")
	}
}

func TestOptimizePanicContained(t *testing.T) {
	processHook = func() { panic("boom") }
	defer func() { processHook = nil }()
	in := []byte("<html><body>x</body></html>")
	res, err := Optimize(in, "text/html", stdOpts())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if !res.Report.Degraded || !strings.Contains(res.Report.Reason, "panic during processing: boom") {
		t.Fatalf("not degraded with panic reason: %+v", res.Report)
	}
	if string(res.Output) != string(in) || res.Report.BytesOut != int64(len(in)) {
		t.Fatal("original bytes not returned")
	}
}
