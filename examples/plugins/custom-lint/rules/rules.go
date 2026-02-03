// Package rules provides custom lint rules for Starlark files.
package rules

import (
	"os"

	"github.com/bazelbuild/buildtools/build"
)

// Finding represents a lint finding.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Rule defines a lint rule.
type Rule struct {
	Name        string
	Description string
	Check       func(file *build.File, path string) []Finding
}

// AllRules contains all available lint rules.
var AllRules = []Rule{
	NoPrint,
	MaxParams,
	NoUnderscore,
}

// LintFile runs all lint rules on a file.
func LintFile(path string) ([]Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	file, err := build.ParseDefault(path, content)
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, rule := range AllRules {
		findings = append(findings, rule.Check(file, path)...)
	}

	return findings, nil
}
