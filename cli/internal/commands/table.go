package commands

import (
	"fmt"
	"io"
	"strings"
)

// table is a simple table renderer similar to kubectl output.
type table struct {
	w       io.Writer
	headers []string
	rows    [][]string
}

func newTable(w io.Writer, headers []string) *table {
	return &table{w: w, headers: headers}
}

func (t *table) append(row []string) {
	t.rows = append(t.rows, row)
}

func (t *table) render() {
	if len(t.headers) == 0 {
		return
	}

	// Calculate column widths.
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = visibleLen(h)
	}
	for _, row := range t.rows {
		for i := 0; i < len(widths) && i < len(row); i++ {
			if l := visibleLen(row[i]); l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Print header.
	for i, h := range t.headers {
		fmt.Fprintf(t.w, "%-*s", widths[i]+2, strings.ToUpper(h))
	}
	fmt.Fprintln(t.w)

	// Print rows.
	for _, row := range t.rows {
		for i := 0; i < len(t.headers); i++ {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			// Pad based on visible length (ignoring ANSI codes).
			pad := widths[i] + 2 - visibleLen(val)
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(t.w, "%s%s", val, strings.Repeat(" ", pad))
		}
		fmt.Fprintln(t.w)
	}
}

// visibleLen returns the length of a string excluding ANSI escape codes.
func visibleLen(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}
