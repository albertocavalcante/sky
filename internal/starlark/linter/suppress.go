package linter

import (
	"regexp"
	"strings"
)

// ruleListPrefix matches a leading comma-separated rule list such as
// "a, b ,c" (but not trailing prose after it).
var ruleListPrefix = regexp.MustCompile(`^[^\s,]*(?:\s*,\s*[^\s,]*)*`)

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

	// commentStarts holds, per line index, the offset of the comment-starting
	// "#" outside any string literal, or -1.
	commentStarts []int
}

// NewSuppressionParser creates a new parser for the given source content.
func NewSuppressionParser(content []byte) *SuppressionParser {
	source := string(content)
	lines := strings.Split(source, "\n")

	parser := &SuppressionParser{
		lines:        lines,
		suppressions: make(map[int][]Suppression),
	}

	parser.commentStarts = findCommentStarts(lines)
	parser.parse()
	return parser
}

// findCommentStarts returns, per line index, the byte offset of the "#" that
// starts a comment, or -1 when the line has none. A "#" inside a single-,
// double- or triple-quoted string literal (including one continued from an
// earlier line) is not a comment.
func findCommentStarts(lines []string) []int {
	starts := make([]int, len(lines))
	var quote byte // active quote character, 0 when outside a string
	triple := false
	for n, line := range lines {
		starts[n] = -1
		for i := 0; i < len(line); i++ {
			c := line[i]
			if quote == 0 {
				switch c {
				case '#':
					starts[n] = i
					i = len(line)
				case '"', '\'':
					quote = c
					triple = strings.HasPrefix(line[i:], strings.Repeat(string(c), 3))
					if triple {
						i += 2
					}
				}
				continue
			}
			if c == '\\' {
				i++ // skip the escaped character
				continue
			}
			if c != quote {
				continue
			}
			if !triple {
				quote = 0
			} else if strings.HasPrefix(line[i:], strings.Repeat(string(c), 3)) {
				quote = 0
				triple = false
				i += 2
			}
		}
		if quote != 0 && !triple {
			quote = 0 // unterminated single-line string ends at the newline
		}
	}
	return starts
}

// parse scans all lines for suppression comments.
func (p *SuppressionParser) parse() {
	for lineNum, line := range p.lines {
		// Line numbers are 1-based
		currentLine := lineNum + 1

		// Look for skylint comments
		suppressions := p.parseLineForSuppressions(line, p.commentStarts[lineNum], currentLine)
		if len(suppressions) > 0 {
			p.suppressions[currentLine] = suppressions
		}
	}
}

// parseLineForSuppressions extracts all suppression directives from a line.
func (p *SuppressionParser) parseLineForSuppressions(line string, commentIdx, lineNum int) []Suppression {
	var suppressions []Suppression

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
	if supp := p.parseDisableDirective(line, commentIdx, comment, lineNum); supp != nil {
		suppressions = append(suppressions, *supp)
	}

	if supp := p.parseDisableNextLineDirective(comment, lineNum); supp != nil {
		suppressions = append(suppressions, *supp)
	}

	return suppressions
}

// parseDisableDirective parses a "skylint: disable=rule-name" directive.
func (p *SuppressionParser) parseDisableDirective(line string, commentIdx int, comment string, lineNum int) *Suppression {
	// Look for "skylint: disable=..." (note: not disable-next-line)
	disablePrefix, idx := findDirectivePrefix(comment, "disable=")
	if idx == -1 {
		return nil
	}

	// Extract the rule list
	rulesStr := comment[idx+len(disablePrefix):]
	// Take a comma-separated list (whitespace around commas allowed); stop
	// at the first whitespace that is not followed by a comma-joined name.
	rules, ok := parseDirectiveRules(rulesStr)
	if !ok {
		return nil
	}

	// Determine if this is inline (has code before the comment) or line suppression
	// Check the original line before the comment starts
	codeBeforeComment := strings.TrimSpace(line[:commentIdx])
	if len(codeBeforeComment) > 0 {
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
	disablePrefix, idx := findDirectivePrefix(comment, "disable-next-line=")
	if idx == -1 {
		return nil
	}

	// Extract the rule list
	rulesStr := comment[idx+len(disablePrefix):]
	// Take a comma-separated list (whitespace around commas allowed); stop
	// at the first whitespace that is not followed by a comma-joined name.
	rules, ok := parseDirectiveRules(rulesStr)
	if !ok {
		return nil
	}

	return &Suppression{
		Type:  SuppressionNextLine,
		Rules: rules,
		Line:  lineNum,
	}
}

// findDirectivePrefix looks for a skylint directive prefix in a comment.
// It tries both "skylint: <directive>" and "skylint:<directive>" (without space).
// Returns the prefix that was found and its index, or ("", -1) if not found.
func findDirectivePrefix(comment string, directive string) (string, int) {
	// Try with space first
	prefix := "skylint: " + directive
	idx := strings.Index(comment, prefix)
	if idx != -1 {
		return prefix, idx
	}

	// Try without space
	prefix = "skylint:" + directive
	idx = strings.Index(comment, prefix)
	if idx != -1 {
		return prefix, idx
	}

	return "", -1
}

// parseDirectiveRules extracts the rule list following "disable=". An empty
// or "all" list means every rule (empty slice, ok). A nonempty list that
// contains no rule names (e.g. ",,") is malformed: ok is false and nothing
// must be suppressed.
func parseDirectiveRules(raw string) (rules []string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, true
	}
	rules = parseRuleList(ruleListPrefix.FindString(raw))
	if rules == nil {
		return nil, false
	}
	return rules, true
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
	if len(rules) == 1 && rules[0] == "all" {
		return []string{}
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
	filtered := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		if !parser.IsSuppressed(finding) {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}
