package main

import (
	"fmt"
	"io"
	"strings"
)

//go:generate sh -c "go run . --man > ../../man/gallant.1"

// printMan writes a roff man page built from the embedded help texts, so the
// manual and the help cannot drift.
func printMan(w io.Writer) {
	fmt.Fprint(w, ".TH GALLANT 1\n.SH NAME\n")
	fmt.Fprint(w, `gallant \- check files against rules with a decision model`+"\n")
	for _, s := range []struct{ title, body string }{
		{"USAGE", usageText},
		{"RULES", rulesHelpText},
		{"PROVIDERS", providersHelpText},
	} {
		fmt.Fprintf(w, ".SH %s\n.nf\n", s.title)
		io.WriteString(w, roffEscape(s.body))
		fmt.Fprint(w, ".fi\n")
	}
}

// roffEscape makes text safe inside a no-fill block. A backslash is an escape
// in roff, and a line beginning with "." or "'" would be read as a request.
func roffEscape(s string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		line = strings.ReplaceAll(line, `\`, `\\`)
		if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "'") {
			line = `\&` + line
		}
		b.WriteString(line)
	}
	return b.String()
}
