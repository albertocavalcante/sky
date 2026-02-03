# Roadmap

## Completed

| Tool       | Status  | Description                                    |
| ---------- | ------- | ---------------------------------------------- |
| `skyfmt`   | ✅ Done | Deterministic formatting with diff/check modes |
| `skylint`  | ✅ Done | Configurable linting with --fix support        |
| `skyquery` | ✅ Done | AST queries, load graph analysis               |
| `skycheck` | ✅ Done | Static analysis (undefined names, unused vars) |
| `skydoc`   | ✅ Done | Documentation generator (markdown/JSON)        |
| `skyrepl`  | ✅ Done | Interactive REPL with builtins                 |
| `skytest`  | ✅ Done | Test runner with assertions                    |
| `sky`      | ✅ Done | Plugin dispatcher and marketplace              |

## In Progress

### Phase 1: Coverage & Testing (Current)

- [ ] `skycov` - Coverage reporting tool
  - CLI and output formatters (text, HTML, Cobertura, LCOV)
  - Requires `starlark-go-x` instrumentation API

### Phase 2: IDE Experience

- [ ] `skylsp` - Language Server Protocol implementation
  - Go to definition, find references (via skyquery)
  - Hover documentation (via skydoc)
  - Diagnostics (via skylint, skycheck)
  - Code actions (via skylint --fix)

### Phase 3: Advanced Type Checking

- [ ] `skycheck` Phase 2 - Type inference
- [ ] `skycheck` Phase 3 - Type annotations (requires starlark-go-x)
- [ ] Type stubs (.skyi files) for builtins

## starlark-go-x Dependencies

Changes needed in our [starlark-go fork](https://github.com/albertocavalcante/starlark-go-x):

| Change                       | Priority | Tools Affected  |
| ---------------------------- | -------- | --------------- |
| Coverage instrumentation API | P0       | skycov, skytest |
| Finish type-hints parsing    | P1       | skycheck        |
| Completion API               | P2       | skylsp, skyrepl |

## Future Ideas

- EditorConfig support for skyfmt
- Watch mode for skylint
- Parallel file processing
- Plugin signing and verification
