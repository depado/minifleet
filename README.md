<p align="center">
  <img alt="minifleet" src="https://shieldcn.dev/header/grid.svg?title=minifleet&subtitle=Minimal+fleet+management+for+your+GitHub+repositories.&mode=dark&align=left&border=false">
</p>

<p align="center">
  <a href="https://github.com/depado/minifleet/actions"><img src="https://shieldcn.dev/github/ci/depado/buoy.svg?variant=branded" alt="CI" /></a>
  <a href="https://github.com/depado/minifleet/releases"><img src="https://shieldcn.dev/github/release/depado/buoy.svg?variant=branded" alt="Release" /></a>
  <a href="https://github.com/depado/minifleet/blob/main/LICENSE"><img src="https://shieldcn.dev/github/license/depado/buoy.svg?variant=branded" alt="License" /></a>
  <a href="https://github.com/depado/minifleet"><img src="https://shieldcn.dev/github/last-commit/depado/buoy.svg?variant=branded" alt="Last Commit" /></a>
  <a href="https://github.com/depado/minifleet"><img src="https://shieldcn.dev/github/stars/depado/buoy.svg?variant=branded" alt="Stars" /></a>
  <a href="https://github.com/depado/minifleet/graphs/contributors"><img src="https://shieldcn.dev/github/contributors/depado/buoy.svg?variant=branded" alt="Contributors" /></a>
  <a href="https://github.com/depado/minifleet/issues"><img src="https://shieldcn.dev/github/issues/depado/buoy.svg?variant=branded" alt="Issues" /></a>
  <a href="https://github.com/depado/minifleet/pkgs/container/buoy"><img src="https://shieldcn.dev/badge/container-ghcr.io%2Fdepado%2Fbuoy-2496ED.svg?logo=docker&variant=branded" alt="container image" /></a>
</p>

> [!WARNING]
> **Work in progress.** minifleet is under active development. APIs, commands, and behavior may change.

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Commands](#commands)
  - [`minifleet sync`](#minifleet-sync)
  - [`minifleet list`](#minifleet-list)
  - [`minifleet status`](#minifleet-status)
  - [`minifleet prs`](#minifleet-prs)
  - [`minifleet run`](#minifleet-run)
  - [`minifleet init`](#minifleet-init)
- [Filters](#filters)
- [One-shot mode (`--fleet.path`)](#one-shot-mode)
- [Shallow clones (`--fleet.shallow`)](#shallow-clones)
- [Manifest File](#manifest-file)
- [Configuration](#configuration)
- [Concurrency](#concurrency)
- [Development](#development)

## Features

- **Sync** — Clone missing repos and pull existing ones in a single command. Auto-detects org vs. user via the GitHub API. Optional shallow clones. Host configurable for GitHub Enterprise.
- **Run across repos** — `run -- "<command>"` executes a shell command in every repository directory (or a filtered subset), with summary or streaming output.
- **Remote discovery** — `list` shows repos from the API with filters for archived, forks, topics, visibility, language, labels, and groups. Output as table, JSON, or YAML manifest.
- **Local status dashboard** — See the state of all cloned repos: current branch, commits ahead/behind remote, uncommitted changes, stashed changes.
- **Cross-repo PR dashboard** — List open pull requests across all repos with CI status (success/pending/failure) and review status (approved/changes/pending) in a single table.
- **Unified filters** — Every command accepts the same filter flags: `--target`, `--topic`, `--include-archived`, `--include-forks`, `--visibility`, `--language`, `--label`, `--group`.
- **One-shot mode** — `--fleet.path <dir>` bypasses the `host/owner/repo` nesting for ad-hoc syncs into any directory.
- **GitHub Enterprise** — `--github.host <host>` retargets the API and clone URLs at a GHE instance.
- **Concurrency built-in** — Parallel goroutines with a bounded worker pool, context-cancellable.
- **gorich progress bars** — Live progress bars with spinners, per-repo status, and M-of-N counts.
- **Manifest** — Optional YAML file declaring groups, per-repo labels, custom clone paths, and ignored repos. Auto-managed by `sync`; user-edited for fleet metadata.
- **Single binary** — Written in Go, only `git` on `$PATH` is required.

## Quick Start

### 1. Set your token

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
```

A token with `repo` scope is needed for private repos; public repos work unauthenticated at lower rate limits.

### 2. Initialize config

```bash
minifleet init --token ghp_xxxx --base ~/dev
# or: minifleet init -s   # show current configuration
```

### 3. Sync an organization or user

```bash
minifleet sync my-org
```

Repos are cloned into `~/dev/github.com/my-org/<repo>`. Repos that already exist locally are pulled (`git fetch` + `git rebase --autostash`). Already-synced repos are skipped, ignored repos (see manifest) are skipped.

### 4. Local status across the fleet

```bash
minifleet status
```

```
┏━━━━━━━━━━━━━━┳━━━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━┳━━━━━━━┓
┃ Repo         ┃ Branch ┃ Behind ┃ Ahead ┃ Dirty ┃ Stash ┃
┡━━━━━━━━━━━━━━╇━━━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━╇━━━━━━━┩
│ svc-api      │ main   │ 0      │ 3     │ no    │ 0     │
│ svc-auth     │ dev    │ 5      │ 0     │ yes   │ 2     │
│ web-app      │ main   │ 0      │ 0     │ no    │ 0     │
└──────────────┴────────┴────────┴───────┴───────┴───────┘
```

### 5. Open PRs with CI status

```bash
minifleet prs my-org
```

```
┏━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━┓
┃ Repo         ┃ Pull Request                 ┃ Author   ┃ CI       ┃ Review     ┃
┡━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━┩
│ svc-api      │ feat: add payment endpoint   │ alice    │ ✓        │ approved   │
│ svc-api      │ fix: handle null pointer     │ bob      │ ✗        │ pending    │
└──────────────┴──────────────────────────────┴──────────┴──────────┴────────────┘
```

## Installation

### From source

```bash
go install github.com/depado/minifleet@latest
```

### Build locally

```bash
git clone https://github.com/depado/minifleet.git
cd minifleet
make build
sudo cp minifleet /usr/local/bin/
```

Prerequisites: `git` on `$PATH`.

## Commands

Every command accepts the [global flags](#configuration) and the [filter flags](#filters).

### `minifleet sync`

```
minifleet sync [owner] [flags]
```

Clone missing repos and pull existing ones for a GitHub user or organization. When no owner is given, syncs every owner in the fleet manifest.

```
Flags:
  --target, -t string        regex to match repo names
  --topic stringArray        filter by topic (repeatable)
  --include-archived         include archived repos
  --include-forks            include forked repos
  --visibility string        all, public, private (default: all)
  --language string          filter by primary language
  --label stringArray        filter by manifest label (key=value or key, repeatable)
  --group string             filter by manifest group
  --dry-run                  (planned) preview what would be synced
```

**Examples:**

```bash
# Sync an org with default layout (~/dev/github.com/<org>/<repo>)
minifleet sync my-org

# Sync only Go services into a custom directory, shallow
minifleet --fleet.path ~/scratch/go-services --fleet.shallow sync my-org --language Go

# Sync multiple owners declared in the manifest
minifleet sync

# GitHub Enterprise
minifleet --github.host github.example.com sync my-org
```

On each sync, the manifest is merged with the GitHub API response: API-tracked fields (`topics`, `language`, `archived`, ...) are overwritten from the API; user-set fields (`labels`, `protocol`, `ignored`, `path`) are preserved.

### `minifleet list`

```
minifleet list <owner> [flags]
```

List repositories from the GitHub API. Auto-detects org vs. user. Output as `table` (default), `json`, or `yaml` (a seed manifest).

```
Flags:
  --target, -t string        regex to match repo names
  --topic stringArray        filter by topic
  --include-archived        include archived repos
  --include-forks           include forked repos
  --visibility string        all, public, private (default: all)
  --language string          filter by primary language
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --format, -f string        table, json, yaml (default: table)
  --limit int                max repos to list (default: 1000)
```

```bash
minifleet list my-org --language Go
minifleet list my-org --format json | jq '.[] | .name'
minifleet list my-org --format yaml > fleet.yml  # seed a manifest
```

### `minifleet status`

```
minifleet status [flags]
```

Walk the local directory tree (`fleet.base` or `fleet.path`) for git repos and show their status in a single table.

```
Flags:
  --target, -t string        regex to match repo names
  --topic stringArray        filter by topic (via manifest)
  --include-archived         include repos flagged as archived in manifest
  --include-forks            include repos flagged as fork in manifest
  --visibility string        filter by visibility (via manifest)
  --language string          filter by language (via manifest)
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --format, -f string        table, json (default: table)
```

Manifest-based filters (`--label`, `--group`, manifest-flagged archived/forks/topics/language) require a manifest; without one, they are silently skipped.

### `minifleet prs`

```
minifleet prs <owner> [flags]
```

Open pull requests across every repo of an owner with CI and review status.

```
Flags:
  --state string             open, closed, all (default: open)
  --author, -a string        filter by PR author
  --no-draft                 exclude draft PRs
  --target, -t string        regex to match repo names
  --topic stringArray        filter by topic
  --include-archived         include archived repos
  --include-forks            include forked repos
  --visibility string        filter by visibility
  --language string          filter by primary language
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --format, -f string        table, json (default: table)
```

### `minifleet run`

```
minifleet run -- <command> [flags]
```

Run a shell command in every local repository directory (or a filtered subset). Uses the existing executor for bounded concurrency and continue-on-error.

```
Flags:
  --target, -t string        regex to match repo names
  --topic stringArray        filter by topic (via manifest)
  --include-archived         include archived repos
  --include-forks            include forked repos
  --visibility string        filter by visibility (via manifest)
  --language string          filter by primary language (via manifest)
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --summary                  one line per repo, capture output; --summary=false shows live blocks (default: true)
  --block-lines int          output lines per repo block in live mode (default: 3)
  --dry-run                  print what would run; do not execute
  --shell string             shell to invoke (default: sh)
```

Use `--` to separate flags from the command itself.

**Examples:**

```bash
# Run Go tests across the fleet
minifleet run -- "go test ./..."

# Lint only backend repos
minifleet run --group backend -- "make lint"

# Stream output of a build (interleaved, per-repo prefixes)
minifleet run --summary=false --language go -- "make build"

# Cross-repo code search
minifleet run -- "grep -r 'TODO' ."

# Dry-run a destructive bulk change
minifleet run --dry-run --target "^old-" -- "rm -f .env.local"
```

**Summary mode** (default): one line per repo (`✓`/`✗ exit N` + duration); failed repos also print their captured stderr and stdout.

**Live block mode** (`--summary=false`): when stdout is a terminal, each concurrent repo gets a fixed-height block that updates in place:

```
→ articles
  3354578 chore: remove drone config...
  26742bf Bump github.com/gin-gonic/gin...
→ bfplus
  53c117a chore(deps): update go toolchain...
  5ed469d fix(deps): update module...
```

When a repo finishes its block collapses to `✓ repo (elapsed)` (or `✗ exit N repo (elapsed)` on failure, keeping the last output lines visible). The display reflows automatically via gorich's `live.Live`. Use `--block-lines N` to control the output window height per block (default 3). In a non-terminal (piped, CI), falls back to `repo › line` prefixes.

### `minifleet init`

```
minifleet init [flags]
```

Write `~/.config/minifleet/config.yml` with defaults, or show the current configuration.

```
Flags:
  -t, --token string   GitHub personal access token to write
  -b, --base string    base directory for clones
  -s, --show           print current configuration
```

## Filters

Every command accepts the same filter flags so users get a consistent vocabulary:

| Flag | Type | Behavior |
|------|------|----------|
| `--target` / `-t` | regex | Match on repo name (or local directory name for `status`) |
| `--topic` | stringArray | Match if repo has any of the given topics (OR) |
| `--include-archived` | bool | Include archived repos (excluded by default) |
| `--include-forks` | bool | Include forked repos (excluded by default) |
| `--visibility` | string | `all` (default), `public`, `private` |
| `--language` | string | Match on repo primary language (e.g. `go`, `python`) |
| `--label` | stringArray | Match on manifest labels: `tier=1` (exact) or `tier` (any value). AND across labels. |
| `--group` | string | Limit to repos in a manifest group |

Filters compose freely:

```bash
# Go services labeled tier=1 in the backend group
minifleet status --language go --label tier=1 --group backend
```

`--label` and `--group` consult the manifest; `--target`, `--topic`, `--include-archived`, `--include-forks`, `--visibility`, and `--language` work from the API response or local scan alone.

## One-shot mode

By default, `sync` and `status` use the `~/dev/{host}/{owner}/{repo}` layout. Pass `--fleet.path <dir>` to bypass the nesting entirely:

```bash
# Drop the org's repos directly under ~/scratch/my-org
minifleet --fleet.path ~/scratch/my-org sync my-org

# Status of those same repos
minifleet --fleet.path ~/scratch/my-org status
```

With `--fleet.path`, `status` scans the directory one level deep (each subdirectory is treated as a repo). `sync` writes each repo to `<path>/<repo-name>` and records the actual path in the manifest.

## Shallow clones

`--fleet.shallow` toggles `git clone --depth 1 --filter=blob:none` for speed:

```bash
minifleet sync my-org --fleet.shallow
```

Shallow clones are smaller and faster but lack full history. They cannot push most commits without unshallowing. Use for one-shots, dashboards, or large fleets where you only need the latest tree. The default (full clone) is recommended for fleets you intend to push from.

Shallow is also available as a config value `fleet.shallow: true`.

## Manifest File

A manifest (`~/.config/minifleet/fleet.yml`) declares metadata about your repos that GitHub's API doesn't track. It's **optional** — every command works without one — but it enables groups, custom labels, per-repo clone protocol, custom paths, and ignored repos.

Generate a seed from the API:

```bash
minifleet list my-org --format yaml > ~/.config/minifleet/fleet.yml
```

Then edit it:

```yaml
version: "1"
owners:
  my-org:
    groups:
      backend:
        - my-org/svc-api
        - my-org/svc-auth
      frontend:
        - my-org/web-app
    repos:
      - full_name: my-org/svc-api
        protocol: ssh
        labels:
          tier: "1"
          language: go
      - full_name: my-org/web-app
        protocol: https
        labels:
          tier: "2"
      - full_name: my-org/old-prototype
        ignored: true
        labels:
          status: deprecated
```

### Field ownership

| Category | Fields | Owner |
|----------|--------|-------|
| API-tracked | `topics`, `language`, `archived`, `fork`, `private`, `updated_at` | `sync` overwrites from API |
| User-set | `labels`, `protocol`, `ignored`, `path` | User — never touched by `sync` |
| User-set | `groups` (per-owner) | User — never touched by `sync` |

### Groups

Use groups to scope filters:

```bash
minifleet status --group backend
minifleet sync --group frontend
```

### Per-repo protocol

Each repo can declare `protocol: ssh` or `protocol: https`. `sync` reads this when cloning. Without a saved protocol, `sync` tries SSH first and falls back to HTTPS.

### Ignored repos

Set `ignored: true` to skip a repo in all bulk operations. Useful for archived or experimental repos you keep locally but don't want updated.

## Configuration

All settings can be set via CLI flags, environment variables, or a config file. Precedence: **Flags > Env > Config File > Defaults**.

### Config file (`~/.config/minifleet/config.yml`)

```yaml
github:
  token: ""           # GitHub PAT (or use GITHUB_TOKEN env)
  host: github.com    # Use a custom host for GitHub Enterprise

fleet:
  base: ~/dev          # Base directory for clones
  path: ""             # (optional) one-shot override; bypass host/owner nesting
  shallow: false       # Use shallow clones by default
  concurrent: 5        # Max concurrent operations

log:
  level: info          # debug, info, warn, error
  format: text         # json or text
  source: false
  color: auto          # auto, always, never

ui:
  progress: true
  color: true
```

### Reference

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `github.token` | `GITHUB_TOKEN` | - | GitHub personal access token |
| `github.host` | `MINIFLEET_GITHUB_HOST` | `github.com` | GitHub host (GHE: `github.example.com`) |
| `fleet.base` | `MINIFLEET_FLEET_BASE` | `~/dev` | Base directory for clones |
| `fleet.path` | `MINIFLEET_FLEET_PATH` | - | One-shot directory override (flat layout) |
| `fleet.shallow` | `MINIFLEET_FLEET_SHALLOW` | `false` | Use shallow clones |
| `fleet.concurrent` | `MINIFLEET_FLEET_CONCURRENT` | `5` | Max concurrent operations |
| `log.level` | `MINIFLEET_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `log.format` | `MINIFLEET_LOG_FORMAT` | `text` | `json` or `text` |
| `log.source` | `MINIFLEET_LOG_SOURCE` | `false` | Include source file in logs |
| `log.color` | `MINIFLEET_LOG_COLOR` | `auto` | `auto`, `always`, `never` |
| `ui.progress` | `MINIFLEET_UI_PROGRESS` | `true` | Show progress bars |
| `ui.color` | `MINIFLEET_UI_COLOR` | `true` | Enable colored output |

## Concurrency

Every bulk operation uses the same bounded-goroutine executor. Configure it globally or per-command:

```bash
# Clone with 10 concurrent git processes
minifleet sync my-org --fleet.concurrent 10

# Fetch PRs with 3 concurrent API calls (staying under secondary rate limits)
minifleet prs my-org --fleet.concurrent 3
```

- A bounded goroutine pool processes tasks from a channel.
- Each task runs independently; one failure doesn't block others.
- `Ctrl+C` stops new tasks but lets in-flight operations complete.
- Results are collected as succeeded, skipped, and failed with per-repo error messages.

**Rate limits:** GitHub's secondary rate limit allows ~100 concurrent requests and ~900 points/minute. For API-heavy commands like `prs`, keep `--fleet.concurrent` at 5 or lower. For local git operations (`sync`, `status`), higher values (10+) are fine — they're I/O-bound.

## Development

```bash
go mod tidy
make build       # compile the binary
make test        # run tests with race detector
make lint        # run golangci-lint
make dev         # run with OTLP enabled
make docker      # build Docker image
make release     # create a GitHub release via goreleaser
make clean       # remove binary and coverage output
```

### Architecture

```
minifleet/
├── main.go                    # Entrypoint
├── cmd/
│   ├── root.go                # Setup, config caching via context, command registration
│   ├── conf.go                # Conf struct, NewConf, NewLogger (viper-backed)
│   ├── flags.go               # Persistent flag definitions
│   ├── filters.go             # Filters struct + Apply — shared by every command
│   ├── shared.go             # printBulkSummary helper
│   ├── sync.go               # clone+pull unified; uses DetectOwner and manifest Index
│   ├── list.go
│   ├── status.go
│   ├── prs.go
│   ├── run.go                # execute shell command across repos
│   ├── config.go             # init/show command
│   └── version.go
├── internal/
│   ├── provider/              # Provider interface
│   │   └── github/            # GitHub REST API client
│   ├── git/                   # System git operations (clone, pull, status)
│   ├── fleet/                # Executor (bounded concurrency) + Scanner
│   ├── manifest/             # YAML manifest load/save/merge/generate + Index
│   └── ui/                   # gorich table helpers
└── docs/                     # Research reports and implementation plan
```

### Design principles

- **DRY** — Every command is ~50 lines. Filters and the executor are shared.
- **LEAN** — One interface (`Provider`) where platform abstraction is needed; function types elsewhere.
- **No global state** — Configuration flows from `PersistentPreRunE` → context → command `RunE`.
- **Concurrency by default** — The `Executor` handles parallelism for every command.
- **Continue on error** — One repo failing never aborts the whole operation.

### Build information

```bash
make build
./minifleet version
# Build: 9a3b2c1
# Version: 0.1.0-dev
# Build Date: 2026-07-12T08:54:45Z
```

## License

MIT — see [LICENSE](LICENSE).
