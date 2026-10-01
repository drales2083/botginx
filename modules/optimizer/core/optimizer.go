package core

import (
	"errors"
	"fmt"
)

// ErrTooLarge is returned when input exceeds Options.MaxBytes.
var ErrTooLarge = errors.New("input exceeds max size")

// processHook is a test seam invoked at the start of processing; nil in production.
var processHook func()

// Optimize cleans an HTML or PDF document. On any processing failure it returns
// the original bytes with Report.Degraded set; it only returns a non-nil error
// for input that is too large (a caller-side precondition).
func Optimize(in []byte, declaredType string, opts Options) (res Result, err error) {
	rep := Report{BytesIn: int64(len(in))}
	if opts.MaxBytes > 0 && int64(len(in)) > opts.MaxBytes {
		return Result{}, fmt.Errorf("%w: %d > %d", ErrTooLarge, len(in), opts.MaxBytes)
	}

	// Contain panics from parsers handling untrusted input: fail safe with the
	// original bytes rather than letting the panic escape.
	defer func() {
		if r := recover(); r != nil {
			rep.MarkDegraded(fmt.Sprintf("panic during processing: %v", r))
			rep.BytesOut = int64(len(in))
			res = Result{Output: in, ContentType: contentTypeFor(rep.DetectedType), Report: rep}
			err = nil
		}
	}()
	t := DetectType(in, declaredType)
	rep.DetectedType = t

	if processHook != nil {
		processHook()
	}

	fail := func(reason string) (Result, error) {
		rep.MarkDegraded(reason)
		rep.BytesOut = int64(len(in))
		return Result{Output: in, ContentType: contentTypeFor(t), Report: rep}, nil
	}

	switch t {
	case "text/html":
		out, err := processHTML(in, opts, &rep)
		if err != nil {
			return fail("html processing failed: " + err.Error())
		}
		rep.BytesOut = int64(len(out))
		return Result{Output: out, ContentType: "text/html", Report: rep}, nil

	case "application/pdf":
		ctx, err := readPDF(in)
		if err != nil {
			return fail("pdf parse failed: " + err.Error())
		}
		if err := sanitizePDF(ctx, &rep); err != nil {
			return fail("pdf sanitize failed: " + err.Error())
		}
		if err := rewritePDFLinks(ctx, opts, &rep); err != nil {
			return fail("pdf link rewrite failed: " + err.Error())
		}
		out, err := writePDF(ctx)
		if err != nil {
			return fail("pdf write failed: " + err.Error())
		}
		rep.BytesOut = int64(len(out))
		return Result{Output: out, ContentType: "application/pdf", Report: rep}, nil

	default:
		return fail("unsupported type")
	}
}

func contentTypeFor(t string) string {
	if t == "" {
		return "application/octet-stream"
	}
	return t
}
