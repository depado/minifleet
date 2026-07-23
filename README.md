<p align="center">
  <img alt="minifleet" src="https://shieldcn.dev/header/grid.svg?title=minifleet&subtitle=Minimal+fleet+management+for+your+repositories.&mode=dark&align=left&border=false&logo=lu%3ASailboat">
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
  - [`minifleet fetch`](#minifleet-fetch)
  - [`minifleet sync`](#minifleet-sync)
  - [`minifleet list`](#minifleet-list)
  - [`minifleet status`](#minifleet-status)
  - [`minifleet prs`](#minifleet-prs)
  - [`minifleet run`](#minifleet-run)
  - [`minifleet init`](#minifleet-init)
- [Fleet directories](#fleet-directories)
- [Filters](#filters)
- [Shallow clones (`--shallow`)](#shallow-clones)
- [Plan files (`--plan`)](#plan-files)
- [Manifest File](#manifest-file)
- [Configuration](#configuration)
- [Concurrency](#concurrency)
- [Development](#development)

## Features

- **Discover**: Fetch repositories from the GitHub API, apply filters, and create or update a `fleet.yml` manifest. No cloning: separate API concerns from local operations.
- **Sync**: Clone missing repos and pull existing ones based on the `fleet.yml` manifest. Purely local: no network calls, no token needed, works offline.
- **Run across repos**: `run -- "<command>"` executes a shell command in every repository directory (or a filtered subset), with summary or live block output.
- **Local-first listing**: `list` shows repos from the manifest when available, with an API fallback when no manifest exists. Output as table or JSON.
- **Local status dashboard**: See the state of all cloned repos: current branch, commits ahead/behind remote, uncommitted changes, stashed changes.
- **Cross-repo PR dashboard**: List open pull requests across all repos with CI status (success/pending/failure) and review status (approved/changes/pending). Repo list comes from the manifest; PR data from the API.
- **Unified filters**: Metadata filters (name, topic, language, labels, groups) apply to all commands. Local filters (file existence, git status) apply to `status`, `run`, and `fetch`.
- **Plan files**: Save filter presets and command options to a YAML file with `--plan`. CLI flags override plan values, making it easy to define and reuse common workflows.
- **Per-directory fleets**: A `fleet.yml` lives alongside the repos it describes. Known fleets are registered in minifleet's configuration unless instructed otherwise.
- **Target directory**: `--path <dir>` sets the working directory for all operations; disables known fleets lookups and overrides `--all`.
- **GitHub Enterprise**: `--github.host <host>` retargets the API and clone URLs at a GHE instance.
- **Concurrency built-in**: Parallel goroutines with a bounded worker pool, context-cancellable.
- **Interactive & non-interactive modes**: Progress bars and live blocks in terminals (`--interactive auto|always|never`); structured slog output with `--interactive never` or `--json`.
- **Single binary**: Written in Go, only `git` on `$PATH` is required.

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

Repos listed in `fleet.yml` are cloned into the fleet directory (`~/dev/github.com/depado/<repo>`). Existing repos are pulled (`git fetch` + `git rebase --autostash`). Ignored repos (see manifest) are skipped. No API calls: works offline.

### 5. Local status across the fleet

```bash
minifleet status
```

```
┏━━━━━━━━━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━┳━━━━━━━━━━━┳━━━━━━━┳━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━┓
┃ Repo         ┃ Behind ┃ Ahead ┃ Dirty ┃ Untracked ┃ Stash ┃ Branch ┃ Remote               ┃
┡━━━━━━━━━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━╇━━━━━━━━━━━╇━━━━━━━╇━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━┩
│ svc-api      │ 0      │ 3     │ no    │ 0         │ 0     │ main   │ git@github.com:org/a │
│ svc-auth     │ 5      │ 0     │ yes   │ 2         │ 0     │ dev    │ git@github.com:org/b │
│ web-app      │ 0      │ 0     │ no    │ 0         │ 0     │ main   │ git@github.com:org/c │
└──────────────┴────────┴───────┴───────┴───────────┴───────┴────────┴──────────────────────┘
```

`status` picks up the fleet in the current directory when invoked inside a fleet directory, or iterates all known fleets otherwise. Branches are highlighted yellow when they differ from the remote default.

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

Fetch repositories from the GitHub API, apply filters, and create or update `fleet.yml` for the given owner. Merges with any existing manifest, preserving user-set fields (`labels`, `protocol`, `ignored`, `groups`). Does **not** clone or pull repositories: use `sync` for that.

```
Flags:
  --no-register              do not register this fleet in config (fleets)
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
minifleet --path ~/work/depado discover depado

# Refresh an existing manifest (re-fetch, re-apply filters, merge)
minifleet discover depado

# Discover without registering in global config
minifleet discover depado --no-register
```

Run `discover` again at any time to pick up new repos or refresh API-tracked metadata (`topics`, `language`, `archived`, etc.).

### `minifleet fetch`

```
minifleet fetch [owner] [flags]
```

Run `git fetch origin --prune --tags` in every cloned repository. Purely local: no API calls. Without an owner, operates on the fleet in the current directory (or all known fleets). With an owner, fetches repositories for that specific fleet. Accepts local filter flags.

```
Flags:
  --force                    force fetch, overwriting diverged local tags
  --include-regex string     regex to match repo names
  --exclude-regex string     regex to exclude repo names
  --include stringArray      include repo by exact name (repeatable)
  --exclude stringArray      exclude repo by exact name (repeatable)
  --has-file stringArray     require file to exist in repo dir (repeatable, AND)
  --dirty                    only repos with uncommitted changes
  --ahead int                only repos with at least N ahead commits
  --behind int               only repos with at least N behind commits
  --wip                      only repos with any uncommitted, unpushed, or unpulled changes
```

**Examples:**

```bash
# Fetch all repos in the current fleet
minifleet fetch

# Fetch a specific fleet
minifleet fetch depado

# Fetch only repos ahead of remote
minifleet fetch --ahead 1

# Fetch with a target directory
minifleet --path ~/work/depado fetch
```

### `minifleet sync`

```
minifleet sync [owner] [flags]
```

Clone missing repos and pull existing ones from the `fleet.yml` manifest. Purely local: no API calls. When no owner is given, syncs the fleet in the current directory (or all known fleets if not in one). Errors if no manifest exists (run `discover` first).

**Fleet directory resolution** (first match wins):

1. `--path <dir>`: explicit directory override (takes precedence over --all)
2. Current working directory
3. `fleets[owner]` in `config.yml`

After a successful sync, `fleets[owner]` is updated in `config.yml` so the fleet is discoverable from any directory.

**Examples:**

```bash
# Sync a single fleet
minifleet sync depado

# Sync all known fleets at once
minifleet sync

# GitHub Enterprise
minifleet --github.host github.example.com sync my-org

# Shallow clones
minifleet --shallow sync depado
```

### `minifleet list`

```
minifleet list [owner] [flags]
```

List repositories. Uses the local manifest when available; falls back to fetching from the GitHub API if no manifest exists. Without an owner, lists repos from the fleet in the current directory (or all known fleets). Output as table (default) or JSON.

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
  --limit int                max repos to list (0 = unlimited, default: 0)
```

```bash
# List from manifest (in a fleet directory)
minifleet list

# List from API fallback (no manifest exists)
minifleet list depado --language go

# JSON output
minifleet list --json | jq '.[] | .name'
```

### `minifleet status`

```
minifleet status [flags]
```

Show git status for repos in the fleet. Uses the manifest as the source of truth for which repos to check; falls back to scanning the filesystem if no manifest exists. Operates on the fleet in the current directory when one is present, otherwise iterates all known fleets. Repos not yet cloned are skipped.

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
  --dirty                    only repos with uncommitted changes
  --ahead int                only repos with at least N ahead commits
  --behind int               only repos with at least N behind commits
  --wip                      only repos with any uncommitted, unpushed, or unpulled changes
```

### `minifleet prs`

```
minifleet prs [owner] [flags]
```

List open pull requests across repositories with CI and review status. Repo list comes from the manifest when available (API fallback if none exists). PR data is always fetched from the API. Without an owner, shows PRs for the fleet in the current directory (or all known fleets).

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
```

### `minifleet run`

```
minifleet run -- <command> [flags]
```

Run a shell command in every local repository directory (or a filtered subset). Uses the manifest as the source of truth; falls back to filesystem scan if none exists. Repos not yet cloned are skipped. Operates on the fleet in the current directory or all known fleets (same discovery as `status`).

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
  --dirty                    only repos with uncommitted changes
  --ahead int                only repos with at least N ahead commits
  --behind int               only repos with at least N behind commits
  --wip                      only repos with any uncommitted, unpushed, or unpulled changes
  --block-lines int          output lines per repo block in live mode (default: 3)
  --dry-run                  print what would run; do not execute
  --shell string             shell to invoke (default: sh)
```

`--json` is a persistent global flag. Use `--interactive` to control display mode (auto/always/never).

Use `--` to separate flags from the command itself.

**Examples:**

```bash
# Run Go tests across the fleet
minifleet run -- "go test ./..."

# Lint only backend repos
minifleet run --group backend -- "make lint"

# Stream output of a build
minifleet run --block-lines 5 --language go -- "make build"

# Cross-repo code search
minifleet run -- "grep -r 'TODO' ."

# Only repos with specific files
minifleet run -H go.mod -H Dockerfile -- "make build"

# Only repos where a dependency check passes
minifleet run --if 'grep -q "go 1.22" go.mod' -- "go vet ./..."

# Dry-run a destructive bulk change
minifleet run --dry-run --include-regex "^old-" -- "rm -f .env.local"

# Load filters and command from a plan file
minifleet run --plan plan.yml

# JSON output
minifleet run --json -- "git rev-parse HEAD"

# Non-interactive (slog output)
minifleet run --interactive never -- "make test"
```

**Live block mode** (TTY default): when stdout is a terminal, each repo gets a growing block that updates in place:

```
→ articles
  3354578 chore: remove drone config...
  26742bf Bump github.com/gin-gonic/gin...
→ bfplus
  53c117a chore(deps): update go toolchain...
  5ed469d fix(deps): update module...
```

When a repo finishes, its header flips to `✓ repo (elapsed)` (or `✗ exit N repo (elapsed)` on failure) and the last `--block-lines` output lines stay visible underneath. New blocks are appended as repos are picked up. Older blocks scroll off the top when the display exceeds terminal height. In a non-terminal (piped, CI, `--interactive never`), per-repo status and captured output is written to stderr via slog.

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

`config.yml` records fleet directories in `fleets` so commands run from outside a fleet dir can still find them:

```yaml
fleets:
  depado: /home/depado/dev/github.com/depado
  work-org: /home/depado/work/github.com/work-org
```

Commands discover the active fleet(s) in this order:

1. `--path <dir>` (explicit override, takes precedence over --all)
2. The current directory has a `fleet.yml` (use it)
3. all known fleets (iterate)

`discover <owner>` creates the manifest and registers the fleet in `fleets`. `sync <owner>` additionally consults `fleets[owner]` as a fallback and re-registers the directory on success. Pass `--no-register` to `discover` to prevent registration: the fleet will only be found when running from its directory.

### Target directory

Pass `--path <dir>` to operate on a specific directory as the fleet target, overriding the current-directory and known fleets lookups. When `--path` is set, `--all` is ignored:

```bash
minifleet --path ~/scratch/my-org discover depado --visibility public
minifleet --path ~/scratch/my-org sync depado
minifleet --path ~/scratch/my-org status
minifleet --path ~/scratch/my-org run -- "git log --oneline -1"
```

All commands treat `<dir>` as the fleet directory: a `fleet.yml` is read if present, repos are discovered/scanned directly inside it. `discover` and `sync` register the directory in `fleets` on success (unless `--no-register` is used).

## Filters

Filters fall into two categories. Every command that queries the API or filters repos accepts the metadata filters. The local filters are only available on commands that operate on cloned repositories (`status`, `run`, `fetch`).

### Metadata filters (applies to `discover`, `list`, `prs`, `status`, `run`)

These filters work against API responses or manifest data, so they apply both online and offline:

| Flag                 | Type        | Behavior                                                                             |
| -------------------- | ----------- | ------------------------------------------------------------------------------------ |
| `--include-regex`    | regex       | Match on repo name                                                                   |
| `--exclude-regex`    | regex       | Exclude repos whose name matches (wins over includes)                                |
| `--include`          | stringArray | Include repo by exact name (repeatable)                                              |
| `--exclude`          | stringArray | Exclude repo by exact name (repeatable, wins over includes)                          |
| `--topic`            | stringArray | Match if repo has any of the given topics (OR)                                       |
| `--include-archived` | bool        | Include archived repos (excluded by default)                                         |
| `--include-forks`    | bool        | Include forked repos (excluded by default)                                           |
| `--visibility`       | string      | `all` (default), `public`, `private` — API-only, ignored by local commands           |
| `--language`         | string      | Match on repo primary language (e.g. `go`, `python`)                                 |
| `--label`            | stringArray | Match on manifest labels: `tier=1` (exact) or `tier` (any value). AND across labels. |
| `--group`            | string      | Limit to repos in a manifest group                                                   |

### Local filters (applies to `status`, `run`, `fetch`)

These inspect the local checkout and are ignored by commands that work against the API:

| Flag         | Type        | Behavior                                                                         |
| ------------ | ----------- | -------------------------------------------------------------------------------- |
| `--has-file` | stringArray | Include only repos with given file (e.g. `go.mod`). AND logic.                   |
| `--if`       | string      | Shell command; exit 0 = include, non-zero = exclude. Runs in parallel            |
| `--dirty`    | bool        | Only repos with uncommitted changes to tracked files                             |
| `--ahead`    | int         | Only repos with at least N ahead commits                                         |
| `--behind`   | int         | Only repos with at least N behind commits                                        |
| `--wip`      | bool        | Only repos with uncommitted changes, ahead/behind commits, or off-default branch |

`sync` does not accept filters: it always operates on all repos in the manifest.

Filters compose freely and can also be declared in a [plan file](#plan-files).

```bash
# Metadata: discover Go services in the backend group
minifleet discover depado --language go --group backend

# Metadata: status of repos with tier=1 label
minifleet status --group backend --label tier=1

# Local: only repos with go.mod and ahead commits
minifleet run -H go.mod --ahead 1 -- "go test ./..."

# Local: check repos with unpushed work
minifleet status --ahead 1
```

## Shallow clones

`--shallow` toggles `git clone --depth 1 --filter=blob:none` for speed:

```bash
minifleet --shallow sync depado
```

Shallow clones are smaller and faster but lack full history. They cannot push most commits without unshallowing. Use for one-shots, dashboards, or large fleets where you only need the latest tree. The default (full clone) is recommended for fleets you intend to push from.

## Plan files

Pass `--plan / -p <file>` to load filters, command options, and fleet targeting from a YAML file instead of (or in addition to) CLI flags. CLI flags override plan values, so you can reuse a plan as a default and tweak it per-run.

Any command that accepts filters supports `--plan`. Fields unrelated to a command are silently ignored: a plan written for `run` works with `status` too (the `command` and `shell` fields are ignored).

### Schema

```yaml
# Fleet targeting (optional: falls back to context)
fleet: my-org # target a specific fleet from fleets
all: true # operate on all known fleets
json: true # output as JSON

# run-specific (optional: ignored by other commands)
shell: bash # shell to invoke (default: sh)
command: "make test" # command to execute (no -- args needed)
block_lines: 5 # output lines per block in live mode
dry_run: false # print what would run; don't execute
interactive: never # auto, always, never

# Filters (optional: all 15 filter fields are supported)
filters:
  include_regex: "^svc-"
  exclude_regex: "legacy"
  include: [repo1, repo2]
  exclude: [repo3]
  topics: [go, rust]
  include_archived: false
  include_forks: false
  visibility: public
  language: Go
  labels: ["tier=1"]
  group: backend
  has_files: [go.mod, Dockerfile]
  if_cmd: "test -f go.mod"
  dirty: false
```

### Examples

```bash
# Define a reusable plan
cat > go-checks.yml << 'EOF'
shell: bash
command: "go vet ./... && go build ./... && go test -race -count=1 ./..."
block_lines: 8
filters:
  if_cmd: "test -f go.mod"
EOF

# Run it (no CLI args needed: command comes from the plan)
minifleet run --plan go-checks.yml

# Override a filter: only the backend group, not all Go repos
minifleet run --plan go-checks.yml --group backend

# Same plan, dry-run to see what would happen
minifleet run --plan go-checks.yml --dry-run

# Use the same filters with status (command & shell are ignored)
minifleet status --plan go-checks.yml --json

# Plan targeting a specific fleet
minifleet run --plan go-checks.yml  # resolves fleet from context
# vs: plan has fleet: "work-org" → targets that fleet specifically
```

### How it works

1. `--plan <file>` is a persistent flag on every command (like `--json` and `--all`)
2. The YAML is loaded once in the pre-run hook and stored in context
3. For each filter, command option, or fleet targeting field: the plan provides a default that CLI flags can override
4. `cmd.Flags().Changed("flag-name")` determines whether the user set the flag: if not, the plan value is used

This makes it easy to define and version-control standard workflows (CI-like checks, lint passes, bulk refactors) without remembering long flag combinations.

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

| Category    | Fields                                                            | Owner                                  |
| ----------- | ----------------------------------------------------------------- | -------------------------------------- |
| API-tracked | `topics`, `language`, `archived`, `fork`, `private`, `updated_at` | `discover` overwrites from API         |
| User-set    | `labels`, `protocol`, `ignored`                                   | User: preserved across `discover` runs |
| User-set    | `groups`                                                          | User: never touched by `discover`      |

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

concurrent: 0 # max concurrent operations (defaults to number of CPUs)
fleets: # owner → directory of registered fleets
  depado: /home/depado/dev/github.com/depado

log:
  level: info # debug, info, warn, error
  format: text # json or text
  source: false
  color: auto # auto, always, never

interactive: auto # auto, always, never
```

### Reference

| Key            | Env                     | Default      | Description                                                       |
| -------------- | ----------------------- | ------------ | ----------------------------------------------------------------- |
| `github.token` | `GITHUB_TOKEN`          | -            | GitHub personal access token                                      |
| `github.host`  | `MINIFLEET_GITHUB_HOST` | `github.com` | GitHub host (GHE: `github.example.com`)                           |
| `path`         | `MINIFLEET_PATH`        | -            | Fleet directory override (bypasses known fleets, even with --all) |
| `concurrent`   | `MINIFLEET_CONCURRENT`  | CPU count    | Max concurrent operations                                         |
| `fleets`       | -                       | -            | Map of owner → fleet directory (managed by `discover`/`sync`)     |
| `log.level`    | `MINIFLEET_LOG_LEVEL`   | `info`       | `debug`, `info`, `warn`, `error`                                  |
| `log.format`   | `MINIFLEET_LOG_FORMAT`  | `text`       | `json` or `text`                                                  |
| `log.source`   | `MINIFLEET_LOG_SOURCE`  | `false`      | Include source file in logs                                       |
| `log.color`    | `MINIFLEET_LOG_COLOR`   | `auto`       | `auto`, `always`, `never`                                         |
| `interactive`  | `MINIFLEET_INTERACTIVE` | `auto`       | `auto`, `always`, `never`                                         |

## Concurrency

Every bulk operation uses the same bounded-goroutine executor. Configure it globally or per-command:

```bash
# Clone with 10 concurrent git processes
minifleet --concurrent 10 sync depado

# Fetch PRs with 3 concurrent API calls (staying under secondary rate limits)
minifleet --concurrent 3 prs depado
```

- A bounded goroutine pool processes tasks from a channel.
- Each task runs independently; one failure doesn't block others.
- `Ctrl+C` stops new tasks but lets in-flight operations complete.
- Results are collected as succeeded, skipped, and failed with per-repo error messages.

**Rate limits:** GitHub's secondary rate limit allows ~100 concurrent requests and ~900 points/minute. For API-heavy commands like `prs`, keep `--concurrent` at 5 or lower. For local git operations (`sync`, `status`), higher values (10+) are fine: they're I/O-bound.

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
│   ├── filters.go             # Filters struct + Apply: shared by every command
│   ├── plan.go                # Plan file (--plan): YAML schema, LoadPlan, ApplyPlan
│   ├── fleet.go               # fleetTarget discovery (current dir / fleets / --path), manifestToTasks, reposForTarget
│   ├── discover.go            # discover command: API → fleet.yml
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
- **Discoverable**: `fleets` in `config.yml` lets commands run from anywhere; the current directory is checked first.
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

MIT: see [LICENSE](LICENSE).
