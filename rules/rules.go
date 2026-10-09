// Package rules reads and parses gallant lint rules.
//
// A rule is a markdown file whose optional frontmatter carries configuration
// and whose body is the lint text. Rules are loaded from an fs.FS supplied by
// the caller and declare the files they apply to through the "appliesTo" glob
// field.
package rules

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Rule is a single lint rule loaded from a markdown file.
type Rule struct {
	// Path is the rule's path within the filesystem it was loaded from. It is
	// empty for rules produced by Parse.
	Path string
	// AppliesTo is nil when the rule declares no patterns.
	AppliesTo []string
	// Body is everything following the frontmatter, preserved verbatim.
	Body string
}

// Parse parses a single rule from raw markdown file contents.
//
// Frontmatter is optional. When present it must begin on the first line and be
// delimited by lines containing exactly "---". The body is returned verbatim,
// including any leading or trailing blank lines.
func Parse(data []byte) (Rule, error) {
	fm, body, ok := splitFrontmatter(string(data))
	rule := Rule{Body: body}
	if !ok {
		return rule, nil
	}

	cfg, err := parseConfig(fm)
	if err != nil {
		return Rule{}, err
	}
	rule.AppliesTo = []string(cfg.AppliesTo)
	return rule, nil
}

// ParseFile parses the rule at name within fsys.
func ParseFile(fsys fs.FS, name string) (Rule, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Rule{}, err
	}

	rule, err := Parse(data)
	if err != nil {
		return Rule{}, fmt.Errorf("%s: %w", name, err)
	}
	rule.Path = name
	return rule, nil
}

// Load reads every markdown rule from fsys, including subdirectories. Rules are
// returned in lexical path order. Hidden files and directories (those whose name
// begins with ".") are ignored.
func Load(fsys fs.FS) ([]Rule, error) {
	var rules []Rule
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isMarkdown(d.Name()) {
			return nil
		}

		rule, err := ParseFile(fsys, p)
		if err != nil {
			return err
		}
		rules = append(rules, rule)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func isMarkdown(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}
