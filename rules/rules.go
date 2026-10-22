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

	"github.com/goccy/go-yaml"
)

// Type is a rule's decision type.
type Type string

// Decision types a rule can declare through its "type" frontmatter field.
const (
	TypeNoul   Type = "noul"
	TypeChoice Type = "choice"
	TypeScore  Type = "score"
)

// DefaultMinConfidence is the probability of a violation above which a noul is
// reported as failed when its rule declares none.
const DefaultMinConfidence = 0.5

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
	// MinConfidence is the probability of a violation above which a noul is
	// reported as failed. It applies to noul rules only; choice and score rules
	// leave it zero. A noul that declares none defaults to DefaultMinConfidence.
	MinConfidence float64
	// Body is everything following the frontmatter, preserved verbatim.
	Body string
}

// Parse parses a single rule from raw markdown file contents.
//
// Frontmatter is required. It must begin on the first line and be delimited by
// lines containing exactly "---". The body is returned verbatim, including any
// leading or trailing blank lines.
func Parse(data []byte) (Rule, error) {
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Rule{}, err
	}

	var cfg struct {
		AppliesTo     globList        `yaml:"appliesTo"`
		Type          Type            `yaml:"type"`
		Criteria      yaml.RawMessage `yaml:"criteria"`
		MinConfidence *float64        `yaml:"minConfidence"`
	}
	if err := yaml.Unmarshal([]byte(fm), &cfg); err != nil {
		return Rule{}, err
	}

	// The declared type fixes the shape of criteria: a choice's options are a
	// mapping, a score's levels are a list. So the type is decoded with the rest
	// of the frontmatter first, then criteria is decoded into the shape it
	// implies.
	var criteria Criteria
	switch cfg.Type {
	case TypeNoul:
	case TypeChoice:
		if err := yaml.Unmarshal(cfg.Criteria, &criteria.Options); err != nil {
			return Rule{}, err
		}
		if len(criteria.Options) == 0 {
			return Rule{}, errors.New("type choice requires criteria options")
		}
	case TypeScore:
		if err := yaml.Unmarshal(cfg.Criteria, &criteria.Levels); err != nil {
			return Rule{}, err
		}
		if len(criteria.Levels) == 0 {
			return Rule{}, errors.New("type score requires criteria levels")
		}
	default:
		return Rule{}, fmt.Errorf("missing or invalid type %q", cfg.Type)
	}

	// A noul asks whether the file violates the rule, so its confidence is the
	// probability of a violation and 0.5 is the midpoint above which a violation
	// is reported. minConfidence raises that bar; the other types answer with a
	// different confidence and have no such midpoint, so they reject the field.
	minConfidence := 0.0
	if cfg.Type == TypeNoul {
		minConfidence = DefaultMinConfidence
	}
	if cfg.MinConfidence != nil {
		if cfg.Type != TypeNoul {
			return Rule{}, errors.New("minConfidence is only supported for type noul")
		}
		minConfidence = *cfg.MinConfidence
		if minConfidence < 0 || minConfidence > 1 {
			return Rule{}, fmt.Errorf("minConfidence must be between 0 and 1, got %v", minConfidence)
		}
	}

	// A malformed glob would silently never match, so reject it here where the
	// file can be named rather than leaving a rule that does nothing.
	for _, glob := range cfg.AppliesTo {
		if _, err := path.Match(glob, ""); err != nil {
			return Rule{}, fmt.Errorf("invalid appliesTo glob %q: %w", glob, err)
		}
	}

	return Rule{
		AppliesTo:     cfg.AppliesTo,
		Type:          cfg.Type,
		Criteria:      criteria,
		MinConfidence: minConfidence,
		Body:          body,
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

// Applies reports whether any of the rule's globs match name. A malformed glob
// never matches; Parse rejects malformed globs, so this only guards rules built
// directly.
func (r Rule) Applies(name string) bool {
	for _, glob := range r.AppliesTo {
		if ok, err := path.Match(glob, name); err == nil && ok {
			return true
		}
	}
	return false
}

// Instructions is the text a model is asked about for this rule. A noul body
// states a rule, so it is framed as a question; choice and score bodies already
// read as instructions.
func (r Rule) Instructions() string {
	if r.Type == TypeNoul {
		return "Does this file violate the following rule?\n\n" + r.Body
	}
	return r.Body
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
