---
description: Strict, deterministic workflow for addressing PR review feedback and verifying CI health in the sky monorepo
---

# /review

<purpose>
A STRICT, DETERMINISTIC workflow for addressing PR review feedback in the sky monorepo. This is "God Mode" — every step is explicit, every decision has a rule, every action has verification.
</purpose>

<ironclad_rules>

1. **NEVER IGNORE ANY COMMENT** — Every thread MUST be accounted for
2. **GRAPHQL IS TRUTH** — Use saved queries for authoritative thread state
3. **DIFF FIRST** — ALWAYS read `gh pr diff` before ANY action
4. **TEMP FILES MANDATORY** — Dump ALL API output to `/tmp/pr-<N>-*.json`
5. **REPLY THEN RESOLVE** — Every addressed thread gets a reply AND explicit resolution
6. **NO BROWSER** — `gh` CLI exclusively
7. **VERIFY BEFORE COMMIT** — Run `bazel test` and `make lint` for EVERY code change </ironclad_rules>

---

## Phase 0: Understand the PR (MANDATORY FIRST STEP)

<critical>
You MUST understand what the PR does BEFORE looking at review comments.
Skipping this step leads to incorrect fixes and wasted cycles.
</critical>

### 0.1 Get PR Metadata

```bash
gh pr view <PR_NUMBER> --json title,body,headRefName,baseRefName,state,author \
  --jq '{title, body: (.body[:500] + "..."), branch: .headRefName, base: .baseRefName, state, author: .author.login}'
```

### 0.2 Read the Diff (SOURCE OF TRUTH FOR CHANGES)

```bash
# Full diff to file for reference
gh pr diff <PR_NUMBER> > /tmp/pr-<PR_NUMBER>-diff.patch

# Quick summary: files changed
gh pr diff <PR_NUMBER> --name-only

# Stat summary
gh pr view <PR_NUMBER> --json files --jq '.files[] | "\(.path) +\(.additions) -\(.deletions)"'
```

<decision_tree id="diff-analysis"> BEFORE proceeding, answer these questions by reading the diff:

1. What is the PRIMARY change? (feature/fix/refactor/test)
2. Which files are CORE to the change vs supporting?
3. Are there any RISKY changes? (public API, config, build, concurrency)
4. What tests cover this change?

Document answers mentally before Phase 1. </decision_tree>

---

## Phase 1: Fetch Status & Threads

### 1.1 CI Status Check

```bash
gh pr view <PR_NUMBER> --json state,statusCheckRollup \
  --jq '{state: .state, checks: [.statusCheckRollup[]? | {name: .name, status: .status, conclusion: .conclusion}]}'
```

### 1.2 Fetch Review Threads (GraphQL Source of Truth)

```bash
# NOTE: ':owner' and ':repo' are gh CLI magic variables that auto-resolve from git remote
gh api graphql -F owner=':owner' -F name=':repo' -F number=<PR_NUMBER> \
  -f query="$(cat .agent/queries/pr-review-threads.graphql)" \
  --paginate > /tmp/pr-<PR_NUMBER>-threads.json

# Verify capture
jq '{pr_id: .data.repository.pullRequest.id, total_threads: .data.repository.pullRequest.reviewThreads.nodes | length}' \
  /tmp/pr-<PR_NUMBER>-threads.json
```

### 1.3 Inventory Actionable Threads

```bash
uv run .agent/scripts/inventory_threads.py /tmp/pr-<PR_NUMBER>-threads.json
```

<fallback>
If script unavailable:
```bash
jq '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false and .isOutdated==false) | {threadId: .id, commentId: .comments.nodes[0].id, path, line, body: .comments.nodes[0].body}' /tmp/pr-<PR_NUMBER>-threads.json
```
</fallback>

---

## Phase 2: Evaluate Each Thread

<critical>
For EACH actionable thread, you MUST classify it using this decision tree.
NO EXCEPTIONS. NO SKIPPING.
</critical>

<decision_tree id="thread-classification">

```
┌─────────────────────────────────────────────────────────────┐
│                    THREAD CLASSIFICATION                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Q1: Is this feedback VALID?                                │
│      ├─ NO  → REJECT (explain why, cite evidence)           │
│      │        (Note: Handle false positives here)           │
│      └─ YES → Continue to Q2                                │
│                                                             │
│  Q2: Is this IN SCOPE for this PR?                          │
│      ├─ NO  → DEFER (create issue, link in reply)           │
│      └─ YES → Continue to Q3                                │
│                                                             │
│  Q3: Is this a QUICK FIX (< 5 min)?                         │
│      ├─ YES → FIX immediately                               │
│      └─ NO  → FIX with dedicated commit                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

</decision_tree>

### Classification Actions

<action id="FIX">
1. Locate the file and line from thread metadata
2. Cross-reference with `/tmp/pr-<PR_NUMBER>-diff.patch`
3. Make the code change
4. Run relevant tests: `bazel test //path/to/pkg:pkg_test`
5. Verify build and dependencies: `make gazelle && bazel build //...`
6. Commit: `git commit -m "fix: address review - <brief description>"`
</action>

<action id="REJECT">
1. Formulate clear technical reasoning (e.g., performance impact, Starlark spec compatibility)
2. Cite specific code/docs or external references (e.g., Bazel docs) as evidence
3. Reply with explanation (Phase 3)
4. Do NOT resolve — let reviewer respond
</action>

<action id="DEFER">
1. Follow `/defer` workflow to create GitHub issue
2. Reply: "Created #NNN to track this. Out of scope for this PR because [reason]."
3. Resolve thread after reply
</action>

---

## Phase 2.5: Common Feedback Patterns (Go & Bazel Examples)

<examples>
<example id="missing-lock">
<feedback>"This map access is not thread-safe"</feedback>
<classification>FIX</classification>
<action>
1. Add a `sync.Mutex` or use `internal/plugins.Store.withLock` if applicable.
2. Ensure `defer mu.Unlock()` is used immediately after locking.
</action>
<reply>"Fixed in abc123. Added mutex protection to the map access."</reply>
</example>

<example id="unwrapped-error">
<feedback>"Wrap this error to provide context"</feedback>
<classification>FIX</classification>
<action>
1. Change `return err` to `return fmt.Errorf("context: %w", err)`.
</action>
<reply>"Fixed in abc123. Wrapped error with additional context."</reply>
</example>

<example id="bazel-deps">
<feedback>"Missing dependency in BUILD.bazel"</feedback>
<classification>FIX</classification>
<action>
1. Add the import in the Go file.
2. Run `make gazelle` to update BUILD.bazel automatically.
</action>
<reply>"Fixed in abc123. Ran gazelle to sync BUILD files."</reply>
</example>

<example id="interface-implementation">
<feedback>"This struct should implement the X interface"</feedback>
<classification>FIX</classification>
<action>
1. Add a compile-time check: `var _ Interface = (*Struct)(nil)`.
2. Implement missing methods.
</action>
<reply>"Fixed in abc123. Implemented Interface X and added compile-time check."</reply>
</example>

<example id="cognitive-complexity">
<feedback>"Cognitive complexity too high"</feedback>
<classification>FIX or DEFER</classification>
<decision>
- If refactor is straightforward (extract helper functions): FIX
- If requires significant architectural change: DEFER with issue
</decision>
<action_fix>
1. Extract logic to private helper functions.
2. Verify tests still pass using `bazel test`.
</action_fix>
<reply_fix>"Fixed in abc123. Refactored complex function into helpers."</reply_fix>
</example>
</examples>

---

## Phase 3: Reply & Resolve (GraphQL Mutations)

<critical>
You need THREE IDs from Phase 1 data:
- `PR_NODE_ID`: From `.data.repository.pullRequest.id`
- `COMMENT_NODE_ID`: From thread's `.comments.nodes[0].id` (for reply)
- `THREAD_NODE_ID`: From thread's `.id` (for resolve)
</critical>

### 3.1 Extract IDs

```bash
# Get PR Node ID
jq -r '.data.repository.pullRequest.id' /tmp/pr-<PR_NUMBER>-threads.json

# Get Thread and Comment IDs for a specific thread
jq -r '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved==false) | {thread_id: .id, comment_id: .comments.nodes[0].id, path: .path, line: .line}' /tmp/pr-<PR_NUMBER>-threads.json
```

### 3.2 Reply to Thread

```bash
gh api graphql \
  -F pullRequestId="<PR_NODE_ID>" \
  -F inReplyTo="<COMMENT_NODE_ID>" \
  -F body="Fixed in <SHORT_SHA>." \
  -f query="$(cat .agent/queries/reply-to-thread.graphql)"
```

### 3.3 Resolve Thread

```bash
gh api graphql \
  -F threadId="<THREAD_NODE_ID>" \
  -f query="$(cat .agent/queries/resolve-review-thread.graphql)"
```

<reply_templates>

| Scenario             | Reply Template                                                                         |
| -------------------- | -------------------------------------------------------------------------------------- |
| Fixed                | "Fixed in `abc123`."                                                                   |
| Fixed with detail    | "Fixed in `abc123`. [brief explanation of change]"                                     |
| Deferred             | "Created #NNN to track this. [reason for deferral]"                                    |
| Rejected             | "[Technical reasoning]. [Evidence/citation]. Happy to discuss."                        |
| Clarification needed | "Could you clarify [specific question]? I want to make sure I address this correctly." |
| </reply_templates>   |                                                                                        |

---

## Phase 4: Push & Verify CI

### 4.1 Pre-Push Checklist

```bash
# Verify branch
git branch --show-current

# Verify no uncommitted changes
git status

# Run full test suite via Bazel
bazel test //...

# Run lint and format
make lint
make format
```

### 4.2 Push

```bash
git push origin <BRANCH_NAME>
```

### 4.3 Watch CI

```bash
gh pr checks <PR_NUMBER> --watch
```

<troubleshooting id="ci-issues">
<issue>Bazel build failure in CI</issue>
<diagnosis>
Check if `MODULE.bazel.lock` or `go.sum` is out of sync.
```bash
bazel mod tidy
go mod tidy
```
</diagnosis>
<resolution>
1. Sync dependencies locally.
2. Commit and push.
</resolution>

<issue>Test failure</issue>
<diagnosis>
```bash
gh run view <RUN_ID> --log-failed
```
</diagnosis>
<resolution>
1. Reproduce locally: `bazel test //path/to/failing:test`
2. Fix the issue
3. Commit and push
</resolution>
</troubleshooting>

---

## Phase 5: Final Verification (ZERO CHECK)

<critical>
This phase is NON-NEGOTIABLE. You MUST verify zero unresolved threads.
</critical>

### 5.1 Re-Fetch Threads

```bash
gh api graphql -F owner=':owner' -F name=':repo' -F number=<PR_NUMBER> \
  -f query="$(cat .agent/queries/pr-review-threads.graphql)" \
  --paginate > /tmp/pr-<PR_NUMBER>-threads-final.json
```

### 5.2 Count Unresolved

```bash
jq '[.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved == false and .isOutdated == false)] | length' /tmp/pr-<PR_NUMBER>-threads-final.json
```

<assertion>
**RESULT MUST BE 0**

If not zero:

1. List remaining threads
2. Return to Phase 2
3. DO NOT proceed until zero
   </assertion>

---

## Phase 6: Cleanup & Summary

### 6.1 Remove Temp Files

```bash
rm -f /tmp/pr-<PR_NUMBER>-*.json /tmp/pr-<PR_NUMBER>-*.patch
```

### 6.2 Summary Report

```markdown
## PR #<NUMBER> Review Summary

| Metric            | Value             |
| -----------------| ----------------- |
| Threads Addressed | X                 |
| Commits Added     | Y                 |
| CI Status         | ✅ Pass / ❌ Fail |
| Unresolved        | 0                 |

### Actions Taken

- Fixed: [list of fixes]
- Deferred: [list with issue links]
- Rejected: [list with reasoning]
```

```