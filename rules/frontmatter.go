package rules

import (
	"errors"
	"strings"
)

const frontmatterDelimiter = "---"

var (
	// errNoFrontmatter reports a rule that does not begin with a frontmatter
	// block at all.
	errNoFrontmatter = errors.New("missing frontmatter: a rule must begin with a --- block")
	// errUnclosedFrontmatter reports an opening "---" line with no matching
	// closing line, so a rule author can tell it apart from a missing block.
	errUnclosedFrontmatter = errors.New("unclosed frontmatter: no closing ---")
)

// globList accepts either a single glob string or a list of them.
type globList []string

func (g *globList) UnmarshalYAML(unmarshal func(any) error) error {
	var patterns []string
	if err := unmarshal(&patterns); err == nil {
		*g = patterns
		return nil
	}
	var pattern string
	if err := unmarshal(&pattern); err == nil {
		if pattern != "" {
			*g = globList{pattern}
		}
		return nil
	}
	return errors.New("appliesTo: expected a string or a list of strings")
}

// Criteria is the set of options or levels a rule's question is answered from:
// a mapping of option to description for a choice, or an ordered list of levels
// for a score.
type Criteria struct {
	Options map[string]string
	Levels  []string
}

// splitFrontmatter separates a frontmatter block from the markdown body. A block
// is only recognised when the very first line is exactly the delimiter; it ends
// at the next line that is exactly the delimiter (ignoring a trailing "\r").
// The body is everything after that closing delimiter, preserved verbatim.
//
// It reports errNoFrontmatter when the input does not begin with the delimiter
// and errUnclosedFrontmatter when the opening delimiter is never closed, so a
// rule author can tell the two mistakes apart.
func splitFrontmatter(src string) (fm, body string, err error) {
	line, next := readLine(src, 0)
	if strings.TrimRight(line, "\r") != frontmatterDelimiter {
		return "", src, errNoFrontmatter
	}

	contentStart := next
	for pos := next; ; {
		if pos >= len(src) {
			return "", src, errUnclosedFrontmatter
		}
		line, next = readLine(src, pos)
		if strings.TrimRight(line, "\r") == frontmatterDelimiter {
			return src[contentStart:pos], src[next:], nil
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
