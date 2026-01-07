package linter

import (
	"strings"
)

// SuppressionType represents the type of suppression comment.
type SuppressionType int

const (
	// SuppressionNone indicates no suppression.
	SuppressionNone SuppressionType = iota

	// SuppressionLine suppresses rules on the current line.
	// Format: # skylint: disable=rule-name
	SuppressionLine

	// SuppressionNextLine suppresses rules on the next line.
	// Format: # skylint: disable-next-line=rule-name
	SuppressionNextLine

	// SuppressionInline suppresses rules on the same line as code.
	// Format: code_here()  # skylint: disable=rule-name
	SuppressionInline
)

// Suppression represents a suppression directive parsed from a comment.
type Suppression struct {
	// Type is the type of suppression (line, next-line, inline).
	Type SuppressionType

	// Rules is the list of rule names to suppress.
	// An empty list means suppress all rules.
	Rules []string

	// Line is the 1-based line number where the suppression appears.
	Line int
}

// SuppressionParser parses suppression comments from source code.
type SuppressionParser struct {
	// lines contains the source code split into lines
	lines []string

	// suppressions maps line numbers to their suppressions
	suppressions map[int][]Suppression
}

// NewSuppressionParser creates a new parser for the given source content.
func NewSuppressionParser(content []byte) *SuppressionParser {
	source := string(content)
	lines := strings.Split(source, "\n")

	parser := &SuppressionParser{
		lines:        lines,
		suppressions: make(map[int][]Suppression),
	}

	parser.parse()
	return parser
}

// parse scans all lines for suppression comments.
func (p *SuppressionParser) parse() {
	for lineNum, line := range p.lines {
		// Line numbers are 1-based
		currentLine := lineNum + 1

		// Look for skylint comments
		suppressions := p.parseLineForSuppressions(line, currentLine)
		if len(suppressions) > 0 {
			p.suppressions[currentLine] = suppressions
		}
	}
}

// parseLineForSuppressions extracts all suppression directives from a line.
func (p *SuppressionParser) parseLineForSuppressions(line string, lineNum int) []Suppression {
	var suppressions []Suppression

	// Find all comment positions
	commentIdx := strings.Index(line, "#")
	if commentIdx == -1 {
		return nil
	}

	// Extract everything after the #
	comment := line[commentIdx:]

	// Check if this is a skylint directive
	if !strings.Contains(comment, "skylint:") {
		return nil
	}

	// Look for disable patterns
	if supp := p.parseDisableDirective(comment, lineNum); supp != nil {
		suppressions = append(suppressions, *supp)
	}

	if supp := p.parseDisableNextLineDirective(comment, lineNum); supp != nil {
		suppressions = append(suppressions, *supp)
	}

	return suppressions
}

// parseDisableDirective parses a "skylint: disable=rule-name" directive.
func (p *SuppressionParser) parseDisableDirective(comment string, lineNum int) *Suppression {
	// Look for "skylint: disable=..."
	disablePrefix := "skylint: disable="
	idx := strings.Index(comment, disablePrefix)
	if idx == -1 {
		// Try without space
		disablePrefix = "skylint:disable="
		idx = strings.Index(comment, disablePrefix)
		if idx == -1 {
			return nil
		}
	}

	// Extract the rule list
	rulesStr := comment[idx+len(disablePrefix):]
	// Take everything until whitespace or end of line
	rulesStr = strings.TrimSpace(rulesStr)
	if spaceIdx := strings.IndexAny(rulesStr, " \t\n\r"); spaceIdx != -1 {
		rulesStr = rulesStr[:spaceIdx]
	}

	rules := parseRuleList(rulesStr)

	// Determine if this is inline (has code before the comment) or line suppression
	codeBeforeComment := strings.TrimSpace(comment[:idx])
	if len(codeBeforeComment) > 0 && !strings.HasPrefix(codeBeforeComment, "#") {
		return &Suppression{
			Type:  SuppressionInline,
			Rules: rules,
			Line:  lineNum,
		}
	}

	return &Suppression{
		Type:  SuppressionLine,
		Rules: rules,
		Line:  lineNum,
	}
}

// parseDisableNextLineDirective parses a "skylint: disable-next-line=rule-name" directive.
func (p *SuppressionParser) parseDisableNextLineDirective(comment string, lineNum int) *Suppression {
	// Look for "skylint: disable-next-line=..."
	disablePrefix := "skylint: disable-next-line="
	idx := strings.Index(comment, disablePrefix)
	if idx == -1 {
		// Try without space
		disablePrefix = "skylint:disable-next-line="
		idx = strings.Index(comment, disablePrefix)
		if idx == -1 {
			return nil
		}
	}

	// Extract the rule list
	rulesStr := comment[idx+len(disablePrefix):]
	// Take everything until whitespace or end of line
	rulesStr = strings.TrimSpace(rulesStr)
	if spaceIdx := strings.IndexAny(rulesStr, " \t\n\r"); spaceIdx != -1 {
		rulesStr = rulesStr[:spaceIdx]
	}

	rules := parseRuleList(rulesStr)

	return &Suppression{
		Type:  SuppressionNextLine,
		Rules: rules,
		Line:  lineNum,
	}
}

// parseRuleList parses a comma-separated list of rule names.
// An empty string or "all" means suppress all rules.
func parseRuleList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "all" {
		return []string{} // Empty list means all rules
	}

	parts := strings.Split(s, ",")
	var rules []string
	for _, part := range parts {
		rule := strings.TrimSpace(part)
		if rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

// IsSuppressed checks if a finding should be suppressed based on suppression directives.
func (p *SuppressionParser) IsSuppressed(finding Finding) bool {
	line := finding.Line

	// Check for inline suppression on the same line
	if suppressions, exists := p.suppressions[line]; exists {
		for _, supp := range suppressions {
			if supp.Type == SuppressionInline || supp.Type == SuppressionLine {
				if matchesSuppressionRules(finding, supp.Rules) {
					return true
				}
			}
		}
	}

	// Check for next-line suppression on the previous line
	if line > 1 {
		if suppressions, exists := p.suppressions[line-1]; exists {
			for _, supp := range suppressions {
				if supp.Type == SuppressionNextLine {
					if matchesSuppressionRules(finding, supp.Rules) {
						return true
					}
				}
			}
		}
	}

	return false
}

// matchesSuppressionRules checks if a finding matches the suppression rules.
// An empty rules list means suppress all rules.
func matchesSuppressionRules(finding Finding, rules []string) bool {
	// Empty list means suppress all rules
	if len(rules) == 0 {
		return true
	}

	// Check if the finding's rule is in the suppression list
	for _, rule := range rules {
		if rule == finding.Rule {
			return true
		}
	}

	return false
}

// FilterSuppressed removes suppressed findings from a list.
func FilterSuppressed(findings []Finding, parser *SuppressionParser) []Finding {
	var filtered []Finding
	for _, finding := range findings {
		if !parser.IsSuppressed(finding) {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}
