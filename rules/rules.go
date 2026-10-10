// Package rules reads and parses gallant lint rules.
//
// A rule is a markdown file whose frontmatter declares a decision type and whose
// body is the lint text. Rules are loaded from an fs.FS supplied by the caller
// and declare which file names they apply to through the "appliesTo" glob
// field.
// glob field, matched against file names rather than paths.
package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Type is a rule's decision type.
type Type string

// Decision types a rule can declare through its "type" frontmatter field.
const (
	TypeNoul   Type = "noul"
	TypeChoice Type = "choice"
	TypeScore  Type = "score"
)

// Rule is a single lint rule loaded from a markdown file.
type Rule struct {
	// Path is the rule's path within the filesystem it was loaded from. It is
	// empty for rules produced by Parse.
	Path string
	// AppliesTo is nil when the rule declares no patterns.
	AppliesTo []string
	// Type is the decision the model makes: TypeNoul, TypeChoice, or TypeScore.
	Type Type
	// Criteria is the options (choice) or levels (score) the decision is made
	// from.
	Criteria Criteria
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
	if !ok {
		return Rule{}, errors.New("missing frontmatter")
	}

	var cfg struct {
		AppliesTo globList `yaml:"appliesTo"`
		Type      Type     `yaml:"type"`
		Criteria  Criteria `yaml:"criteria"`
	}
	if err := yaml.Unmarshal([]byte(fm), &cfg); err != nil {
		return Rule{}, err
	}
	switch cfg.Type {
	case TypeNoul:
	case TypeChoice:
		if len(cfg.Criteria.Options) == 0 {
			return Rule{}, errors.New("type choice requires criteria options")
		}
	case TypeScore:
		if len(cfg.Criteria.Levels) == 0 {
			return Rule{}, errors.New("type score requires criteria levels")
		}
	default:
		return Rule{}, fmt.Errorf("missing or invalid type %q", cfg.Type)
	}

	return Rule{
		AppliesTo: cfg.AppliesTo,
		Type:      cfg.Type,
		Criteria:  cfg.Criteria,
		Body:      body,
	}, nil
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

// Applies reports whether any of the rule's globs match name. Malformed globs
// never match.
func (r Rule) Applies(name string) bool {
	for _, glob := range r.AppliesTo {
		if ok, err := path.Match(glob, name); err == nil && ok {
			return true
		}
	}
	return false
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
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(path.Ext(d.Name()))
		if ext != ".md" && ext != ".markdown" {
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
