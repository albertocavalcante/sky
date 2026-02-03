package docgen

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// MarkdownOptions configures markdown rendering.
type MarkdownOptions struct {
	// Title is the document title (defaults to filename).
	Title string

	// IncludeTableOfContents adds a table of contents.
	IncludeTableOfContents bool

	// IncludeSourceLinks adds links to source lines.
	IncludeSourceLinks bool

	// SourceBaseURL is the base URL for source links.
	SourceBaseURL string
}

// DefaultMarkdownOptions returns sensible defaults.
func DefaultMarkdownOptions() MarkdownOptions {
	return MarkdownOptions{
		IncludeTableOfContents: true,
		IncludeSourceLinks:     false,
	}
}

// RenderMarkdown renders module documentation as Markdown.
func RenderMarkdown(w io.Writer, doc *ModuleDoc, opts MarkdownOptions) error {
	title := opts.Title
	if title == "" {
		title = filepath.Base(doc.File)
	}

	// Header
	writef(w, "# %s\n\n", title)

	// Module docstring
	if doc.Docstring != "" {
		writef(w, "%s\n\n", doc.Docstring)
	}

	// Table of contents
	if opts.IncludeTableOfContents && (len(doc.Functions) > 0 || len(doc.Globals) > 0) {
		writeln(w, "## Contents\n")

		if len(doc.Functions) > 0 {
			writeln(w, "### Functions\n")
			for _, fn := range doc.Functions {
				anchor := toAnchor(fn.Name)
				writef(w, "- [%s](#%s)\n", fn.Name, anchor)
			}
			writeln(w, "")
		}

		if len(doc.Globals) > 0 {
			writeln(w, "### Variables\n")
			for _, g := range doc.Globals {
				writef(w, "- `%s`\n", g.Name)
			}
			writeln(w, "")
		}

		writeln(w, "---\n")
	}

	// Functions
	if len(doc.Functions) > 0 {
		writeln(w, "## Functions\n")

		for _, fn := range doc.Functions {
			renderFunctionMarkdown(w, fn, opts)
		}
	}

	// Globals
	if len(doc.Globals) > 0 {
		writeln(w, "## Variables\n")

		for _, g := range doc.Globals {
			writef(w, "### `%s`\n\n", g.Name)
			if g.Value != "" && g.Value != "..." {
				writef(w, "```python\n%s = %s\n```\n\n", g.Name, g.Value)
			}
		}
	}

	return nil
}

// renderFunctionMarkdown renders a single function's documentation.
func renderFunctionMarkdown(w io.Writer, fn FunctionDoc, opts MarkdownOptions) {
	// Function header with signature
	writef(w, "### %s\n\n", fn.Name)

	// Signature
	sig := buildSignature(fn)
	writef(w, "```python\n%s\n```\n\n", sig)

	// Source link
	if opts.IncludeSourceLinks && opts.SourceBaseURL != "" {
		writef(w, "*Defined at [line %d](%s#L%d)*\n\n", fn.Line, opts.SourceBaseURL, fn.Line)
	}

	// Docstring content
	if fn.Parsed != nil && fn.Parsed.HasDocumentation() {
		renderParsedDocstring(w, fn)
	} else if fn.Docstring != "" {
		writef(w, "%s\n\n", fn.Docstring)
	}

	writeln(w, "---\n")
}

// buildSignature builds a function signature string.
func buildSignature(fn FunctionDoc) string {
	var params []string
	for _, p := range fn.Params {
		if p.HasDefault {
			params = append(params, fmt.Sprintf("%s=%s", p.Name, p.Default))
		} else {
			params = append(params, p.Name)
		}
	}
	return fmt.Sprintf("def %s(%s)", fn.Name, strings.Join(params, ", "))
}

// renderParsedDocstring renders a parsed docstring with sections.
func renderParsedDocstring(w io.Writer, fn FunctionDoc) {
	p := fn.Parsed

	// Summary
	if p.Summary != "" {
		writef(w, "%s\n\n", p.Summary)
	}

	// Description
	if p.Description != "" {
		writef(w, "%s\n\n", p.Description)
	}

	// Args
	if len(p.Args) > 0 || len(fn.Params) > 0 {
		writeln(w, "**Arguments:**\n")
		writeln(w, "| Name | Description |")
		writeln(w, "|------|-------------|")

		// Get all param names in order
		paramNames := make([]string, 0, len(fn.Params))
		for _, param := range fn.Params {
			paramNames = append(paramNames, param.Name)
		}

		// Render params that exist in function signature
		for _, name := range paramNames {
			desc := p.Args[name]
			if desc == "" {
				desc = "*No description*"
			}
			// Find default value
			var defaultStr string
			for _, param := range fn.Params {
				if param.Name == name && param.HasDefault {
					defaultStr = fmt.Sprintf(" (default: `%s`)", param.Default)
					break
				}
			}
			writef(w, "| `%s` | %s%s |\n", name, desc, defaultStr)
		}

		// Render any documented params not in signature (rare but possible)
		for name, desc := range p.Args {
			found := false
			for _, pn := range paramNames {
				if pn == name {
					found = true
					break
				}
			}
			if !found {
				writef(w, "| `%s` | %s |\n", name, desc)
			}
		}

		writeln(w, "")
	}

	// Returns
	if p.Returns != "" {
		writeln(w, "**Returns:**\n")
		writef(w, "%s\n\n", p.Returns)
	}

	// Raises
	if len(p.Raises) > 0 {
		writeln(w, "**Raises:**\n")
		// Sort for consistent output
		var names []string
		for name := range p.Raises {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			writef(w, "- `%s`: %s\n", name, p.Raises[name])
		}
		writeln(w, "")
	}

	// Example
	if p.Example != "" {
		writeln(w, "**Example:**\n")
		writef(w, "```python\n%s\n```\n\n", p.Example)
	}

	// Note
	if p.Note != "" {
		writeln(w, "**Note:**\n")
		writef(w, "> %s\n\n", strings.ReplaceAll(p.Note, "\n", "\n> "))
	}
}

// toAnchor converts a name to a markdown anchor.
func toAnchor(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, s string) {
	_, _ = fmt.Fprintln(w, s)
}
