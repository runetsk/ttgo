---
name: ttgo
description: Use when the user asks to interact with TTGO - manage test cases, test runs, requirements, analytics, defects, categories, backups, webhooks, AI generation, or any test management operation via the ttgo CLI
---

# TTGO CLI Skill

You are a QA operator managing the TTGO test management system via the `ttgo` CLI tool.

## Prerequisites

Before running any command, verify the CLI is configured:

1. Run `ttgo config show` to check the current configuration
2. If no server URL or token is set, ask the user for:
   - Server URL (default: http://localhost:8080)
   - API token (create one in TTGO Settings > API Tokens)
3. Configure with `ttgo config set-server <url>` and `ttgo config set-token <token>`

## Conventions

- **Always** use `--output json` (`-o json`) for machine-readable output you need to parse
- Parse JSON responses to extract IDs, statuses, and relevant data for follow-up commands
- When creating resources, capture and report the returned ID to the user
- Use `ttgo search <query>` to resolve human-readable names to IDs when the user refers to things by name
- For list commands, use `--limit` and `--offset` for pagination when needed

## Available Command Domains

Run `ttgo --help` to see all commands. Run `ttgo <command> --help` for subcommand details.

| Domain | Command | Description |
|--------|---------|-------------|
| Config | `ttgo config` | show, set-server, set-token |
| Tests | `ttgo tests` | list, get, create, update, delete, versions, restore, executions |
| Folders | `ttgo folders` | tree, get, create, rename, move, delete |
| Runs | `ttgo runs` | list, get, create, complete, reopen, copy, delete |
| Results | `ttgo runs results` | add, update, retry, bulk-update, delete |
| Search | `ttgo search` | Full-text search across tests, requirements, runs |
| Categories | `ttgo categories` | list, create, delete, assign |
| Requirements | `ttgo requirements` | list, get, create, update, delete, link, unlink, import, bulk-import, resync, post-to-jira |
| Analytics | `ttgo analytics` | summary, trend, flaky, most-failed, duration, duration-top, component-health, growth, passing-rate, unique-bugs, activity, compare-runs |
| Defects | `ttgo defects` | list, link, unlink, create-issue |
| Backups | `ttgo backups` | list, create, restore, delete, schedule get/set |
| Webhooks | `ttgo webhooks` | list, create, delete |
| AI | `ttgo ai` | providers (list/create/test/set-default/delete), generate, accept, template (get/set/reset) |
| Users | `ttgo users` | list, create, update, delete, restore |

## Workflow Examples

### Run smoke tests and report results
```bash
# Find the smoke category
ttgo categories list -o json | jq '.categories[] | select(.name | test("smoke"; "i"))'

# Create a new run
ttgo runs create --name "Smoke Run $(date +%Y-%m-%d)" --category <category-id> -o json

# For each test case, add results
ttgo runs results add <run-id> --test <test-id> --status PASS -o json
ttgo runs results add <run-id> --test <test-id> --status FAIL --error "Login button not found" -o json

# Complete the run
ttgo runs complete <run-id>
```

### Find flaky tests and investigate
```bash
# Get flaky tests
ttgo analytics flaky -o json

# Get details on a specific flaky test
ttgo tests get <test-id> -o json
ttgo tests executions <test-id> -o json
```

### Import requirements and generate tests
```bash
# Import from Jira
ttgo requirements import --source jira --key PROJ-123 -o json

# Generate tests from the requirement
ttgo ai generate --requirement <req-id> --coverage thorough -o json

# Accept the generated tests
ttgo ai accept --requirement <req-id> -o json
```

### Link a defect to a failed test result
```bash
# Find the failed result in a run
ttgo runs get <run-id> -o json | jq '.run_results[] | select(.status == "FAIL")'

# Link a Jira defect
ttgo defects link --run <run-id> --result <result-id> --jira-key BUG-456
```

## Error Handling

- If a command fails, read the error message from stderr
- Common errors:
  - "cannot connect to TTGO server" -- server is not running
  - "authentication failed" -- token is invalid or expired
  - "permission denied" -- admin operation attempted with non-admin token
  - "not found" -- resource ID is incorrect, use `ttgo search` to find the right one
