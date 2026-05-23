package formatter

import (
	"fmt"
	"io"
	"text/tabwriter"

	"yndns/internal/enricher"
)

// PrintResults writes enriched results as a tab-aligned table.
func PrintResults(w io.Writer, results []*enricher.Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			dashIfEmpty(r.IP),
			dashIfEmpty(r.ASN),
			dashIfEmpty(r.ASName),
			dashIfEmpty(r.ASDomain),
			dashIfEmpty(r.Country),
		)
	}
	tw.Flush()
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
