package formatter

import (
	"bytes"
	"strings"
	"testing"

	"yndns/internal/enricher"
)

func TestPrintResultsFillsMissingFields(t *testing.T) {
	var output bytes.Buffer
	PrintResults(&output, []*enricher.Result{{IP: "192.0.2.1"}})

	fields := strings.Fields(output.String())
	expected := []string{"192.0.2.1", "-", "-", "-", "-"}
	if len(fields) != len(expected) {
		t.Fatalf("PrintResults output fields = %v, want %v", fields, expected)
	}
	for i := range expected {
		if fields[i] != expected[i] {
			t.Fatalf("PrintResults output fields = %v, want %v", fields, expected)
		}
	}
}
