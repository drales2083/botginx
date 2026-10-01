package core

import "testing"

func TestMarkDegraded(t *testing.T) {
	var r Report
	r.MarkDegraded("boom")
	if !r.Degraded || r.Reason != "boom" {
		t.Fatalf("got degraded=%v reason=%q", r.Degraded, r.Reason)
	}
}
