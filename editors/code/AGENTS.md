# AGENTS.md

Instructions for AI coding agents working on the Sky - Starlark VS Code extension.

<project>
  <type>VS Code extension</type>
  <name>sky-starlark</name>
  <language>TypeScript (strict)</language>
  <lsp-server>skyls</lsp-server>
</project>

## Project Structure

```
src/
├── extension.ts      # Entry point: activate/deactivate
├── context.ts        # Shared state (ExtensionContext)
└── server/
    └── client.ts     # LSP client lifecycle
syntaxes/
└── starlark.tmLanguage.json  # TextMate grammar
```

<critical-files>
  <file path="src/extension.ts" purpose="Extension entry point, command registration" />
  <file path="src/context.ts" purpose="Central state management" />
  <file path="src/server/client.ts" purpose="Language server client setup" />
  <file path="package.json" purpose="Extension manifest, contributes, activation events" />
</critical-files>

## Build Commands

```bash
# Install and build
pnpm install && pnpm run build

# Watch mode
pnpm run watch

# Full validation
pnpm run typecheck && pnpm run lint && pnpm run format:check

# Package for distribution
pnpm run package
```

## Code Standards

<standards>
  <rule>TypeScript strict mode (@tsconfig/strictest)</rule>
  <rule>ESLint strictTypeChecked + stylisticTypeChecked</rule>
  <rule>No `any` types - use `unknown` with type guards</rule>
  <rule>Conventional commits for commit messages</rule>
</standards>

## VS Code Extension Patterns

<vscode-patterns>
  <pattern name="activation">Use activationEvents in package.json, not eager activation</pattern>
  <pattern name="commands">Register in package.json contributes.commands, implement in extension.ts</pattern>
  <pattern name="config">Define in package.json contributes.configuration, read via vscode.workspace.getConfiguration</pattern>
  <pattern name="output">Use OutputChannel for logs, not console.log</pattern>
</vscode-patterns>

## Common Tasks

<task name="add-command">
1. Add to package.json contributes.commands
2. Register handler in extension.ts activate()
3. Add to context.subscriptions for cleanup
</task>

<task name="add-config">
1. Add to package.json contributes.configuration.properties
2. Read via vscode.workspace.getConfiguration("sky-starlark")
3. Listen for changes via onDidChangeConfiguration if needed
</task>

<task name="lsp-feature">
1. Implement in language server (skyls)
2. Client in src/server/client.ts handles transport
3. Test with LSP inspector or debug logs
</task>

## Do Not

<constraints>
  <never>Use `any` type - use `unknown` with narrowing</never>
  <never>Add dependencies without justification</never>
  <never>Skip type checking or linting</never>
</constraints>
