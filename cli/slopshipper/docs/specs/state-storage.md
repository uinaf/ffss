# State storage and payload transport

Status: implemented

## Problem

Agents follow `next_action` literally. When it suggests repository paths such
as `<review.json>`, transport payloads can survive the command and be swept
into a commit. Moving canonical state into every repository would reduce
sandbox friction, but would also fragment runs across checkouts, introduce
SQLite sidecars into worktrees, and make accidental commits more likely.

## Requirements

- R1: Canonical workflow state and evidence remain outside the repository by
  default.
- R2: CLI-generated commands never require a payload file in the repository.
- R3: Sandboxes can select an explicit writable database without changing the
  normal default.
- R4: State-location failures are actionable and never trigger a silent
  fallback.
- R5: Repository-local SQLite is accepted only when the database and all of its
  sidecars are untracked and ignored by Git.
- R6: Agents can inspect the resolved state location without opening or
  mutating the database.
- R7: Existing installations continue to find their current database.
- R8: A human who invokes a stdin command without piped input receives an
  immediate recovery message instead of a hanging process.

## Contract

### Stream command payloads

`next_action` uses stdin for structured payloads:

```text
slopshipper intake --file - --run='<run>'
slopshipper review --evidence - --run='<run>'
slopshipper deliver --evidence - --run='<run>'
```

Raw machine callers may continue to use `--input -`. Accepted evidence is
canonicalized into the SQLite event log. The CLI and skill never create or
recommend `intake.json`, `review.json`, `deliver.json`, or
`*.evidence.json` transport artifacts.

When `-` selects stdin and stdin is an interactive terminal, the CLI fails
immediately with `invalid_input`. The message says to pipe or redirect JSON and
points to the command schema; it does not wait indefinitely for terminal EOF.

### Resolve one explicit database

The database path resolves in this order:

1. Non-empty `SLOPSHIPPER_DB`.
2. `$XDG_DATA_HOME/slopshipper/slopshipper.sqlite`.
3. `~/.local/share/slopshipper/slopshipper.sqlite` when `XDG_DATA_HOME` is
   unset.

`XDG_DATA_HOME`, when set, must be absolute as required by the XDG contract.

Relative `SLOPSHIPPER_DB` values resolve from the Git worktree root, not the
caller's current subdirectory. The resolved absolute path is fixed for the
command. The CLI never falls back from one path to another after an open,
permission, migration, or locking failure.

Scope checks use the physical location of the nearest existing ancestor, so a
symlink cannot make a worktree path appear external or an external path appear
repository-local.

`XDG_STATE_HOME` is a reasonable semantic alternative, but it remains outside
the contract.

### Inspect storage without mutation

`slopshipper storage --json` is read-only and does not require the database to
exist. It returns:

```json
{
  "schema_version": 1,
  "path": "/absolute/path/slopshipper.sqlite",
  "source": "environment",
  "scope": "repository",
  "exists": false,
  "git_ignored": true
}
```

`source` is `environment` or `xdg-data`. `scope` is `user`, `repository`, or
`custom`. `git_ignored` is `true` or `false` for paths inside the current
worktree and `null` otherwise. Human output prints the path, source, and any
required recovery in one compact block.

### Fail closed inside a worktree

When the resolved database is inside the Git worktree, the CLI checks the
following paths with Git's normal exclusion rules before creating or opening
the database:

```text
<database>
<database>-wal
<database>-shm
```

All three must be untracked and ignored. The check applies to existing and
nonexistent paths using the equivalent of
`git check-ignore --quiet --no-index`, so an ignored parent directory such as
`/.slopshipper/` is sufficient. A local override therefore looks like:

```bash
export SLOPSHIPPER_DB=.slopshipper/slopshipper.sqlite
```

If any path is not ignored, the command fails with structured kind
`unsafe_state_path`. Its message identifies the path and offers two recoveries:
choose a writable path outside the worktree, or select
`.slopshipper/slopshipper.sqlite` after adding `/.slopshipper/` to the
repository-local Git exclude file.

The CLI does not edit `.gitignore`, `$GIT_COMMON_DIR/info/exclude`, or global
Git configuration. Exclusion is a repository or user decision, and an
explicit `SLOPSHIPPER_DB` remains required on every process that uses the local
database.

### Preserve dry-run and output guarantees

Dry runs use the same resolved path as the real command. They never create,
migrate, or update canonical database state. SQLite may create WAL coordination
sidecars for a read-only snapshot; repository-local safety therefore requires
the database, WAL, and shared-memory paths to be ignored before any open.
`init --dry-run` succeeds when the selected path does not exist; other dry runs
report that canonical state is unavailable.

With `--json`, storage and state-location failures use the standard error
envelope and keep stdout machine-readable. Plain errors include the resolved
path and one supported recovery. Neither form writes an ignore rule or creates
a fallback database.

## Acceptance criteria

- AC1: Intake, review, and delivery statuses suggest stdin commands and contain
  no JSON payload filename.
- AC2: A complete run driven by suggested commands creates no file in the
  worktree.
- AC3: With no override, every command continues to use the existing XDG data
  path and does not inspect or modify Git ignore files.
- AC4: The same relative `SLOPSHIPPER_DB` resolves to one path from the worktree
  root and any subdirectory.
- AC5: A repository-local override fails before database creation unless the
  database, WAL, and shared-memory paths are all untracked and ignored.
- AC6: An ignored `.slopshipper/` override supports normal commands, dry runs,
  and concurrent read-only `serve` access.
- AC7: An unwritable, invalid, or locked selected path returns a structured
  error without creating state elsewhere.
- AC8: `slopshipper storage --json` reports path resolution and Git safety
  without creating or migrating state.
- AC9: A command selecting stdin exits immediately with actionable
  `invalid_input` when stdin is a terminal.

## Constraints

- SQLite remains the only state authority.
- Existing schema versions, runs, and event evidence remain readable.
- WAL mode requires the database directory to support local file locking and
  creation of `-wal` and `-shm` sidecars.
- State selection remains deterministic across every command in one workflow.
- No command may mutate Git ignore configuration solely to make storage work.

## Non-goals

- Automatically choosing `.slopshipper/` when user-level storage is
  unavailable.
- Adding broad `*.evidence.json` patterns to repositories.
- Treating payload files as durable evidence or recovery checkpoints.
- Synchronizing state between machines, containers, clones, or worktrees.
- Moving existing databases to `XDG_STATE_HOME` in this change.
- Adding a remote state backend.

## Alternatives considered

| Alternative | Result | Reason |
| --- | --- | --- |
| Global XDG database | Chosen default | Keeps canonical audit state out of worktrees and available across commands. |
| `.slopshipper/` database | Explicit override | Useful in constrained sandboxes, but unsafe as an invisible fallback. |
| Automatic repo-local fallback | Rejected | A permission problem would silently select a different history. |
| Project `.gitignore` mutation | Rejected | Storage selection must not create a shared repository policy change. |
| `$GIT_COMMON_DIR/info/exclude` | Recommended local exclusion | Git defines it for repository-specific, user-local auxiliary files. |
| Broad `*.evidence.json` ignore | Rejected | Hides transport residue and can mask legitimate fixtures. |
| `XDG_STATE_HOME` migration | Deferred | Semantically plausible, but requires an explicit no-split migration contract. |

## External review

- The [XDG Base Directory specification](https://specifications.freedesktop.org/basedir/)
  supports an application-owned user data directory and a stable fallback,
  matching the existing global default. It also distinguishes state data;
  changing categories requires an explicit migration rather than implicit
  discovery of two locations.
- Git documents `$GIT_COMMON_DIR/info/exclude` as the correct home for
  repository-specific, user-local auxiliary files that should not be shared.
  A project `.gitignore` is appropriate only when every clone should carry the
  rule. See [gitignore](https://git-scm.com/docs/gitignore).
- Terraform demonstrates the valid alternative: per-working-directory data
  can default to a repository-local directory, with an environment override
  that must remain consistent across commands. Slopshipper differs because its
  SQLite event log is canonical audit state, not a reproducible working cache.
  See [`TF_DATA_DIR`](https://developer.hashicorp.com/terraform/cli/config/environment-variables#tf_data_dir).
- SQLite WAL mode can create persistent `-wal` and `-shm` files beside the
  database and requires compatible directory permissions and same-host shared
  memory. Ignoring only the main `.sqlite` file is therefore incomplete. See
  [SQLite WAL](https://sqlite.org/wal.html).
- Hosted automation exposes explicit workspace, home, and temporary locations
  rather than making tools infer a fallback. Callers should route
  `SLOPSHIPPER_DB` to the environment's intended writable storage. See
  [GitHub-hosted runner filesystems](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#file-systems).
