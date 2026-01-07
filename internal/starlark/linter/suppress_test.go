package linter

import (
	"testing"
)

// TestSuppressionParser_SameLine verifies "skylint: disable=rule" on the same line.
func TestSuppressionParser_SameLine(t *testing.T) {
	source := `# skylint: disable=rule-name
print("hello")
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("Finding on line 1 should be suppressed")
	}

	finding2 := Finding{
		FilePath: "test.star",
		Line:     2,
		Rule:     "rule-name",
	}

	if parser.IsSuppressed(finding2) {
		t.Error("Finding on line 2 should NOT be suppressed")
	}
}

// TestSuppressionParser_NextLine verifies "skylint: disable-next-line=rule".
func TestSuppressionParser_NextLine(t *testing.T) {
	source := `# skylint: disable-next-line=rule-name
print("hello")
print("world")
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     2,
		Rule:     "rule-name",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("Finding on line 2 should be suppressed")
	}

	finding2 := Finding{
		FilePath: "test.star",
		Line:     3,
		Rule:     "rule-name",
	}

	if parser.IsSuppressed(finding2) {
		t.Error("Finding on line 3 should NOT be suppressed")
	}
}

// TestSuppressionParser_Inline verifies inline suppression with code before comment.
func TestSuppressionParser_Inline(t *testing.T) {
	source := `code()  # skylint: disable=rule-name
another_code()
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("Finding on line 1 should be suppressed (inline)")
	}

	// Verify it's actually classified as inline
	suppressions := parser.suppressions[1]
	if len(suppressions) == 0 {
		t.Fatal("No suppressions found for line 1")
	}
	if suppressions[0].Type != SuppressionInline {
		t.Errorf("Expected SuppressionInline, got %v", suppressions[0].Type)
	}
}

// TestSuppressionParser_MultipleRules verifies "disable=rule1,rule2,rule3".
func TestSuppressionParser_MultipleRules(t *testing.T) {
	source := `# skylint: disable=rule1,rule2,rule3
code()
`
	parser := NewSuppressionParser([]byte(source))

	for _, ruleName := range []string{"rule1", "rule2", "rule3"} {
		finding := Finding{
			FilePath: "test.star",
			Line:     1,
			Rule:     ruleName,
		}

		if !parser.IsSuppressed(finding) {
			t.Errorf("Finding for rule %s should be suppressed", ruleName)
		}
	}

	// Rule not in the list should not be suppressed
	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "other-rule",
	}

	if parser.IsSuppressed(finding) {
		t.Error("Finding for other-rule should NOT be suppressed")
	}
}

// TestSuppressionParser_DisableAll verifies "disable=all" and "disable=".
func TestSuppressionParser_DisableAll(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"disable=all", `# skylint: disable=all
code()
`},
		{"disable=", `# skylint: disable=
code()
`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewSuppressionParser([]byte(tt.source))

			for _, ruleName := range []string{"rule1", "rule2", "any-rule"} {
				finding := Finding{
					FilePath: "test.star",
					Line:     1,
					Rule:     ruleName,
				}

				if !parser.IsSuppressed(finding) {
					t.Errorf("Finding for rule %s should be suppressed (all)", ruleName)
				}
			}
		})
	}
}

// TestSuppressionParser_CaseSensitive verifies case sensitivity of rule names.
func TestSuppressionParser_CaseSensitive(t *testing.T) {
	source := `# skylint: disable=RuleName
code()
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "RuleName",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("Exact case match should suppress")
	}

	finding2 := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rulename",
	}

	if parser.IsSuppressed(finding2) {
		t.Error("Different case should NOT suppress")
	}
}

// TestSuppressionParser_Malformed verifies that malformed comments don't crash.
func TestSuppressionParser_Malformed(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"no equals", "# skylint: disable\ncode()\n"},
		{"trailing comma", "# skylint: disable=rule1,\ncode()\n"},
		{"spaces in rules", "# skylint: disable=rule 1, rule 2\ncode()\n"},
		{"special chars", "# skylint: disable=rule!@#$%\ncode()\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			parser := NewSuppressionParser([]byte(tt.source))
			_ = parser

			finding := Finding{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule1",
			}

			// Just verify it doesn't crash
			_ = parser.IsSuppressed(finding)
		})
	}
}

// TestSuppressionParser_ExtraWhitespace verifies handling of extra whitespace.
func TestSuppressionParser_ExtraWhitespace(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"spaces around equals", "# skylint: disable = rule-name\ncode()\n"},
		{"tabs", "# skylint:\tdisable=rule-name\ncode()\n"},
		{"multiple spaces", "#  skylint:  disable=rule-name\ncode()\n"},
		{"trailing spaces", "# skylint: disable=rule-name   \ncode()\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewSuppressionParser([]byte(tt.source))

			finding := Finding{
				FilePath: "test.star",
				Line:     1,
				Rule:     "rule-name",
			}

			// Some of these might not suppress due to strict parsing,
			// but they should not crash
			_ = parser.IsSuppressed(finding)
		})
	}
}

// TestSuppressionParser_NoSpace verifies "skylint:disable=" without space.
func TestSuppressionParser_NoSpace(t *testing.T) {
	source := `# skylint:disable=rule-name
code()
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("skylint:disable= (no space) should work")
	}
}

// TestSuppressionParser_EndOfFile verifies comment at end of file.
func TestSuppressionParser_EndOfFile(t *testing.T) {
	source := `code()
# skylint: disable=rule-name`

	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     2,
		Rule:     "rule-name",
	}

	if !parser.IsSuppressed(finding) {
		t.Error("Comment at end of file should work")
	}
}

// TestSuppressionParser_NestedSuppressions verifies multiple suppressions on same line.
func TestSuppressionParser_NestedSuppressions(t *testing.T) {
	// This tests the case where we might have multiple suppression directives
	// (though this is unusual)
	source := `# skylint: disable=rule1 skylint: disable-next-line=rule2
code()
`
	parser := NewSuppressionParser([]byte(source))

	finding1 := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule1",
	}

	if !parser.IsSuppressed(finding1) {
		t.Error("First suppression should work")
	}

	finding2 := Finding{
		FilePath: "test.star",
		Line:     2,
		Rule:     "rule2",
	}

	if !parser.IsSuppressed(finding2) {
		t.Error("Second suppression (next-line) should work")
	}
}

// TestSuppressionParser_EmptySource verifies handling of empty source.
func TestSuppressionParser_EmptySource(t *testing.T) {
	parser := NewSuppressionParser([]byte(""))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if parser.IsSuppressed(finding) {
		t.Error("Empty source should not suppress anything")
	}
}

// TestSuppressionParser_NoComments verifies source with no comments.
func TestSuppressionParser_NoComments(t *testing.T) {
	source := `def foo():
    pass
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if parser.IsSuppressed(finding) {
		t.Error("No comments should not suppress anything")
	}
}

// TestSuppressionParser_RegularComment verifies that regular comments are ignored.
func TestSuppressionParser_RegularComment(t *testing.T) {
	source := `# This is a regular comment
code()
`
	parser := NewSuppressionParser([]byte(source))

	finding := Finding{
		FilePath: "test.star",
		Line:     1,
		Rule:     "rule-name",
	}

	if parser.IsSuppressed(finding) {
		t.Error("Regular comment should not suppress")
	}
}

// TestFilterSuppressed verifies FilterSuppressed function.
func TestFilterSuppressed(t *testing.T) {
	source := `# skylint: disable=rule1
code()  # skylint: disable=rule2
code()
`
	parser := NewSuppressionParser([]byte(source))

	findings := []Finding{
		{Line: 1, Rule: "rule1"},
		{Line: 2, Rule: "rule2"},
		{Line: 3, Rule: "rule3"},
		{Line: 1, Rule: "other-rule"},
	}

	filtered := FilterSuppressed(findings, parser)

	// Should have 2 findings: line 3 rule3, and line 1 other-rule
	if len(filtered) != 2 {
		t.Errorf("Expected 2 findings after filtering, got %d", len(filtered))
	}

	// Verify the remaining findings
	for _, f := range filtered {
		if f.Line == 1 && f.Rule == "rule1" {
			t.Error("rule1 on line 1 should be filtered out")
		}
		if f.Line == 2 && f.Rule == "rule2" {
			t.Error("rule2 on line 2 should be filtered out")
		}
	}
}

// TestFilterSuppressed_EmptyList verifies FilterSuppressed with empty input.
func TestFilterSuppressed_EmptyList(t *testing.T) {
	parser := NewSuppressionParser([]byte(""))
	findings := []Finding{}

	filtered := FilterSuppressed(findings, parser)

	if len(filtered) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(filtered))
	}
}

// TestFilterSuppressed_NoneMatching verifies FilterSuppressed with no matches.
func TestFilterSuppressed_NoneMatching(t *testing.T) {
	source := `# skylint: disable=rule1
code()
`
	parser := NewSuppressionParser([]byte(source))

	findings := []Finding{
		{Line: 2, Rule: "rule2"},
		{Line: 3, Rule: "rule3"},
	}

	filtered := FilterSuppressed(findings, parser)

	if len(filtered) != len(findings) {
		t.Errorf("Expected %d findings (none suppressed), got %d", len(findings), len(filtered))
	}
}

// TestSuppressionParser_InlineVsLine verifies correct distinction between inline and line suppressions.
func TestSuppressionParser_InlineVsLine(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		expectedType SuppressionType
		line         int
	}{
		{
			name:         "standalone comment",
			source:       "# skylint: disable=rule\n",
			expectedType: SuppressionLine,
			line:         1,
		},
		{
			name:         "inline after code",
			source:       "code()  # skylint: disable=rule\n",
			expectedType: SuppressionInline,
			line:         1,
		},
		{
			name:         "inline with tab",
			source:       "code()\t# skylint: disable=rule\n",
			expectedType: SuppressionInline,
			line:         1,
		},
		{
			name:         "indented standalone",
			source:       "    # skylint: disable=rule\n",
			expectedType: SuppressionLine,
			line:         1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewSuppressionParser([]byte(tt.source))

			suppressions := parser.suppressions[tt.line]
			if len(suppressions) == 0 {
				t.Fatalf("No suppressions found for line %d", tt.line)
			}

			if suppressions[0].Type != tt.expectedType {
				t.Errorf("Expected type %v, got %v", tt.expectedType, suppressions[0].Type)
			}
		})
	}
}
