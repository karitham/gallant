package rules

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterDelimiter = "---"

// globList accepts either a single glob string or a list of them.
type globList []string

func (g *globList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		if s != "" {
			*g = globList{s}
		}
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		*g = items
		return nil
	default:
		return fmt.Errorf("appliesTo: expected a string or a list of strings")
	}
}

// splitFrontmatter separates an optional frontmatter block from the markdown
// body. The boolean result reports whether a frontmatter block was found.
//
// A block is only recognised when the very first line is exactly the delimiter.
// It ends at the next line that is exactly the delimiter (ignoring a trailing
// "\r"). The body is everything after that closing delimiter, preserved
// verbatim. If the delimiter is missing or never closed, the whole input is
// returned as the body.
func splitFrontmatter(src string) (fm, body string, ok bool) {
	line, next := readLine(src, 0)
	if strings.TrimRight(line, "\r") != frontmatterDelimiter {
		return "", src, false
	}

	contentStart := next
	for pos := next; ; {
		if pos >= len(src) {
			return "", src, false
		}
		line, next = readLine(src, pos)
		if strings.TrimRight(line, "\r") == frontmatterDelimiter {
			return src[contentStart:pos], src[next:], true
		}
		pos = next
	}
}

// readLine returns the line beginning at pos (excluding its trailing "\n") and
// the index of the first byte of the following line. A missing trailing newline
// yields next == len(s).
func readLine(s string, pos int) (line string, next int) {
	i := strings.IndexByte(s[pos:], '\n')
	if i < 0 {
		return s[pos:], len(s)
	}
	return s[pos : pos+i], pos + i + 1
}
