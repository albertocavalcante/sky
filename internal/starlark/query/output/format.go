// Package output provides output formatting for skyquery results.
package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/albertocavalcante/sky/internal/starlark/sortutil"
)

// Format defines the output format for query results.
type Format string

// Supported output formats.
const (
	FormatName     Format = "name"
	FormatLocation Format = "location"
	FormatJSON     Format = "json"
	FormatCount    Format = "count"
)

// ParseFormat parses a format string into a Format.
// Returns an error if the format is not recognized.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "name", "":
		return FormatName, nil
	case "location":
		return FormatLocation, nil
	case "json":
		return FormatJSON, nil
	case "count":
		return FormatCount, nil
	default:
		return "", fmt.Errorf("unknown output format: %q (valid: name, location, json, count)", s)
	}
}

// Result represents a query result that can be formatted.
// This is an interface to decouple from the query package's specific types.
type Result interface {
	// Query returns the original query string.
	Query() string
	// Items returns the result items.
	Items() []Item
}

// Item represents a single result item.
type Item interface {
	// Type returns the item type (e.g., "def", "load", "call", "file", "assign").
	Type() string
	// Name returns the item's name.
	Name() string
	// File returns the file path.
	File() string
	// Line returns the line number.
	Line() int
}

// DefItem represents a function definition result.
type DefItem interface {
	Item
	// Params returns parameter names.
	Params() []string
	// Docstring returns the function docstring.
	Docstring() string
}

// LoadItem represents a load statement result.
type LoadItem interface {
	Item
	// Module returns the loaded module path.
	Module() string
	// Symbols returns the imported symbols (local name -> exported name).
	Symbols() map[string]string
}

// CallItem represents a function call result.
type CallItem interface {
	Item
	// Function returns the called function name.
	Function() string
}

// AssignItem represents an assignment result.
type AssignItem interface {
	Item
}

// Formatter formats query results for output.
type Formatter struct {
	format Format
}

// NewFormatter creates a formatter for the given format string.
// If the format is invalid, it defaults to "name" format.
func NewFormatter(format string) *Formatter {
	f, err := ParseFormat(format)
	if err != nil {
		f = FormatName
	}
	return &Formatter{format: f}
}

// NewFormatterWithFormat creates a formatter with a pre-parsed format.
func NewFormatterWithFormat(format Format) *Formatter {
	return &Formatter{format: format}
}

// Format returns the current format setting.
func (f *Formatter) Format() Format {
	return f.format
}

// Write writes formatted results to the writer.
func (f *Formatter) Write(w io.Writer, result Result) error {
	switch f.format {
	case FormatName:
		return f.formatName(w, result)
	case FormatLocation:
		return f.formatLocation(w, result)
	case FormatJSON:
		return f.formatJSON(w, result)
	case FormatCount:
		return f.formatCount(w, result)
	default:
		return f.formatName(w, result)
	}
}

// formatName outputs just names, one per line.
func (f *Formatter) formatName(w io.Writer, result Result) error {
	items := result.Items()
	// Sort by file, then line, then name for deterministic output
	sorted := make([]Item, len(items))
	copy(sorted, items)
	sortutil.ByFileLineName(sorted,
		func(i Item) string { return i.File() },
		func(i Item) int { return i.Line() },
		func(i Item) string { return i.Name() },
	)

	for _, item := range sorted {
		if _, err := fmt.Fprintln(w, item.Name()); err != nil {
			return err
		}
	}
	return nil
}

// formatLocation outputs "file:line: name" format.
func (f *Formatter) formatLocation(w io.Writer, result Result) error {
	items := result.Items()
	// Sort by file, then line
	sorted := make([]Item, len(items))
	copy(sorted, items)
	sortutil.ByFileLine(sorted,
		func(i Item) string { return i.File() },
		func(i Item) int { return i.Line() },
	)

	for _, item := range sorted {
		if _, err := fmt.Fprintf(w, "//%s:%d: %s\n", item.File(), item.Line(), item.Name()); err != nil {
			return err
		}
	}
	return nil
}

// jsonOutput represents the root JSON structure.
type jsonOutput struct {
	Query   string       `json:"query"`
	Count   int          `json:"count"`
	Results []jsonResult `json:"results"`
}

// jsonResult represents a single result item in JSON.
type jsonResult struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	File      string            `json:"file"`
	Line      int               `json:"line"`
	Params    []string          `json:"params,omitempty"`
	Docstring string            `json:"docstring,omitempty"`
	Module    string            `json:"module,omitempty"`
	Symbols   map[string]string `json:"symbols,omitempty"`
	Function  string            `json:"function,omitempty"`
}

// formatJSON outputs results as JSON.
func (f *Formatter) formatJSON(w io.Writer, result Result) error {
	items := result.Items()

	// Sort for deterministic output
	sorted := make([]Item, len(items))
	copy(sorted, items)
	sortutil.ByFileLineName(sorted,
		func(i Item) string { return i.File() },
		func(i Item) int { return i.Line() },
		func(i Item) string { return i.Name() },
	)

	output := jsonOutput{
		Query:   result.Query(),
		Count:   len(sorted),
		Results: make([]jsonResult, 0, len(sorted)),
	}

	for _, item := range sorted {
		jr := jsonResult{
			Type: item.Type(),
			Name: item.Name(),
			File: item.File(),
			Line: item.Line(),
		}

		// Add type-specific fields
		if def, ok := item.(DefItem); ok {
			jr.Params = def.Params()
			if doc := def.Docstring(); doc != "" {
				jr.Docstring = doc
			}
		}
		if load, ok := item.(LoadItem); ok {
			jr.Module = load.Module()
			jr.Symbols = load.Symbols()
		}
		if call, ok := item.(CallItem); ok {
			jr.Function = call.Function()
		}

		output.Results = append(output.Results, jr)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// formatCount outputs just the count.
func (f *Formatter) formatCount(w io.Writer, result Result) error {
	_, err := fmt.Fprintln(w, len(result.Items()))
	return err
}

// SimpleResult is a basic implementation of Result for testing and simple use cases.
type SimpleResult struct {
	QueryStr    string
	ResultItems []Item
}

// Query returns the query string.
func (r *SimpleResult) Query() string {
	return r.QueryStr
}

// Items returns the result items.
func (r *SimpleResult) Items() []Item {
	return r.ResultItems
}

// SimpleItem is a basic implementation of Item.
type SimpleItem struct {
	ItemType string
	ItemName string
	ItemFile string
	ItemLine int
}

// Type returns the item type.
func (i *SimpleItem) Type() string { return i.ItemType }

// Name returns the item name.
func (i *SimpleItem) Name() string { return i.ItemName }

// File returns the file path.
func (i *SimpleItem) File() string { return i.ItemFile }

// Line returns the line number.
func (i *SimpleItem) Line() int { return i.ItemLine }

// SimpleDef is an implementation of DefItem for testing.
type SimpleDef struct {
	SimpleItem
	ParamNames []string
	Doc        string
}

// Params returns parameter names.
func (d *SimpleDef) Params() []string { return d.ParamNames }

// Docstring returns the docstring.
func (d *SimpleDef) Docstring() string { return d.Doc }

// SimpleLoad is an implementation of LoadItem for testing.
type SimpleLoad struct {
	SimpleItem
	ModulePath      string
	ImportedSymbols map[string]string
}

// Module returns the module path.
func (l *SimpleLoad) Module() string { return l.ModulePath }

// Symbols returns the imported symbols.
func (l *SimpleLoad) Symbols() map[string]string { return l.ImportedSymbols }

// SimpleCall is an implementation of CallItem for testing.
type SimpleCall struct {
	SimpleItem
	FunctionName string
}

// Function returns the function name.
func (c *SimpleCall) Function() string { return c.FunctionName }
