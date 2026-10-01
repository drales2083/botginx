package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func pdfConf() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}

// readPDF parses PDF bytes into a pdfcpu context.
func readPDF(in []byte) (*model.Context, error) {
	ctx, err := api.ReadContext(bytes.NewReader(in), pdfConf())
	if err != nil {
		return nil, err
	}
	// ReadContext does not validate, so PageCount is unset; populate it so
	// page-level sanitizing and optimization actually run.
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, err
	}
	return ctx, nil
}

// writePDF optimizes and serializes a context back to bytes.
func writePDF(ctx *model.Context) ([]byte, error) {
	if err := api.OptimizeContext(ctx); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := api.WriteContext(ctx, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sanitizePDF removes JavaScript, active actions and embedded files. Any
// dereference error on a structure that is present is returned, so callers can
// fall back to the original bytes instead of claiming a clean result.
func sanitizePDF(ctx *model.Context, rep *Report) error {
	root, err := ctx.Catalog()
	if err != nil {
		return err
	}
	// Document-level JavaScript entry points and associated files.
	if _, found := root.Find("OpenAction"); found {
		root.Delete("OpenAction")
		rep.PDFJSRemoved++
	}
	if _, found := root.Find("AA"); found {
		root.Delete("AA")
		rep.PDFJSRemoved++
	}
	if _, found := root.Find("AF"); found {
		root.Delete("AF")
		rep.PDFEmbeddedRemoved++
	}
	// Names tree: /JavaScript and /EmbeddedFiles.
	if namesObj, found := root.Find("Names"); found {
		names, err := ctx.DereferenceDict(namesObj)
		if err != nil {
			return fmt.Errorf("pdf: names dict: %w", err)
		}
		if names != nil {
			if _, ok := names.Find("JavaScript"); ok {
				names.Delete("JavaScript")
				rep.PDFJSRemoved++
			}
			if _, ok := names.Find("EmbeddedFiles"); ok {
				names.Delete("EmbeddedFiles")
				rep.PDFEmbeddedRemoved++
			}
		}
	}
	if err := sanitizeAcroForm(ctx, root, rep); err != nil {
		return err
	}
	for i := 1; i <= ctx.PageCount; i++ {
		d, _, _, err := ctx.PageDict(i, false)
		if err != nil {
			return fmt.Errorf("pdf: page %d: %w", i, err)
		}
		if d == nil {
			return fmt.Errorf("pdf: page %d: missing dict", i)
		}
		if _, ok := d.Find("AA"); ok {
			d.Delete("AA")
			rep.PDFJSRemoved++
		}
		if _, ok := d.Find("AF"); ok {
			d.Delete("AF")
			rep.PDFEmbeddedRemoved++
		}
		if err := sanitizeAnnots(ctx, d, rep); err != nil {
			return fmt.Errorf("pdf: page %d annots: %w", i, err)
		}
	}
	return nil
}

// sanitizeAcroForm drops XFA and field-level additional actions.
func sanitizeAcroForm(ctx *model.Context, root types.Dict, rep *Report) error {
	obj, found := root.Find("AcroForm")
	if !found {
		return nil
	}
	form, err := ctx.DereferenceDict(obj)
	if err != nil {
		return fmt.Errorf("pdf: acroform: %w", err)
	}
	if form == nil {
		return nil
	}
	if _, ok := form.Find("XFA"); ok {
		form.Delete("XFA")
		rep.PDFJSRemoved++
	}
	fields, found := form.Find("Fields")
	if !found {
		return nil
	}
	return sanitizeFields(ctx, fields, rep, 0, map[int]bool{})
}

const maxFieldDepth = 64

func sanitizeFields(ctx *model.Context, obj types.Object, rep *Report, depth int, seen map[int]bool) error {
	if depth > maxFieldDepth {
		return errors.New("pdf: form field tree too deep")
	}
	arr, err := ctx.DereferenceArray(obj)
	if err != nil {
		return fmt.Errorf("pdf: form fields: %w", err)
	}
	for _, f := range arr {
		if ref, ok := f.(types.IndirectRef); ok {
			n := ref.ObjectNumber.Value()
			if seen[n] {
				continue
			}
			seen[n] = true
		}
		field, err := ctx.DereferenceDict(f)
		if err != nil {
			return fmt.Errorf("pdf: form field: %w", err)
		}
		if field == nil {
			continue
		}
		if _, ok := field.Find("AA"); ok {
			field.Delete("AA")
			rep.PDFJSRemoved++
		}
		if _, ok := field.Find("A"); ok {
			if err := sanitizeAction(ctx, field, "A", rep); err != nil {
				return err
			}
		}
		if kids, ok := field.Find("Kids"); ok {
			if err := sanitizeFields(ctx, kids, rep, depth+1, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// dropAnnotSubtypes are annotation subtypes removed outright: they embed files
// or media/scripts through keys other than actions. Counted as PDFEmbeddedRemoved.
var dropAnnotSubtypes = map[string]bool{
	"FileAttachment": true, "Screen": true, "RichMedia": true,
	"3D": true, "Sound": true, "Movie": true,
}

// sanitizeAnnots removes FileAttachment annotations and unsafe actions from
// the remaining annotations of a page.
func sanitizeAnnots(ctx *model.Context, page types.Dict, rep *Report) error {
	obj, found := page.Find("Annots")
	if !found {
		return nil
	}
	annots, err := ctx.DereferenceArray(obj)
	if err != nil {
		return err
	}
	kept := make(types.Array, 0, len(annots))
	removed := false
	for _, a := range annots {
		annot, err := ctx.DereferenceDict(a)
		if err != nil {
			return err
		}
		if annot == nil {
			kept = append(kept, a)
			continue
		}
		if st := annot.NameEntry("Subtype"); st != nil && dropAnnotSubtypes[*st] {
			rep.PDFEmbeddedRemoved++
			removed = true
			continue
		}
		kept = append(kept, a)
		if _, ok := annot.Find("A"); ok {
			if err := sanitizeAction(ctx, annot, "A", rep); err != nil {
				return err
			}
		}
		if aaObj, ok := annot.Find("AA"); ok {
			aa, err := ctx.DereferenceDict(aaObj)
			if err != nil {
				return err
			}
			for k := range aa {
				if err := sanitizeAction(ctx, aa, k, rep); err != nil {
					return err
				}
			}
		}
	}
	if removed {
		page["Annots"] = kept
	}
	return nil
}

// sanitizeAction deletes d[key] when it is an action other than /URI or an
// internal /GoTo (or when it chains to one via /Next). Unreadable or
// unrecognizable actions are removed (fail closed); dereference errors propagate.
func sanitizeAction(ctx *model.Context, d types.Dict, key string, rep *Report) error {
	obj, ok := d.Find(key)
	if !ok {
		return nil
	}
	safe, err := actionSafe(ctx, obj, 0)
	if err != nil {
		return err
	}
	if !safe {
		d.Delete(key)
		rep.PDFJSRemoved++
	}
	return nil
}

// actionSafe reports whether an action (and its /Next chain) only uses /URI or
// /GoTo.
func actionSafe(ctx *model.Context, obj types.Object, depth int) (bool, error) {
	if depth > maxFieldDepth {
		return false, nil
	}
	act, err := ctx.DereferenceDict(obj)
	if err != nil {
		return false, fmt.Errorf("pdf: action: %w", err)
	}
	if act == nil {
		return true, nil
	}
	sObj, ok := act.Find("S")
	if !ok {
		return false, nil
	}
	sv, err := ctx.Dereference(sObj)
	if err != nil {
		return false, fmt.Errorf("pdf: action type: %w", err)
	}
	name, ok := sv.(types.Name)
	if !ok || (name != "URI" && name != "GoTo") {
		return false, nil
	}
	next, ok := act.Find("Next")
	if !ok {
		return true, nil
	}
	nv, err := ctx.Dereference(next)
	if err != nil {
		return false, fmt.Errorf("pdf: action next: %w", err)
	}
	if arr, isArr := nv.(types.Array); isArr {
		for _, n := range arr {
			s, err := actionSafe(ctx, n, depth+1)
			if err != nil || !s {
				return false, err
			}
		}
		return true, nil
	}
	return actionSafe(ctx, next, depth+1)
}

// rewritePDFLinks walks page annotations and rewrites URI link actions.
func rewritePDFLinks(ctx *model.Context, opts Options, rep *Report) error {
	return eachURIAction(ctx, func(action types.Dict) {
		raw := uriString(action)
		if raw == "" {
			return
		}
		rep.LinksHandled++
		repl := NeutralizeURL(raw)
		if opts.LinkMode == LinkWrap && IsHTTPURL(raw) && IsHTTPURL(opts.WrapBase) {
			if tok, err := SignToken(opts.SignKey, raw, opts.TokenTTL); err == nil {
				repl = WrapURL(opts.WrapBase, tok)
			}
		}
		action["URI"] = types.StringLiteral(escapePDFString(repl))
	})
}

// pdfURIs returns all URI strings from page link annotations.
func pdfURIs(ctx *model.Context) []string {
	var out []string
	_ = eachURIAction(ctx, func(action types.Dict) {
		if u := uriString(action); u != "" {
			out = append(out, u)
		}
	})
	return out
}

// eachURIAction calls fn once with every action dict that has a /URI, wherever
// it can appear: annotation /A, annotation /AA entries, /Next chains, and
// form-field /A and /AA (recursing through /Kids). Shared dicts are visited once.
func eachURIAction(ctx *model.Context, fn func(action types.Dict)) error {
	return walkActions(ctx, func(act types.Dict) {
		if _, ok := act.Find("URI"); ok {
			fn(act)
		}
	})
}

// walkActions visits every action dict reachable from page annotations and
// AcroForm fields. Dereference errors on present structures are returned.
func walkActions(ctx *model.Context, fn func(action types.Dict)) error {
	seen := map[uintptr]bool{}
	first := func(d types.Dict) bool {
		p := reflect.ValueOf(d).Pointer()
		if seen[p] {
			return false
		}
		seen[p] = true
		return true
	}
	var visitAction func(obj types.Object, depth int) error
	visitAction = func(obj types.Object, depth int) error {
		if depth > maxFieldDepth {
			return errors.New("pdf: action chain too deep")
		}
		act, err := ctx.DereferenceDict(obj)
		if err != nil {
			return fmt.Errorf("pdf: action: %w", err)
		}
		if act == nil || !first(act) {
			return nil
		}
		fn(act)
		next, ok := act.Find("Next")
		if !ok {
			return nil
		}
		nv, err := ctx.Dereference(next)
		if err != nil {
			return fmt.Errorf("pdf: action next: %w", err)
		}
		if arr, isArr := nv.(types.Array); isArr {
			for _, n := range arr {
				if err := visitAction(n, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		return visitAction(next, depth+1)
	}
	// visitHolder visits d's /A and every /AA entry.
	visitHolder := func(d types.Dict) error {
		if a, ok := d.Find("A"); ok {
			if err := visitAction(a, 0); err != nil {
				return err
			}
		}
		if aaObj, ok := d.Find("AA"); ok {
			aa, err := ctx.DereferenceDict(aaObj)
			if err != nil {
				return fmt.Errorf("pdf: additional actions: %w", err)
			}
			for _, a := range aa {
				if err := visitAction(a, 0); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var visitFields func(obj types.Object, depth int) error
	visitFields = func(obj types.Object, depth int) error {
		if depth > maxFieldDepth {
			return errors.New("pdf: form field tree too deep")
		}
		arr, err := ctx.DereferenceArray(obj)
		if err != nil {
			return fmt.Errorf("pdf: form fields: %w", err)
		}
		for _, f := range arr {
			field, err := ctx.DereferenceDict(f)
			if err != nil {
				return fmt.Errorf("pdf: form field: %w", err)
			}
			if field == nil || !first(field) {
				continue
			}
			if err := visitHolder(field); err != nil {
				return err
			}
			if kids, ok := field.Find("Kids"); ok {
				if err := visitFields(kids, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for i := 1; i <= ctx.PageCount; i++ {
		d, _, _, err := ctx.PageDict(i, false)
		if err != nil {
			return fmt.Errorf("pdf: page %d: %w", i, err)
		}
		if d == nil {
			continue
		}
		obj, ok := d.Find("Annots")
		if !ok {
			continue
		}
		annots, err := ctx.DereferenceArray(obj)
		if err != nil {
			return fmt.Errorf("pdf: page %d annots: %w", i, err)
		}
		for _, a := range annots {
			annot, err := ctx.DereferenceDict(a)
			if err != nil {
				return fmt.Errorf("pdf: annotation: %w", err)
			}
			if annot == nil {
				continue
			}
			if err := visitHolder(annot); err != nil {
				return err
			}
		}
	}
	root, err := ctx.Catalog()
	if err != nil {
		return err
	}
	if obj, ok := root.Find("AcroForm"); ok {
		form, err := ctx.DereferenceDict(obj)
		if err != nil {
			return fmt.Errorf("pdf: acroform: %w", err)
		}
		if fields, ok := form.Find("Fields"); form != nil && ok {
			return visitFields(fields, 0)
		}
	}
	return nil
}

func uriString(action types.Dict) string {
	v, ok := action.Find("URI")
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case types.StringLiteral:
		// Value() keeps PDF escapes (\( \) \\ \ddd); decode to the real URL.
		b, err := types.Unescape(s.Value())
		if err != nil {
			return ""
		}
		return string(b)
	case types.HexLiteral:
		// Value() is the hex digits; decode to the real bytes.
		b, err := hex.DecodeString(s.Value())
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return ""
	}
}

// escapePDFString escapes characters that would terminate or corrupt a PDF
// literal string, since pdfcpu serializes StringLiteral values verbatim.
func escapePDFString(s string) string {
	return strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(s)
}
