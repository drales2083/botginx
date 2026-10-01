package core

// Report summarizes what Optimize did to a document.
type Report struct {
	DetectedType         string
	BytesIn              int64
	BytesOut             int64
	TrackersRemoved      int
	ScriptsRemoved       int
	ActiveContentRemoved int
	LinksHandled         int
	PDFJSRemoved         int
	PDFEmbeddedRemoved   int
	Degraded             bool
	Reason               string
}

// MarkDegraded records that processing fell back to the original bytes.
func (r *Report) MarkDegraded(reason string) {
	r.Degraded = true
	r.Reason = reason
}

// Result is the output of Optimize.
type Result struct {
	Output      []byte
	ContentType string
	Report      Report
}
