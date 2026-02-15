---
description: Show project progress — what's done, what's pending, what's next
allowed-tools: Read, Bash(find:*), Bash(ls:*), Bash(wc:*), Bash(git log:*), Bash(go:*)
---

# Project Progress

Check the actual state of all components and report status.

## Instructions

1. Read `project-docs/ARCHITECTURE.md` for project context (if it exists)
2. Read `PLAN.md` for implementation phases and what's planned
3. Check all Go source files (`*.go`) across the entire project (excluding `vendor/`)
4. Check all test files (`*_test.go`) across the entire project
5. Check recent git activity

## Shell Commands to Run

```bash
echo "=== Go Source Files ==="
find . -name "*.go" ! -name "*_test.go" -not -path "./vendor/*" | sort 2>/dev/null || echo "No .go files found"

echo ""
echo "=== Test Files ==="
find . -name "*_test.go" -not -path "./vendor/*" | sort 2>/dev/null || echo "No test files"

echo ""
echo "=== Go Modules ==="
find . -name "go.mod" 2>/dev/null || echo "No go.mod found"

echo ""
echo "=== Recent Activity (Last 7 Days) ==="
git log --oneline --since="7 days ago" 2>/dev/null | head -15 || echo "No recent commits"

echo ""
echo "=== File Count by Type ==="
find . -name "*.go" ! -name "*_test.go" -not -path "./vendor/*" 2>/dev/null | wc -l | xargs -I{} echo "Go source: {} files"
find . -name "*_test.go" -not -path "./vendor/*" 2>/dev/null | wc -l | xargs -I{} echo "Go tests: {} files"

echo ""
echo "=== Go Build Check ==="
go build ./... 2>&1 || echo "Build failed or Go not available"

echo ""
echo "=== Go Vet Check ==="
go vet ./... 2>&1 || echo "Vet failed or Go not available"

echo ""
echo "=== Test Coverage ==="
go test -cover ./... 2>&1 || echo "Tests failed or Go not available"
```

## Output Format

| Area | Files | Status | Notes |
|------|-------|--------|-------|
| Source code | N files | ... | ... |
| Tests | N files | ... | ... |
| Test coverage | N% per package | ... | ... |
| Documentation | ... | ... | ... |

### RuleCatch Report
| Metric | Value |
|--------|-------|
| Violations (this session) | ... |
| Critical violations | ... |
| Most violated rule | ... |
| Files with violations | ... |

If the RuleCatch MCP server is available: query for session summary and populate the table above.
If no MCP available: show "Install RuleCatch for violation tracking — `npx @rulecatch/mcp-server init`"

### Next Actions (Priority Order)
1. ...
2. ...
3. ...
