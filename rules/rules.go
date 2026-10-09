// Package rules reads and parses gallant lint rules.
//
// A rule is a markdown file whose optional frontmatter carries configuration
// and whose body is the lint text. Rules live in a directory (by default
// ./agents/gallant, relative to the current working directory) and declare the
// files they apply to through the "appliesTo" glob field.
package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDir is the directory, relative to the current working directory, from
// which gallant reads lint rules.
const DefaultDir = "agents/gallant"

// Rule is a single lint rule loaded from a markdown file.
type Rule struct {
	// Path is empty for rules produced by Parse, which are not backed by a file.
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

func ParseFile(path string) (Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Rule{}, err
	}

	rule, err := Parse(data)
	if err != nil {
		return Rule{}, fmt.Errorf("%s: %w", path, err)
	}
	rule.Path = path
	return rule, nil
}

// Load reads every markdown rule under dir, including subdirectories. Rules are
// returned in lexical path order. Hidden files and directories (those whose name
// begins with ".") are ignored.
func Load(dir string) ([]Rule, error) {
	var rules []Rule
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isMarkdown(d.Name()) {
			return nil
		}

		rule, err := ParseFile(path)
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

// LoadDefault reads every markdown rule under DefaultDir (./agents/gallant,
// relative to the current working directory).
func LoadDefault() ([]Rule, error) {
	return Load(DefaultDir)
}

func isMarkdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}
