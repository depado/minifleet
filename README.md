<p align="center">
  <img alt="minifleet" src="https://shieldcn.dev/header/grid.svg?title=minifleet&subtitle=Minimal+fleet+management+for+your+GitHub+repositories.&mode=dark&align=left&border=false">
</p>

<p align="center">
  Sync, observe, and operate on all your repositories with progress bars, unified filters, and concurrency built in.
</p>

<p align="center">
  <a href="https://github.com/depado/minifleet/actions"><img src="https://shieldcn.dev/github/ci/depado/minifleet.svg?variant=branded" alt="CI" /></a>
  <a href="https://github.com/depado/minifleet/releases"><img src="https://shieldcn.dev/github/release/depado/minifleet.svg?variant=branded" alt="Release" /></a>
  <a href="https://github.com/depado/minifleet/blob/main/LICENSE"><img src="https://shieldcn.dev/github/license/depado/minifleet.svg?variant=branded" alt="License" /></a>
  <a href="https://github.com/depado/minifleet"><img src="https://shieldcn.dev/github/last-commit/depado/minifleet.svg?variant=branded" alt="Last Commit" /></a>
  <a href="https://github.com/depado/minifleet"><img src="https://shieldcn.dev/github/stars/depado/minifleet.svg?variant=branded" alt="Stars" /></a>
  <a href="https://github.com/depado/minifleet/graphs/contributors"><img src="https://shieldcn.dev/github/contributors/depado/minifleet.svg?variant=branded" alt="Contributors" /></a>
  <a href="https://github.com/depado/minifleet/issues"><img src="https://shieldcn.dev/github/issues/depado/minifleet.svg?variant=branded" alt="Issues" /></a>
  <a href="https://github.com/depado/minifleet/pkgs/container/minifleet"><img src="https://shieldcn.dev/badge/container-ghcr.io%2Fdepado%2Fminifleet-2496ED.svg?logo=docker&variant=branded" alt="container image" /></a>
</p>

<p align="center">
  <img alt="minifleet demo" src="vhs/minifleet.gif">
</p>

> [!WARNING]
> **Work in progress.** minifleet is under active development. APIs, commands, and behavior may change.

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Commands](#commands)
  - [`minifleet discover`](#minifleet-discover)
  - [`minifleet sync`](#minifleet-sync)
  - [`minifleet list`](#minifleet-list)
  - [`minifleet status`](#minifleet-status)
  - [`minifleet prs`](#minifleet-prs)
  - [`minifleet run`](#minifleet-run)
  - [`minifleet init`](#minifleet-init)
- [Fleet directories](#fleet-directories)
- [Filters](#filters)
- [One-shot mode (`--fleet.path`)](#one-shot-mode)
- [Shallow clones (`--fleet.shallow`)](#shallow-clones)
- [Manifest File](#manifest-file)
- [Configuration](#configuration)
- [Concurrency](#concurrency)
- [Development](#development)

## Features

- **Discover** — Fetch repositories from the GitHub API, apply filters, and create or update a `fleet.yml` manifest. No cloning — separate API concerns from local operations.
- **Sync** — Clone missing repos and pull existing ones based on the `fleet.yml` manifest. Purely local: no network calls, no token needed, works offline.
- **Run across repos** — `run -- "<command>"` executes a shell command in every repository directory (or a filtered subset), with summary or live block output.
- **Local-first listing** — `list` shows repos from the manifest when available, with an API fallback when no manifest exists. Output as table, JSON, or YAML.
- **Local status dashboard** — See the state of all cloned repos: current branch, commits ahead/behind remote, uncommitted changes, stashed changes.
- **Cross-repo PR dashboard** — List open pull requests across all repos with CI status (success/pending/failure) and review status (approved/changes/pending). Repo list comes from the manifest; PR data from the API.
- **Unified filters** — Every command that queries the API or filters repos accepts the same filter flags: `--include-regex`, `--exclude-regex`, `--include`, `--exclude`, `--topic`, `--include-archived`, `--include-forks`, `--visibility`, `--language`, `--label`, `--group`, `--has-file`, `--if`.
- **Per-directory fleets** — A `fleet.yml` lives alongside the repos it describes; `config.yml` tracks known fleet directories in `known_fleets`. No path bookkeeping per repo.
- **One-shot mode** — `--fleet.path <dir>` bypasses discovery for ad-hoc operations in any directory.
- **GitHub Enterprise** — `--github.host <host>` retargets the API and clone URLs at a GHE instance.
- **Concurrency built-in** — Parallel goroutines with a bounded worker pool, context-cancellable.
- **gorich progress bars** — Live progress bars with spinners, per-repo status, and M-of-N counts.
- **Single binary** — Written in Go, only `git` on `$PATH` is required.

## Quick Start

### 1. Set your token

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
```

A token with `repo` scope is needed for private repos; public repos work unauthenticated at lower rate limits.

### 2. Initialize config

```bash
minifleet init --token ghp_xxxx
# or: minifleet init -s   # show current configuration
```

### 3. Discover and create a manifest

```bash
minifleet discover depado
# Filter with flags:
minifleet discover depado --visibility public --language go
```

Fetches repositories from the GitHub API, writes `fleet.yml` to the current directory, and registers the fleet in `config.yml`. Does **not** clone anything.

### 4. Sync (clone/pull)

```bash
minifleet sync depado
# or, from anywhere if the fleet is known:
minifleet sync
```

Repos listed in `fleet.yml` are cloned into the fleet directory (`~/dev/github.com/depado/<repo>`). Existing repos are pulled (`git fetch` + `git rebase --autostash`). Ignored repos (see manifest) are skipped. No API calls — works offline.

### 5. Local status across the fleet

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

`status` picks up the fleet in CWD when invoked inside a fleet directory, or iterates all `known_fleets` otherwise.

### 6. Open PRs with CI status

```bash
minifleet prs depado
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

Every command accepts the [global flags](#configuration). Commands that talk to the API or filter repos accept [filter flags](#filters).

### `minifleet discover`

```
minifleet discover <owner> [flags]
```

Fetch repositories from the GitHub API, apply filters, and create or update `fleet.yml` for the given owner. Merges with any existing manifest, preserving user-set fields (`labels`, `protocol`, `ignored`, `groups`). Does **not** clone or pull repositories — use `sync` for that.

```
Flags:
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
  --topic stringArray        filter by topic (repeatable)
  --include-archived         include archived repos
  --include-forks            include forked repos
  --visibility string        all, public, private (default: all)
  --language string          filter by primary language
  --label stringArray        filter by manifest label (key=value or key, repeatable)
  --group string             filter by manifest group
```

**Examples:**

```bash
# Discover all public Go repos
minifleet discover depado --visibility public --language go

# Discover with a custom path
minifleet --fleet.path ~/work/depado discover depado

# Refresh an existing manifest (re-fetch, re-apply filters, merge)
minifleet discover depado
```

Run `discover` again at any time to pick up new repos or refresh API-tracked metadata (`topics`, `language`, `archived`, etc.).

### `minifleet sync`

```
minifleet sync [owner] [flags]
```

Clone missing repos and pull existing ones from the `fleet.yml` manifest. Purely local — no API calls. When no owner is given, syncs the fleet in CWD (or all known fleets if not in one). Errors if no manifest exists (run `discover` first).

**Fleet directory resolution** (first match wins):

1. `--fleet.path <dir>` — explicit one-shot override
2. Current working directory
3. `known_fleets[owner]` in `config.yml`

After a successful sync, `known_fleets[owner]` is updated in `config.yml` so the fleet is discoverable from any directory.

```
Flags:
  --format, -f string        table, json (default: table)
```

**Examples:**

```bash
# Sync a single fleet
minifleet sync depado

# Sync all known fleets at once
minifleet sync

# GitHub Enterprise
minifleet --github.host github.example.com sync my-org

# Shallow clones
minifleet --fleet.shallow sync depado
```

### `minifleet list`

```
minifleet list [owner] [flags]
```

List repositories. Uses the local manifest when available; falls back to fetching from the GitHub API if no manifest exists. Without an owner, lists repos from the fleet in CWD (or all known fleets). Output as `table` (default), `json`, or `yaml`.

```
Flags:
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
  --topic stringArray        filter by topic
  --include-archived         include archived repos
  --include-forks            include forked repos
  --visibility string        all, public, private (default: all)
  --language string          filter by primary language
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --format, -f string        table, json, yaml (default: table)
  --limit int                max repos to list (default: 1000)
```

```bash
# List from manifest (in a fleet directory)
minifleet list

# List from API fallback (no manifest exists)
minifleet list depado --language go

# JSON output
minifleet list depado --format json | jq '.[] | .name'

# Generate a seed manifest
minifleet list depado --format yaml > fleet.yml
```

### `minifleet status`

```
minifleet status [flags]
```

Show git status for repos in the fleet. Uses the manifest as the source of truth for which repos to check; falls back to scanning the filesystem if no manifest exists. Operates on the fleet in CWD when one is present, otherwise iterates all known fleets. Repos not yet cloned are skipped.

```
Flags:
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
  --topic stringArray        filter by topic (via manifest)
  --include-archived         include repos flagged as archived in manifest
  --include-forks            include repos flagged as fork in manifest
  --language string          filter by language (via manifest)
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --has-file stringArray     require file to exist in repo dir (repeatable, AND)
  --if string                shell command; exit 0 = include repo
  --format, -f string        table, json (default: table)
```

### `minifleet prs`

```
minifleet prs [owner] [flags]
```

List open pull requests across repositories with CI and review status. Repo list comes from the manifest when available (API fallback if none exists). PR data is always fetched from the API. Without an owner, shows PRs for the fleet in CWD (or all known fleets).

```
Flags:
  --state string             open, closed, all (default: open)
  --author, -a string        filter by PR author
  --no-draft                 exclude draft PRs
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
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

Run a shell command in every local repository directory (or a filtered subset). Uses the manifest as the source of truth; falls back to filesystem scan if none exists. Repos not yet cloned are skipped. Operates on the fleet in CWD or all known fleets (same discovery as `status`).

```
Flags:
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
  --topic stringArray        filter by topic (via manifest)
  --include-archived         include archived repos
  --include-forks            include forked repos
  --language string          filter by primary language (via manifest)
  --label stringArray        filter by manifest label
  --group string             filter by manifest group
  --has-file stringArray     require file to exist in repo dir (repeatable, AND)
  --if string                shell command; exit 0 = include repo
  --summary                  force summary mode (one line per repo after completion)
  --progress                 force live block mode (animated spinners + streaming output)
  --block-lines int          output lines per repo block in live mode (default: 3)
  --dry-run                  print what would run; do not execute
  --shell string             shell to invoke (default: sh)
  --format string            output format: table (auto), json
```

Use `--` to separate flags from the command itself.

**Examples:**

```bash
# Run Go tests across the fleet
minifleet run -- "go test ./..."

# Lint only backend repos
minifleet run --group backend -- "make lint"

# Stream output of a build (force live blocks)
minifleet run --progress --language go -- "make build"

# Cross-repo code search
minifleet run -- "grep -r 'TODO' ."

# Only repos with specific files
minifleet run -H go.mod -H Dockerfile -- "make build"

# Only repos where a dependency check passes
minifleet run --if 'grep -q "go 1.22" go.mod' -- "go vet ./..."

# Dry-run a destructive bulk change
minifleet run --dry-run --include-regex "^old-" -- "rm -f .env.local"
```

**Summary mode** (`--summary`): one line per repo (`✓`/`✗ exit N` + duration); failed repos also print their captured stderr and stdout. This is the default when stdout is not a terminal (piped or redirected).

**Live block mode** (`--progress`, or TTY default): when stdout is a terminal, each repo gets a growing block that updates in place:

```
→ articles
  3354578 chore: remove drone config...
  26742bf Bump github.com/gin-gonic/gin...
→ bfplus
  53c117a chore(deps): update go toolchain...
  5ed469d fix(deps): update module...
```

When a repo finishes, its header flips to `✓ repo (elapsed)` (or `✗ exit N repo (elapsed)` on failure) and the last `--block-lines` output lines stay visible underneath. New blocks are appended as repos are picked up. Older blocks scroll off the top when the display exceeds terminal height. In a non-terminal (piped, CI), falls back to `repo › line` prefixes.

### `minifleet init`

```
minifleet init [flags]
```

Write `~/.config/minifleet/config.yml` with defaults, or show the current configuration including registered fleets.

```
Flags:
  -t, --token string   GitHub personal access token to write
  -s, --show           print current configuration
```

## Fleet directories

A **fleet directory** is any directory that contains a `fleet.yml`. It typically also contains the cloned repos it describes, side-by-side. The directory IS the fleet.

```
~/dev/github.com/depado/
├── fleet.yml       ← manifest for the depado fleet
├── articles/       ← cloned repo
├── buoy/           ← cloned repo
├── minifleet/      ← cloned repo
└── ...
```

`config.yml` records known fleet directories in `fleet.known_fleets` so commands run from outside a fleet dir can still find them:

```yaml
fleet:
  known_fleets:
    depado: /home/depado/dev/github.com/depado
    work-org: /home/depado/work/github.com/work-org
```

Commands discover the active fleet(s) in this order:

1. `--fleet.path <dir>` (explicit override)
2. CWD has a `fleet.yml` (use CWD)
3. all `known_fleets` (iterate)

`discover <owner>` creates the manifest and registers the fleet in `known_fleets`. `sync <owner>` additionally consults `known_fleets[owner]` as a fallback and re-registers the directory on success.

## Filters

Every command that queries the API or filters repos accepts the same flags:

| Flag                 | Type        | Behavior                                                                             |
| -------------------- | ----------- | ------------------------------------------------------------------------------------ |
| `--include-regex`    | regex       | Match on repo name                                                                   |
| `--exclude-regex`    | regex       | Exclude repos whose name matches (wins over includes)                                |
| `--include`          | stringArray | Include repo by exact name (repeatable)                                              |
| `--exclude`          | stringArray | Exclude repo by exact name (repeatable, wins over includes)                          |
| `--topic`            | stringArray | Match if repo has any of the given topics (OR)                                       |
| `--include-archived` | bool        | Include archived repos (excluded by default)                                         |
| `--include-forks`    | bool        | Include forked repos (excluded by default)                                           |
| `--visibility`       | string      | `all` (default), `public`, `private`                                                 |
| `--language`         | string      | Match on repo primary language (e.g. `go`, `python`)                                 |
| `--label`            | stringArray | Match on manifest labels: `tier=1` (exact) or `tier` (any value). AND across labels. |
| `--group`            | string      | Limit to repos in a manifest group                                                   |
| `--has-file`         | stringArray | Include only repos with given file (e.g. `go.mod`, `Makefile`). AND logic, local-only. |
| `--if`               | string      | Shell command; exit 0 = include, non-zero = exclude. Runs in parallel, local-only.   |

Filters are available on `discover`, `list`, `prs`, `status`, and `run`. `sync` does not accept filters — it operates on all repos in the manifest. `--has-file` and `--if` only apply when repos are cloned locally (`run`, `status`, manifest-backed `list`/`prs`).

Filters compose freely:

```bash
# Discover Go services with tier=1 label in the backend group
minifleet discover depado --language go --label tier=1 --group backend

# Status of those repos
minifleet status --group backend --label tier=1

# Only repos with a go.mod and main.go
minifleet run -H go.mod -H main.go -- "go test ./..."

# Only repos where go.mod pins a specific dependency
minifleet run --if 'grep -q "github.com/foo/bar v2" go.mod' -- "make build"

# Combine: Go repos with a Makefile, using a specific lib version
minifleet run -H go.mod -H Makefile --if 'grep -q "github.com/foo/bar v2" go.mod' -- "make build"
```

`--label` and `--group` consult the manifest; `--include-regex`, `--exclude-regex`, `--include`, `--exclude`, `--topic`, `--include-archived`, `--include-forks`, `--visibility`, and `--language` work from the manifest data or API response.

## One-shot mode

Pass `--fleet.path <dir>` to operate on any directory as if it were a fleet directory, bypassing the normal CWD/known_fleets discovery:

```bash
# Discover repos directly into a custom directory
minifleet --fleet.path ~/scratch/my-org discover depado --visibility public

# Sync those same repos
minifleet --fleet.path ~/scratch/my-org sync depado

# Status of those repos
minifleet --fleet.path ~/scratch/my-org status

# Run a command in them
minifleet --fleet.path ~/scratch/my-org run -- "git log --oneline -1"
```

In one-shot mode, all commands treat `<dir>` as the fleet directory: a `fleet.yml` is read if present, repos are discovered/scanned directly inside it. `discover` and `sync` register the directory in `known_fleets` on success.

## Shallow clones

`--fleet.shallow` toggles `git clone --depth 1 --filter=blob:none` for speed:

```bash
minifleet --fleet.shallow sync depado
```

Shallow clones are smaller and faster but lack full history. They cannot push most commits without unshallowing. Use for one-shots, dashboards, or large fleets where you only need the latest tree. The default (full clone) is recommended for fleets you intend to push from.

Shallow is also available as a config value `fleet.shallow: true`.

## Manifest File

A `fleet.yml` lives at the root of a fleet directory. It is the source of truth for which repos belong to the fleet and carries metadata that GitHub's API doesn't track. Create one with `discover` and edit it to add groups, labels, protocols, and ignored repos.

```bash
minifleet discover depado --visibility public
```

Then edit the generated `fleet.yml`:

```yaml
version: "1"
owner: depado

groups:
  backend:
    - depado/svc-api
    - depado/svc-auth
  frontend:
    - depado/web-app

repos:
  - full_name: depado/svc-api
    protocol: ssh
    labels:
      tier: "1"
      language: go
  - full_name: depado/web-app
    protocol: https
    labels:
      tier: "2"
  - full_name: depado/old-prototype
    ignored: true
    labels:
      status: deprecated
```

### Field ownership

| Category    | Fields                                                            | Owner                                   |
| ----------- | ----------------------------------------------------------------- | --------------------------------------- |
| API-tracked | `topics`, `language`, `archived`, `fork`, `private`, `updated_at` | `discover` overwrites from API          |
| User-set    | `labels`, `protocol`, `ignored`                                   | User — preserved across `discover` runs |
| User-set    | `groups`                                                          | User — never touched by `discover`      |

### Groups

Use groups to scope filters:

```bash
minifleet status --group backend
minifleet run --group frontend -- "make lint"
minifleet discover depado --group backend
```

### Per-repo protocol

Each repo can declare `protocol: ssh` or `protocol: https`. `sync` reads this when cloning. Without a saved protocol, `sync` tries SSH first and falls back to HTTPS.

### Ignored repos

Set `ignored: true` to skip a repo in all bulk operations. Useful for archived or experimental repos you keep locally but don't want synced.

## Configuration

All settings can be set via CLI flags, environment variables, or a config file. Precedence: **Flags > Env > Config File > Defaults**.

### Config file (`~/.config/minifleet/config.yml`)

```yaml
github:
  token: "" # GitHub PAT (or use GITHUB_TOKEN env)
  host: github.com # Use a custom host for GitHub Enterprise

fleet:
  path: "" # (optional) one-shot override; bypass discovery
  shallow: false # Use shallow clones by default
  concurrent: 5 # Max concurrent operations
  known_fleets: # owner → directory of registered fleets
    depado: /home/depado/dev/github.com/depado

log:
  level: info # debug, info, warn, error
  format: text # json or text
  source: false
  color: auto # auto, always, never

ui:
  progress: true
  color: true
```

### Reference

| Key                  | Env                          | Default      | Description                                                  |
| -------------------- | ---------------------------- | ------------ | ------------------------------------------------------------ |
| `github.token`       | `GITHUB_TOKEN`               | -            | GitHub personal access token                                 |
| `github.host`        | `MINIFLEET_GITHUB_HOST`      | `github.com` | GitHub host (GHE: `github.example.com`)                      |
| `fleet.path`         | `MINIFLEET_FLEET_PATH`       | -            | One-shot directory override                                  |
| `fleet.shallow`      | `MINIFLEET_FLEET_SHALLOW`    | `false`      | Use shallow clones                                           |
| `fleet.concurrent`   | `MINIFLEET_FLEET_CONCURRENT` | `5`          | Max concurrent operations                                    |
| `fleet.known_fleets` | -                            | -            | Map of owner → fleet directory (managed by `discover`/`sync`) |
| `log.level`          | `MINIFLEET_LOG_LEVEL`        | `info`       | `debug`, `info`, `warn`, `error`                             |
| `log.format`         | `MINIFLEET_LOG_FORMAT`       | `text`       | `json` or `text`                                             |
| `log.source`         | `MINIFLEET_LOG_SOURCE`       | `false`      | Include source file in logs                                  |
| `log.color`          | `MINIFLEET_LOG_COLOR`        | `auto`       | `auto`, `always`, `never`                                    |
| `ui.progress`        | `MINIFLEET_UI_PROGRESS`      | `true`       | Show progress bars                                           |
| `ui.color`           | `MINIFLEET_UI_COLOR`         | `true`       | Enable colored output                                        |

## Concurrency

Every bulk operation uses the same bounded-goroutine executor. Configure it globally or per-command:

```bash
# Clone with 10 concurrent git processes
minifleet --fleet.concurrent 10 sync depado

# Fetch PRs with 3 concurrent API calls (staying under secondary rate limits)
minifleet --fleet.concurrent 3 prs depado
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
│   ├── config.go              # init command, SaveConf, RegisterFleet, printConfig
│   ├── flags.go               # Persistent flag definitions
│   ├── filters.go             # Filters struct + Apply — shared by every command
│   ├── fleet.go               # fleetTarget discovery (CWD / known_fleets / --path), manifestToTasks, reposForTarget
│   ├── discover.go            # discover command — API → fleet.yml
│   ├── shared.go              # printBulkSummary helper
│   ├── sync.go                # clone+pull from manifest; syncFromManifest
│   ├── list.go
│   ├── status.go
│   ├── prs.go
│   ├── run.go                 # execute shell command across repos + gorich live block display
│   └── version.go
├── internal/
│   ├── provider/              # Provider interface (Host, CloneURL, ListRepos, ...)
│   │   └── github/            # GitHub REST API client
│   ├── git/                   # System git operations (clone, pull, status)
│   ├── fleet/                # Executor (bounded concurrency) + Scanner (flat)
│   ├── manifest/             # Single-owner YAML manifest (load/save/merge/generate)
│   └── ui/                   # gorich table helpers
```

### Design principles

- **Local-first**: repos live next to `fleet.yml`; the directory IS the fleet. No central registry needed.
- **API / local separation**: `discover` talks to the API; `sync`, `status`, and `run` are purely local. The manifest is the explicit bridge.
- **Discoverable**: `known_fleets` in `config.yml` lets commands run from anywhere; CWD is checked first.
- **DRY**: every command is short. Filters, executor, and discovery are shared via `manifestToTasks` and `reposForTarget`.
- **LEAN**: one interface (`Provider`) for platform abstraction; function types elsewhere.
- **No global state**: configuration flows from `PersistentPreRunE` → context → command `RunE`.
- **Concurrency by default**: the `Executor` handles parallelism for every command.
- **Continue on error**: one repo failing never aborts the whole operation.

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
