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
	"slices"
	"strconv"
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
	// Violation is the condition under which a choice or score rule is reported
	// as failed. It is nil when the rule declares none, and always nil for a
	// noul, which states its threshold with MinConfidence instead.
	Violation *Violation
	// Body is everything following the frontmatter, preserved verbatim. It is
	// the rule text the model is asked about.
	Body string
}

// Violation declares when a choice or score rule is reported as failed. If
// lists the results that make a violation and Unless the results that cancel
// one. An entry fires when the model gives its result a probability strictly
// above the entry's threshold.
//
// The rule fails when some If entry fires and no Unless entry does. Because the
// outcomes of one decision sum to one, a violation is usually a single result or
// a few unlikely ones, and an Unless entry is only reachable when it names a
// result that If does not.
type Violation struct {
	If     map[string]float64 `yaml:"if"`
	Unless map[string]float64 `yaml:"unless"`
}

// Fails reports whether probability violates the rule, and why. Results missing
// from probability count as zero. When several entries fire, the reason names
// the first in name order so a run is reproducible.
func (v *Violation) Fails(probability map[string]float64) (bool, string) {
	trigger, p, threshold, ok := firstAbove(v.If, probability)
	if !ok {
		return false, "no if result above its threshold"
	}
	if name, q, limit, ok := firstAbove(v.Unless, probability); ok {
		return false, fmt.Sprintf("unless %s %s > %s", name, probabilityText(q), probabilityText(limit))
	}
	return true, fmt.Sprintf("%s %s > %s", trigger, probabilityText(p), probabilityText(threshold))
}

// firstAbove returns the first entry in name order whose result exceeds its
// threshold.
func firstAbove(clauses, probability map[string]float64) (name string, p, threshold float64, ok bool) {
	for _, name := range sortedKeys(clauses) {
		if probability[name] > clauses[name] {
			return name, probability[name], clauses[name], true
		}
	}
	return "", 0, 0, false
}

// sortedKeys returns the keys of m in lexical order.
func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// probabilityText renders a probability without trailing zeros, so a reason
// reads "useless 0.95 > 0.5" and stays unambiguous at the threshold.
func probabilityText(p float64) string {
	return strconv.FormatFloat(p, 'g', 4, 64)
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
		Violation     yaml.RawMessage `yaml:"violation"`
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

	// A violation condition names the results it decides on, so it is checked
	// against the criteria that type has just established. It is decoded
	// strictly because a misspelled key would otherwise silently never fire.
	var violation *Violation
	if len(cfg.Violation) > 0 {
		if cfg.Type == TypeNoul {
			return Rule{}, errors.New("violation is only supported for type choice or score; use minConfidence for a noul")
		}
		violation = &Violation{}
		if err := yaml.UnmarshalWithOptions(cfg.Violation, violation, yaml.DisallowUnknownField()); err != nil {
			return Rule{}, fmt.Errorf("violation: %w", err)
		}
		if len(violation.If) == 0 {
			return Rule{}, errors.New("violation requires at least one result under if")
		}

		// Results are addressed by name, so a score's levels must be distinct.
		known := make(map[string]bool, len(criteria.Options)+len(criteria.Levels))
		for name := range criteria.Options {
			known[name] = true
		}
		for _, level := range criteria.Levels {
			if known[level] {
				return Rule{}, fmt.Errorf("violation requires unique criteria levels, but %q repeats", level)
			}
			known[level] = true
		}

		for _, clause := range []struct {
			name       string
			thresholds map[string]float64
		}{
			{"if", violation.If},
			{"unless", violation.Unless},
		} {
			for _, name := range sortedKeys(clause.thresholds) {
				threshold := clause.thresholds[name]
				if !known[name] {
					return Rule{}, fmt.Errorf("violation.%s names unknown result %q", clause.name, name)
				}
				if threshold < 0 || threshold > 1 {
					return Rule{}, fmt.Errorf("violation.%s threshold for %q must be between 0 and 1, got %v", clause.name, name, threshold)
				}
			}
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
		Violation:     violation,
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
