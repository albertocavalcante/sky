# LSP Improvement Plan for skyls

**Date:** 2026-02-04
**Status:** Draft
**Author:** Generated from codebase analysis

## Executive Summary

The Sky Language Server (skyls) has a solid foundation with working completion, hover, definition, symbols, formatting, and diagnostics. However, there are significant opportunities to improve the developer experience through additional LSP features and better integration with our existing builtins infrastructure.

This document outlines a comprehensive improvement plan with prioritized features, TDD test cases, and implementation guidance.

---

## Current State Analysis

### Implemented Capabilities

| Capability                        | Implementation | Quality | Notes                                         |
| --------------------------------- | -------------- | ------- | --------------------------------------------- |
| `textDocument/completion`         | ✅ Full        | Good    | Keywords, builtins, document symbols, modules |
| `textDocument/hover`              | ✅ Full        | Good    | Docstrings, builtins via Provider             |
| `textDocument/definition`         | ⚠️ Partial      | Limited | **File-local only** - cannot follow load()    |
| `textDocument/documentSymbol`     | ✅ Full        | Good    | Functions, globals                            |
| `textDocument/formatting`         | ✅ Full        | Good    | Uses buildtools formatter                     |
| `textDocument/publishDiagnostics` | ✅ Full        | Good    | Linter + semantic checker                     |

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        LSP Server                           │
├─────────────────────────────────────────────────────────────┤
│  Server struct                                              │
│  ├── conn *Conn              // JSON-RPC connection         │
│  ├── documents map[URI]*Doc  // Open documents              │
│  ├── lintDriver *Driver      // Linter rules                │
│  ├── checker *Checker        // Semantic analysis           │
│  └── builtins Provider       // Completion/hover source     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Builtins Infrastructure                  │
├─────────────────────────────────────────────────────────────┤
│  Provider interface                                         │
│  ├── JSONProvider    // Human-friendly JSON files           │
│  ├── ProtoProvider   // Efficient protobuf data             │
│  └── ChainProvider   // Merges multiple providers           │
└─────────────────────────────────────────────────────────────┘
```

### Critical Gap: Provider Not Initialized

```go
// Current state in NewServer():
func NewServer(onExit func()) *Server {
    return NewServerWithProvider(onExit, nil)  // ← Always nil!
}
```

The builtins.Provider infrastructure is wired but **never initialized with real data**. This means:

- Completion falls back to hardcoded lists
- Hover for builtins uses hardcoded data
- No dialect-specific builtins

---

## Improvement Categories

### Category A: Foundation (Must Have)

These improvements unlock other features and fix architectural gaps.

### Category B: High-Impact Features

Highly visible improvements that users interact with constantly.

### Category C: Developer Experience

Quality-of-life improvements that make the LSP feel polished.

### Category D: Advanced Features

Complex features requiring significant infrastructure.

---

## Detailed Improvement Specifications

### A1: Initialize Builtins Provider at Startup

**Priority:** 🔴 Critical
**Effort:** 1 day
**Category:** Foundation

#### Problem

The `builtins.Provider` is wired to completion and hover but never loaded with real data.

#### Solution

Initialize a `ChainProvider` with Proto and JSON providers during `handleInitialize`:

```go
func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
    // ... existing code ...

    // Initialize builtins provider
    protoProvider := loader.NewProtoProvider()
    jsonProvider := loader.NewJSONProvider()
    s.builtins = builtins.NewChainProvider(protoProvider, jsonProvider)

    // ... return capabilities ...
}
```

#### Test Cases (TDD)

```go
func TestServer_InitializesBuiltinsProvider(t *testing.T) {
    server := NewServer(nil)

    // Simulate initialize request
    _, err := server.Handle(ctx, &Request{
        Method: "initialize",
        Params: json.RawMessage(`{"capabilities":{}}`),
    })
    require.NoError(t, err)

    // Provider should be initialized
    assert.NotNil(t, server.builtins)

    // Should have Bazel builtins
    b, err := server.builtins.Builtins("bazel", filekind.KindBuild)
    require.NoError(t, err)
    assert.True(t, len(b.Functions) > 0, "should have Bazel functions")
}

func TestCompletion_UsesBazelBuiltins_InBUILDFile(t *testing.T) {
    server := setupInitializedServer(t)

    // Open a BUILD file
    server.handleDidOpen(ctx, didOpenParams("file:///BUILD", ""))

    // Complete "cc_"
    result := getCompletions(server, "file:///BUILD", "cc_")

    // Should include cc_library, cc_binary from Bazel builtins
    labels := extractLabels(result.Items)
    assert.Contains(t, labels, "cc_library")
    assert.Contains(t, labels, "cc_binary")
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add provider initialization in handleInitialize

#### Acceptance Criteria

- [ ] Provider initialized during initialize handshake
- [ ] Bazel builtins available for BUILD files
- [ ] Starlark builtins available for .star files
- [ ] Existing tests still pass

---

### A2: Config File Loading

**Priority:** 🔴 Critical
**Effort:** 2-3 days
**Category:** Foundation
**Depends on:** A1

#### Problem

No way to configure which dialect applies to which files. All files treated the same.

#### Solution

Implement `.starlark/config.json` discovery and parsing per RFC-dialect-support.md:

```go
// internal/starlark/config/config.go
package config

type Config struct {
    Version  int                `json:"version"`
    Dialect  string             `json:"dialect,omitempty"`
    Rules    []Rule             `json:"rules,omitempty"`
    Dialects map[string]Dialect `json:"dialects,omitempty"`
    Settings Settings           `json:"settings,omitempty"`
}

type Rule struct {
    Files   []string `json:"files"`
    Dialect string   `json:"dialect"`
}

type Dialect struct {
    Builtins []string `json:"builtins"`
    Extends  string   `json:"extends,omitempty"`
}

func Discover(workspaceRoot string) (*Config, error)
func (c *Config) DialectForFile(path string) string
```

#### Test Cases (TDD)

```go
func TestConfigDiscovery_FindsStarlarkConfig(t *testing.T) {
    root := t.TempDir()
    configPath := filepath.Join(root, ".starlark", "config.json")
    os.MkdirAll(filepath.Dir(configPath), 0755)
    os.WriteFile(configPath, []byte(`{"version": 1, "dialect": "bazel"}`), 0644)

    cfg, err := config.Discover(root)
    require.NoError(t, err)
    assert.Equal(t, "bazel", cfg.Dialect)
}

func TestConfigDiscovery_WalksUpDirectories(t *testing.T) {
    root := t.TempDir()
    subdir := filepath.Join(root, "pkg", "sub")
    os.MkdirAll(subdir, 0755)

    configPath := filepath.Join(root, ".starlark", "config.json")
    os.MkdirAll(filepath.Dir(configPath), 0755)
    os.WriteFile(configPath, []byte(`{"version": 1}`), 0644)

    cfg, err := config.DiscoverFrom(filepath.Join(subdir, "test.star"))
    require.NoError(t, err)
    assert.NotNil(t, cfg)
}

func TestConfig_DialectForFile_MatchesRules(t *testing.T) {
    cfg := &config.Config{
        Dialect: "starlark",
        Rules: []config.Rule{
            {Files: []string{"Tiltfile"}, Dialect: "tilt"},
            {Files: []string{"**/*.bzl"}, Dialect: "bazel-bzl"},
            {Files: []string{"BUILD", "**/BUILD"}, Dialect: "bazel-build"},
        },
    }

    tests := []struct {
        path string
        want string
    }{
        {"Tiltfile", "tilt"},
        {"pkg/rules.bzl", "bazel-bzl"},
        {"BUILD", "bazel-build"},
        {"pkg/BUILD", "bazel-build"},
        {"script.star", "starlark"}, // default
    }

    for _, tt := range tests {
        t.Run(tt.path, func(t *testing.T) {
            got := cfg.DialectForFile(tt.path)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

#### Files to Create/Modify

- `internal/starlark/config/config.go` (new)
- `internal/starlark/config/config_test.go` (new)
- `internal/starlark/config/discover.go` (new)
- `internal/starlark/config/discover_test.go` (new)
- `internal/lsp/server.go` - Load config in handleInitialize

#### Acceptance Criteria

- [ ] Discovers `.starlark/config.json` from workspace root
- [ ] Walks up directories to find config
- [ ] Parses rules and matches files to dialects
- [ ] Falls back to auto-detection if no config
- [ ] Exposes dialect for use by LSP handlers

---

### A3: Dialect-Aware Diagnostics

**Priority:** 🟡 High
**Effort:** 1-2 days
**Category:** Foundation
**Depends on:** A1, A2

#### Problem

The semantic checker reports "undefined" errors for dialect-specific builtins:

- `undefined: cc_library` in BUILD files
- `undefined: docker_build` in Tiltfiles

#### Solution

Pass the current dialect's predeclared names to the checker:

```go
func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
    // Get dialect for this file
    dialect := s.config.DialectForFile(uriToPath(uri))
    kind := s.classifier.Classify(uriToPath(uri))

    // Get predeclared names from builtins
    builtinsData, _ := s.builtins.Builtins(dialect, kind)
    predeclared := extractPredeclaredNames(builtinsData)

    // Run checker with predeclared names
    diags, _ := s.checker.CheckWithPredeclared(path, content, predeclared)

    // ... publish diagnostics ...
}

func extractPredeclaredNames(b *builtins.Builtins) map[string]bool {
    names := make(map[string]bool)
    for _, fn := range b.Functions {
        names[fn.Name] = true
    }
    for _, g := range b.Globals {
        names[g.Name] = true
    }
    for _, t := range b.Types {
        names[t.Name] = true
    }
    return names
}
```

#### Test Cases (TDD)

```go
func TestDiagnostics_NoFalsePositivesForBazelBuiltins(t *testing.T) {
    server := setupServerWithBazelConfig(t)

    uri := "file:///BUILD"
    content := `cc_library(
        name = "foo",
        srcs = glob(["*.cc"]),
    )`

    server.handleDidOpen(ctx, didOpenParams(uri, content))
    diags := collectDiagnostics(server, uri)

    // Should NOT report undefined for cc_library, glob
    for _, d := range diags {
        assert.NotContains(t, d.Message, "undefined: cc_library")
        assert.NotContains(t, d.Message, "undefined: glob")
    }
}

func TestDiagnostics_ReportsActualUndefined(t *testing.T) {
    server := setupServerWithBazelConfig(t)

    uri := "file:///BUILD"
    content := `nonexistent_rule(name = "foo")`

    server.handleDidOpen(ctx, didOpenParams(uri, content))
    diags := collectDiagnostics(server, uri)

    // Should report undefined for nonexistent_rule
    hasUndefined := false
    for _, d := range diags {
        if strings.Contains(d.Message, "undefined: nonexistent_rule") {
            hasUndefined = true
        }
    }
    assert.True(t, hasUndefined)
}
```

#### Files to Modify

- `internal/starlark/checker/checker.go` - Add CheckWithPredeclared method
- `internal/lsp/server.go` - Pass predeclared to checker

#### Acceptance Criteria

- [ ] No false positives for Bazel builtins in BUILD files
- [ ] No false positives for Tilt builtins in Tiltfiles
- [ ] Still reports actually undefined names
- [ ] Works with custom dialects from config

---

### B1: Signature Help (Parameter Hints)

**Priority:** 🟡 High
**Effort:** 1-2 days
**Category:** High-Impact

#### Problem

When typing function calls, users don't see parameter information:

```starlark
docker_build(|)  # What parameters does this take?
```

#### Solution

Implement `textDocument/signatureHelp`:

```go
func (s *Server) handleSignatureHelp(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.SignatureHelpParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    // Get document content
    doc := s.getDocument(p.TextDocument.URI)
    if doc == nil {
        return nil, nil
    }

    // Find the function call context
    callCtx := findCallContext(doc.Content, int(p.Position.Line), int(p.Position.Character))
    if callCtx == nil {
        return nil, nil
    }

    // Look up function signature
    sig := s.getFunctionSignature(callCtx.FunctionName, p.TextDocument.URI)
    if sig == nil {
        return nil, nil
    }

    return &protocol.SignatureHelp{
        Signatures: []protocol.SignatureInformation{*sig},
        ActiveSignature: 0,
        ActiveParameter: uint32(callCtx.ArgumentIndex),
    }, nil
}

type callContext struct {
    FunctionName  string
    ArgumentIndex int
}

func findCallContext(content string, line, char int) *callContext {
    // Parse backwards from cursor to find opening paren
    // Count commas to determine argument index
    // Extract function name before the paren
}
```

#### Test Cases (TDD)

```go
func TestSignatureHelp_BuiltinFunction(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `print(`
    //              ^ cursor here (after open paren)

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    result := getSignatureHelp(server, uri, 0, 6)

    require.NotNil(t, result)
    require.Len(t, result.Signatures, 1)

    sig := result.Signatures[0]
    assert.Contains(t, sig.Label, "print")
    assert.True(t, len(sig.Parameters) > 0)
}

func TestSignatureHelp_ActiveParameter(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `dict(pairs, `
    //                     ^ cursor after comma (2nd param)

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    result := getSignatureHelp(server, uri, 0, 12)

    assert.Equal(t, uint32(1), result.ActiveParameter)
}

func TestSignatureHelp_NestedCalls(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `print(len(`
    //                   ^ cursor inside inner call

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    result := getSignatureHelp(server, uri, 0, 10)

    // Should show len() signature, not print()
    assert.Contains(t, result.Signatures[0].Label, "len")
}

func TestSignatureHelp_UserDefinedFunction(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `def my_func(name, value=None):
    """Do something."""
    pass

my_func(`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    result := getSignatureHelp(server, uri, 4, 8)

    require.NotNil(t, result)
    assert.Contains(t, result.Signatures[0].Label, "my_func")
    assert.Contains(t, result.Signatures[0].Documentation.Value, "Do something")
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleSignatureHelp, register capability
- `internal/lsp/signature.go` (new) - Signature help logic

#### Acceptance Criteria

- [ ] Shows signature for builtin functions
- [ ] Shows signature for user-defined functions
- [ ] Highlights active parameter based on cursor position
- [ ] Handles nested function calls correctly
- [ ] Triggers on `(` and `,`

---

### B2: Document Links for load() Statements

**Priority:** 🟡 High
**Effort:** 2 days
**Category:** High-Impact

#### Problem

`load()` statements are not clickable - users can't navigate to imported files:

```starlark
load("//pkg:rules.bzl", "my_rule")  # Can't Ctrl+Click to open
```

#### Solution

Implement `textDocument/documentLink`:

```go
func (s *Server) handleDocumentLink(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.DocumentLinkParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    doc := s.getDocument(p.TextDocument.URI)
    if doc == nil {
        return nil, nil
    }

    // Parse the file to find load statements
    file, err := build.ParseDefault(uriToPath(p.TextDocument.URI), []byte(doc.Content))
    if err != nil {
        return nil, nil
    }

    var links []protocol.DocumentLink
    for _, stmt := range file.Stmt {
        load, ok := stmt.(*build.LoadStmt)
        if !ok {
            continue
        }

        // Resolve the load path to a file URI
        targetURI := s.resolveLoadPath(load.Module.Value, p.TextDocument.URI)
        if targetURI == "" {
            continue
        }

        // Create link for the module string
        links = append(links, protocol.DocumentLink{
            Range:  tokenToRange(load.Module),
            Target: targetURI,
        })
    }

    return links, nil
}

func (s *Server) resolveLoadPath(module string, fromURI protocol.DocumentURI) string {
    // Handle different load path formats:
    // - "//pkg:file.bzl" - workspace-relative
    // - ":file.bzl" - package-relative
    // - "@repo//pkg:file.bzl" - external repo
    // - "/path/to/file.bzl" - absolute (rare)
}
```

#### Test Cases (TDD)

```go
func TestDocumentLink_LoadStatement(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///workspace/pkg/BUILD"
    content := `load("//lib:rules.bzl", "my_rule")`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    links := getDocumentLinks(server, uri)

    require.Len(t, links, 1)
    assert.Equal(t, "file:///workspace/lib/rules.bzl", links[0].Target)
    assert.Equal(t, uint32(0), links[0].Range.Start.Line)
    assert.Equal(t, uint32(5), links[0].Range.Start.Character) // Start of string
}

func TestDocumentLink_RelativeLoad(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///workspace/pkg/BUILD"
    content := `load(":helpers.bzl", "helper")`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    links := getDocumentLinks(server, uri)

    require.Len(t, links, 1)
    assert.Equal(t, "file:///workspace/pkg/helpers.bzl", links[0].Target)
}

func TestDocumentLink_MultipleLoads(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///workspace/BUILD"
    content := `load("//a:a.bzl", "a")
load("//b:b.bzl", "b")`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    links := getDocumentLinks(server, uri)

    assert.Len(t, links, 2)
}

func TestDocumentLink_ExternalRepo_NoLink(t *testing.T) {
    // External repos can't be resolved without WORKSPACE info
    server := setupInitializedServer(t)

    uri := "file:///workspace/BUILD"
    content := `load("@rules_go//go:def.bzl", "go_library")`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    links := getDocumentLinks(server, uri)

    // External repos not resolvable - no link
    assert.Len(t, links, 0)
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleDocumentLink, register capability
- `internal/lsp/links.go` (new) - Link resolution logic

#### Acceptance Criteria

- [ ] Workspace-relative loads (`//pkg:file.bzl`) become clickable
- [ ] Package-relative loads (`:file.bzl`) become clickable
- [ ] External repo loads gracefully ignored (no broken links)
- [ ] Multiple load statements all get links
- [ ] Works in BUILD files and .bzl files

---

### B3: Cross-File Go-to-Definition

**Priority:** 🟡 High
**Effort:** 3-4 days
**Category:** High-Impact
**Depends on:** B2

#### Problem

Go-to-definition only works within the current file:

```starlark
load("//pkg:rules.bzl", "my_rule")
my_rule(...)  # Ctrl+Click doesn't jump to definition
```

#### Solution

Extend definition handler to resolve imports:

```go
func (s *Server) handleDefinition(ctx context.Context, params json.RawMessage) (any, error) {
    // ... existing code to find word ...

    // Check if it's an imported symbol
    if importInfo := s.findImport(doc, word); importInfo != nil {
        // Load the target file
        targetPath := s.resolveLoadPath(importInfo.Module, uri)
        targetContent, err := os.ReadFile(targetPath)
        if err != nil {
            return nil, nil
        }

        // Parse and find the symbol definition
        targetDoc, _ := docgen.ExtractFile(targetPath, targetContent, docgen.Options{})
        for _, fn := range targetDoc.Functions {
            if fn.Name == word {
                return &protocol.Location{
                    URI:   protocol.DocumentURI("file://" + targetPath),
                    Range: posToRange(fn.Pos),
                }, nil
            }
        }
    }

    // ... existing code for local definitions ...
}

type importInfo struct {
    Module string   // "//pkg:rules.bzl"
    Names  []string // ["my_rule", "other"]
}

func (s *Server) findImport(doc *Document, symbol string) *importInfo {
    // Parse load statements, check if symbol is imported
}
```

#### Test Cases (TDD)

```go
func TestDefinition_ImportedSymbol(t *testing.T) {
    server := setupInitializedServer(t)
    root := t.TempDir()

    // Create rules.bzl with my_rule
    rulesPath := filepath.Join(root, "pkg", "rules.bzl")
    os.MkdirAll(filepath.Dir(rulesPath), 0755)
    os.WriteFile(rulesPath, []byte(`def my_rule(name):
    """A custom rule."""
    pass
`), 0644)

    // Create BUILD that imports my_rule
    buildPath := filepath.Join(root, "BUILD")
    buildContent := `load("//pkg:rules.bzl", "my_rule")
my_rule(name = "foo")`

    uri := "file://" + buildPath
    server.handleDidOpen(ctx, didOpenParams(uri, buildContent))

    // Go to definition of "my_rule" on line 2
    result := getDefinition(server, uri, 1, 0)

    require.NotNil(t, result)
    loc := result.(protocol.Location)
    assert.Equal(t, "file://"+rulesPath, string(loc.URI))
    assert.Equal(t, uint32(0), loc.Range.Start.Line) // def my_rule is on line 0
}

func TestDefinition_LocalSymbolStillWorks(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `def helper():
    pass

def main():
    helper()  # Should jump to helper above
`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    // Go to definition of "helper" on line 5
    result := getDefinition(server, uri, 4, 4)

    require.NotNil(t, result)
    loc := result.(protocol.Location)
    assert.Equal(t, uri, string(loc.URI))
    assert.Equal(t, uint32(0), loc.Range.Start.Line)
}
```

#### Files to Modify

- `internal/lsp/server.go` - Extend handleDefinition
- `internal/lsp/imports.go` (new) - Import resolution

#### Acceptance Criteria

- [ ] Ctrl+Click on imported symbol jumps to definition
- [ ] Local definitions still work
- [ ] Works across multiple files in load chain
- [ ] Handles symbols that don't exist gracefully

---

### B4: Find References

**Priority:** 🟢 Medium
**Effort:** 2-3 days
**Category:** High-Impact

#### Problem

Can't find all usages of a symbol across the workspace.

#### Solution

Implement `textDocument/references`:

```go
func (s *Server) handleReferences(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.ReferenceParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    // Get the symbol at cursor
    doc := s.getDocument(p.TextDocument.URI)
    word := getWordAtPosition(doc.Content, int(p.Position.Line), int(p.Position.Character))

    var locations []protocol.Location

    // Search all open documents (or all workspace files)
    for uri, doc := range s.documents {
        refs := findReferencesInFile(doc.Content, word)
        for _, ref := range refs {
            locations = append(locations, protocol.Location{
                URI:   uri,
                Range: ref,
            })
        }
    }

    // Optionally include declaration
    if p.Context.IncludeDeclaration {
        // Add the definition location
    }

    return locations, nil
}
```

#### Test Cases (TDD)

```go
func TestReferences_FindsAllUsages(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `def helper():
    pass

def main():
    helper()
    helper()
`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    // Find references to "helper"
    refs := getReferences(server, uri, 0, 4, true) // include declaration

    assert.Len(t, refs, 3) // 1 definition + 2 usages
}

func TestReferences_AcrossFiles(t *testing.T) {
    server := setupInitializedServer(t)

    // Open two files that share a symbol
    server.handleDidOpen(ctx, didOpenParams("file:///a.star", `MY_CONST = 42`))
    server.handleDidOpen(ctx, didOpenParams("file:///b.star", `load(":a.star", "MY_CONST")
print(MY_CONST)`))

    refs := getReferences(server, "file:///a.star", 0, 0, true)

    assert.Len(t, refs, 3) // definition + import + usage
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleReferences, register capability
- `internal/lsp/references.go` (new) - Reference finding logic

#### Acceptance Criteria

- [ ] Finds all usages in current file
- [ ] Finds usages across open documents
- [ ] Optionally includes declaration
- [ ] Handles symbols that span multiple lines

---

### C1: Workspace Symbols

**Priority:** 🟢 Medium
**Effort:** 2 days
**Category:** Developer Experience

#### Problem

Can't quickly navigate to any symbol in the workspace (Cmd+T / Ctrl+T).

#### Solution

Implement `workspace/symbol`:

```go
func (s *Server) handleWorkspaceSymbol(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.WorkspaceSymbolParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    query := strings.ToLower(p.Query)
    var symbols []protocol.SymbolInformation

    // Search all .star/.bzl files in workspace
    filepath.WalkDir(s.rootURI, func(path string, d fs.DirEntry, err error) error {
        if !isStarlarkFile(path) {
            return nil
        }

        content, _ := os.ReadFile(path)
        doc, _ := docgen.ExtractFile(path, content, docgen.Options{})

        for _, fn := range doc.Functions {
            if strings.Contains(strings.ToLower(fn.Name), query) {
                symbols = append(symbols, protocol.SymbolInformation{
                    Name:     fn.Name,
                    Kind:     protocol.SymbolKindFunction,
                    Location: protocol.Location{URI: pathToURI(path), Range: posToRange(fn.Pos)},
                })
            }
        }

        return nil
    })

    return symbols, nil
}
```

#### Test Cases (TDD)

```go
func TestWorkspaceSymbol_FindsMatchingSymbols(t *testing.T) {
    server := setupWorkspaceServer(t, map[string]string{
        "a.star": "def alpha():\n    pass",
        "b.star": "def beta():\n    pass",
        "c.star": "def gamma():\n    pass",
    })

    symbols := getWorkspaceSymbols(server, "alph")

    require.Len(t, symbols, 1)
    assert.Equal(t, "alpha", symbols[0].Name)
}

func TestWorkspaceSymbol_FuzzyMatch(t *testing.T) {
    server := setupWorkspaceServer(t, map[string]string{
        "rules.star": "def my_custom_rule():\n    pass",
    })

    symbols := getWorkspaceSymbols(server, "mcr") // fuzzy for my_custom_rule

    // Should find my_custom_rule with fuzzy matching
    require.Len(t, symbols, 1)
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleWorkspaceSymbol, register capability
- `internal/lsp/symbols.go` (new) - Symbol search logic

#### Acceptance Criteria

- [ ] Finds functions matching query
- [ ] Finds globals matching query
- [ ] Searches all Starlark files in workspace
- [ ] Fuzzy matching supported
- [ ] Caches results for performance

---

### C2: Folding Ranges

**Priority:** 🟢 Medium
**Effort:** 1 day
**Category:** Developer Experience

#### Problem

Can't collapse functions or blocks in the editor.

#### Solution

Implement `textDocument/foldingRange`:

```go
func (s *Server) handleFoldingRange(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.FoldingRangeParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    doc := s.getDocument(p.TextDocument.URI)
    if doc == nil {
        return nil, nil
    }

    file, err := build.ParseDefault(uriToPath(p.TextDocument.URI), []byte(doc.Content))
    if err != nil {
        return nil, nil
    }

    var ranges []protocol.FoldingRange

    for _, stmt := range file.Stmt {
        switch s := stmt.(type) {
        case *build.DefStmt:
            ranges = append(ranges, protocol.FoldingRange{
                StartLine:      uint32(s.Def.Line - 1),
                EndLine:        uint32(s.End.Line - 1),
                Kind:           protocol.FoldingRangeKindRegion,
            })
        case *build.ForStmt:
            ranges = append(ranges, protocol.FoldingRange{
                StartLine:      uint32(s.For.Line - 1),
                EndLine:        uint32(s.End.Line - 1),
                Kind:           protocol.FoldingRangeKindRegion,
            })
        case *build.IfStmt:
            ranges = append(ranges, protocol.FoldingRange{
                StartLine:      uint32(s.If.Line - 1),
                EndLine:        uint32(s.End.Line - 1),
                Kind:           protocol.FoldingRangeKindRegion,
            })
        }
    }

    return ranges, nil
}
```

#### Test Cases (TDD)

```go
func TestFoldingRange_Functions(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `def foo():
    x = 1
    y = 2
    return x + y

def bar():
    pass
`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    ranges := getFoldingRanges(server, uri)

    require.Len(t, ranges, 2)
    assert.Equal(t, uint32(0), ranges[0].StartLine) // foo starts line 0
    assert.Equal(t, uint32(3), ranges[0].EndLine)   // foo ends line 3
}

func TestFoldingRange_NestedBlocks(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `def foo():
    if True:
        for x in [1,2]:
            print(x)
`

    server.handleDidOpen(ctx, didOpenParams(uri, content))

    ranges := getFoldingRanges(server, uri)

    // Should have ranges for: def, if, for
    assert.Len(t, ranges, 3)
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleFoldingRange, register capability

#### Acceptance Criteria

- [ ] Functions are foldable
- [ ] If statements are foldable
- [ ] For loops are foldable
- [ ] Nested blocks work correctly

---

### C3: Code Actions (Quick Fixes)

**Priority:** 🟢 Medium
**Effort:** 2-3 days
**Category:** Developer Experience

#### Problem

Linter warnings don't have quick fixes:

```starlark
x = 1  # Warning: unused variable
       # No "remove unused variable" action
```

#### Solution

Implement `textDocument/codeAction`:

```go
func (s *Server) handleCodeAction(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.CodeActionParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    var actions []protocol.CodeAction

    // Generate actions based on diagnostics in range
    for _, diag := range p.Context.Diagnostics {
        switch {
        case strings.Contains(diag.Message, "unused variable"):
            actions = append(actions, protocol.CodeAction{
                Title:       "Remove unused variable",
                Kind:        protocol.CodeActionKindQuickFix,
                Diagnostics: []protocol.Diagnostic{diag},
                Edit: &protocol.WorkspaceEdit{
                    Changes: map[protocol.DocumentURI][]protocol.TextEdit{
                        p.TextDocument.URI: {
                            {Range: diag.Range, NewText: ""},
                        },
                    },
                },
            })
        case strings.Contains(diag.Message, "undefined"):
            // Suggest adding a load statement
            // ...
        }
    }

    return actions, nil
}
```

#### Test Cases (TDD)

```go
func TestCodeAction_RemoveUnusedVariable(t *testing.T) {
    server := setupInitializedServer(t)

    uri := "file:///test.star"
    content := `x = 1
y = 2
print(y)
`

    server.handleDidOpen(ctx, didOpenParams(uri, content))
    diags := collectDiagnostics(server, uri)

    // Should have diagnostic for unused x
    require.Len(t, diags, 1)

    actions := getCodeActions(server, uri, diags[0].Range, diags)

    require.Len(t, actions, 1)
    assert.Equal(t, "Remove unused variable", actions[0].Title)
}
```

#### Files to Modify

- `internal/lsp/server.go` - Add handleCodeAction, register capability
- `internal/lsp/actions.go` (new) - Code action logic

#### Acceptance Criteria

- [ ] Quick fix for unused variables
- [ ] Quick fix for missing imports (suggest load)
- [ ] Quick fix for formatting issues
- [ ] Actions appear inline with diagnostics

---

### D1: Semantic Tokens

**Priority:** 🔵 Low
**Effort:** 3-4 days
**Category:** Advanced

#### Problem

Syntax highlighting is based on regex, not semantic understanding:

- Can't distinguish between function calls and variable references
- Can't color user-defined types differently

#### Solution

Implement `textDocument/semanticTokens`:

```go
func (s *Server) handleSemanticTokensFull(ctx context.Context, params json.RawMessage) (any, error) {
    // Parse file into AST
    // Walk AST and emit tokens:
    // - Function definitions: function
    // - Function calls: function
    // - Parameters: parameter
    // - Variables: variable
    // - Keywords: keyword
    // - Strings: string
    // - Numbers: number
    // - Comments: comment
    // - Types: type
    // - Builtins: builtin (custom modifier)
}
```

#### Acceptance Criteria

- [ ] Functions highlighted differently from variables
- [ ] Parameters highlighted in function signatures
- [ ] Builtins have distinct highlighting
- [ ] Incremental updates for performance

---

### D2: Inlay Hints

**Priority:** 🔵 Low
**Effort:** 2-3 days
**Category:** Advanced
**Depends on:** Type inference

#### Problem

No inline type information:

```starlark
x = get_data()  # What type is x?
```

#### Solution

Implement `textDocument/inlayHint`:

```go
func (s *Server) handleInlayHint(ctx context.Context, params json.RawMessage) (any, error) {
    // For each variable assignment:
    // - Infer type from RHS
    // - Show type hint after variable name
    // For each function call:
    // - Show parameter names before arguments
}
```

This requires type inference which is complex for Starlark.

#### Acceptance Criteria

- [ ] Shows parameter names in function calls
- [ ] Shows inferred types for variables (if type inference available)
- [ ] Configurable (can be disabled)

---

### D3: Rename Symbol

**Priority:** 🔵 Low
**Effort:** 3-4 days
**Category:** Advanced
**Depends on:** B4 (Find References)

#### Problem

Can't rename a symbol across the workspace.

#### Solution

Implement `textDocument/rename` and `textDocument/prepareRename`:

```go
func (s *Server) handleRename(ctx context.Context, params json.RawMessage) (any, error) {
    var p protocol.RenameParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }

    // Find all references
    refs := s.findAllReferences(p.TextDocument.URI, p.Position)

    // Create edits for each reference
    changes := make(map[protocol.DocumentURI][]protocol.TextEdit)
    for _, ref := range refs {
        changes[ref.URI] = append(changes[ref.URI], protocol.TextEdit{
            Range:   ref.Range,
            NewText: p.NewName,
        })
    }

    return &protocol.WorkspaceEdit{Changes: changes}, nil
}
```

#### Acceptance Criteria

- [ ] Renames symbol in current file
- [ ] Renames symbol across workspace
- [ ] Updates load() statements when renaming exports
- [ ] Validates new name is valid identifier

---

## Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

| Task                             | Priority    | Effort   | Depends On |
| -------------------------------- | ----------- | -------- | ---------- |
| A1: Initialize Builtins Provider | 🔴 Critical | 1 day    | -          |
| A2: Config File Loading          | 🔴 Critical | 2-3 days | A1         |
| A3: Dialect-Aware Diagnostics    | 🟡 High     | 1-2 days | A1, A2     |

**Deliverable:** LSP correctly uses dialect-specific builtins, no false positives.

### Phase 2: High-Impact Features (Week 3-4)

| Task                      | Priority | Effort   | Depends On |
| ------------------------- | -------- | -------- | ---------- |
| B1: Signature Help        | 🟡 High  | 1-2 days | A1         |
| B2: Document Links        | 🟡 High  | 2 days   | -          |
| B3: Cross-File Definition | 🟡 High  | 3-4 days | B2         |

**Deliverable:** Users can navigate codebase efficiently.

### Phase 3: Developer Experience (Week 5-6)

| Task                  | Priority  | Effort   | Depends On |
| --------------------- | --------- | -------- | ---------- |
| B4: Find References   | 🟢 Medium | 2-3 days | -          |
| C1: Workspace Symbols | 🟢 Medium | 2 days   | -          |
| C2: Folding Ranges    | 🟢 Medium | 1 day    | -          |
| C3: Code Actions      | 🟢 Medium | 2-3 days | -          |

**Deliverable:** Polished, professional LSP experience.

### Phase 4: Advanced (Future)

| Task                | Priority | Effort   | Depends On     |
| ------------------- | -------- | -------- | -------------- |
| D1: Semantic Tokens | 🔵 Low   | 3-4 days | -              |
| D2: Inlay Hints     | 🔵 Low   | 2-3 days | Type inference |
| D3: Rename Symbol   | 🔵 Low   | 3-4 days | B4             |

**Deliverable:** Feature parity with mature LSPs.

---

## Success Metrics

### Code Quality

- [ ] 80%+ test coverage for new code
- [ ] All existing tests pass
- [ ] No performance regressions (latency < 100ms for completion)

### User Experience

- [ ] Zero false positive diagnostics for known dialects
- [ ] Ctrl+Click navigation works for 90%+ of load statements
- [ ] Signature help appears within 50ms

### Compatibility

- [ ] Works in VS Code
- [ ] Works in Neovim (nvim-lspconfig)
- [ ] Works in Helix
- [ ] Works in Zed

---

## Testing Strategy

### Unit Tests

Each feature should have unit tests covering:

- Happy path
- Edge cases (empty input, malformed input)
- Error handling

### Integration Tests

Test full LSP request/response cycles:

```go
func TestIntegration_CompleteThenHover(t *testing.T) {
    server := setupIntegrationServer(t)

    // Initialize
    // Open document
    // Request completion
    // Verify items
    // Request hover on completion item
    // Verify hover content
}
```

### Manual Testing Checklist

For each feature, test in:

- [ ] VS Code with vscode-languageclient
- [ ] Neovim with nvim-lspconfig
- [ ] Sample Bazel workspace
- [ ] Sample Tilt project

---

## References

- [LSP Specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)
- [go.lsp.dev/protocol](https://pkg.go.dev/go.lsp.dev/protocol)
- [RFC-dialect-support.md](./RFC-dialect-support.md)
- [starpls](https://github.com/withered-magic/starpls) - Reference Starlark LSP
- [rust-analyzer](https://github.com/rust-lang/rust-analyzer) - Best-in-class LSP for inspiration
