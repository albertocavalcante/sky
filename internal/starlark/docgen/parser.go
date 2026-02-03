package docgen

import (
	"regexp"
	"strings"
)

// ParsedDocstring represents a parsed docstring with sections.
type ParsedDocstring struct {
	// Summary is the first paragraph (short description).
	Summary string

	// Description is the full description (after summary, before sections).
	Description string

	// Args maps parameter names to their descriptions.
	Args map[string]string

	// Returns is the return value description.
	Returns string

	// Raises maps exception types to their descriptions.
	Raises map[string]string

	// Example contains example code.
	Example string

	// Note contains additional notes.
	Note string
}

var sectionRegex = regexp.MustCompile(`(?m)^\s*(Args|Arguments|Parameters|Returns?|Yields?|Raises|Throws|Examples?|Notes?|See Also|Deprecated|Warning|Todo):`)

// ParseDocstring parses a docstring into structured sections.
func ParseDocstring(docstring string) *ParsedDocstring {
	parsed := &ParsedDocstring{
		Args:   make(map[string]string),
		Raises: make(map[string]string),
	}

	if docstring == "" {
		return parsed
	}

	// Normalize line endings
	docstring = strings.ReplaceAll(docstring, "\r\n", "\n")

	// Split into sections
	sections := splitSections(docstring)

	for header, content := range sections {
		header = strings.ToLower(strings.TrimSuffix(header, ":"))

		switch header {
		case "":
			// Main description
			parts := splitSummaryDescription(strings.TrimSpace(content))
			parsed.Summary = parts[0]
			if len(parts) > 1 {
				parsed.Description = parts[1]
			}

		case "args", "arguments", "parameters":
			// Don't TrimSpace - parseArgsSection needs indentation to detect arg boundaries
			parsed.Args = parseArgsSection(content)

		case "returns", "return":
			parsed.Returns = strings.TrimSpace(content)

		case "yields":
			parsed.Returns = strings.TrimSpace(content) // Treat yields like returns

		case "raises", "throws":
			// Don't TrimSpace - parseArgsSection needs indentation to detect arg boundaries
			parsed.Raises = parseArgsSection(content)

		case "example", "examples":
			parsed.Example = strings.TrimSpace(content)

		case "note", "notes":
			parsed.Note = strings.TrimSpace(content)
		}
	}

	return parsed
}

// splitSections splits a docstring into sections based on headers.
func splitSections(docstring string) map[string]string {
	sections := make(map[string]string)

	// Find all section positions
	matches := sectionRegex.FindAllStringIndex(docstring, -1)

	if len(matches) == 0 {
		// No sections, entire string is description
		sections[""] = docstring
		return sections
	}

	// Extract content before first section
	if matches[0][0] > 0 {
		sections[""] = strings.TrimSpace(docstring[:matches[0][0]])
	}

	// Extract each section
	for i, match := range matches {
		// Find the header (may have leading whitespace due to regex allowing ^\s*)
		headerEnd := strings.Index(docstring[match[0]:], ":")
		if headerEnd == -1 {
			continue
		}
		header := strings.TrimSpace(docstring[match[0] : match[0]+headerEnd+1])

		// Find content end (next section or end of string)
		contentStart := match[0] + headerEnd + 1
		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(docstring)
		}

		content := docstring[contentStart:contentEnd]
		// Only trim trailing whitespace, preserve leading newlines and indentation
		content = strings.TrimRight(content, " \t\n\r")
		sections[header] = content
	}

	return sections
}

// splitSummaryDescription splits text into summary (first paragraph) and description (rest).
func splitSummaryDescription(text string) []string {
	// Find first blank line
	parts := strings.SplitN(text, "\n\n", 2)
	if len(parts) == 1 {
		return []string{strings.TrimSpace(parts[0])}
	}
	return []string{
		strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[1]),
	}
}

// parseArgsSection parses an Args: section into name->description map.
func parseArgsSection(content string) map[string]string {
	args := make(map[string]string)

	lines := strings.Split(content, "\n")
	var currentArg string
	var currentDesc strings.Builder
	baseIndent := -1

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Calculate indentation of this line
		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else if ch == '\t' {
				indent += 4
			} else {
				break
			}
		}

		trimmed := strings.TrimSpace(line)

		// Establish base indentation from first non-empty line
		if baseIndent < 0 {
			baseIndent = indent
		}

		// Check if this is a new argument definition
		// A new arg starts at base indentation level and has "name:" pattern
		colonIdx := strings.Index(trimmed, ":")
		if indent <= baseIndent && colonIdx > 0 {
			// Save previous arg
			if currentArg != "" {
				args[currentArg] = strings.TrimSpace(currentDesc.String())
			}

			// Parse new arg
			currentArg = strings.TrimSpace(trimmed[:colonIdx])
			currentDesc.Reset()
			if colonIdx+1 < len(trimmed) {
				currentDesc.WriteString(strings.TrimSpace(trimmed[colonIdx+1:]))
			}
		} else if currentArg != "" {
			// Continuation of current arg description
			if currentDesc.Len() > 0 {
				currentDesc.WriteString(" ")
			}
			currentDesc.WriteString(trimmed)
		}
	}

	// Save last arg
	if currentArg != "" {
		args[currentArg] = strings.TrimSpace(currentDesc.String())
	}

	return args
}

// HasDocumentation returns true if the parsed docstring has meaningful content.
func (p *ParsedDocstring) HasDocumentation() bool {
	if p == nil {
		return false
	}
	return p.Summary != "" || p.Description != "" || len(p.Args) > 0 ||
		p.Returns != "" || len(p.Raises) > 0 || p.Example != ""
}
