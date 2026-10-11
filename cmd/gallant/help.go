package main

import (
	_ "embed"
	"fmt"
	"io"
)

// The help texts live as plain files so they are easy to edit, and are embedded
// so the command carries them with no runtime files.

//go:embed help/usage.txt
var usageText string

//go:embed help/rules.txt
var rulesHelpText string

//go:embed help/providers.txt
var providersHelpText string

//go:embed help/output.txt
var outputHelpText string

func printUsage(w io.Writer)         { fmt.Fprint(w, usageText) }
func printRulesHelp(w io.Writer)     { fmt.Fprint(w, rulesHelpText) }
func printProvidersHelp(w io.Writer) { fmt.Fprint(w, providersHelpText) }
func printOutputHelp(w io.Writer)    { fmt.Fprint(w, outputHelpText) }
