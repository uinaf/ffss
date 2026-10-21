# Security

## What leaves the machine

The whole frozen bundle, deleted lines and context included, goes to the
provider unscanned for credentials. Binaries are never sent (tracked ones
appear as Git's summary line, untracked as size and SHA-256). Changed `.env`
files go with key names only; their values are redacted. Environment
templates (`.env.example`, `.sample`, `.template`) go whole. Codex also reads a
redacted snapshot of the repository by default, so any file it opens leaves
too; `--repository-access=false` keeps it to the bundle. Make sure the target
may go to that provider; if the harness asks for approval, say what leaves the
machine and to whom.

slopguard's refusals (sensitive path, size, binary, invalid UTF-8, symlink,
revision, capability, source change) are the boundary. Report a refusal as a
blocker unless the change itself was wrong and you fix it. A review covers
only the target it ran on, so a reduced or split bundle is a partial review;
say so.

Web access lets repo content steer the reviewer to attacker-chosen URLs.
Turn it off for targets that must stay off the network.

## Filing a slopguard bug

Search first:
`gh issue list --repo uinaf/ffss --state all --search "<summary> in:title"`.
Show the user the sanitized issue before `gh issue create --repo uinaf/ffss`.

It holds only: version, OS and arch, provider, web-access state, failure
class (plus protocol reason and rule), a repro on public or synthetic input,
and expected versus actual behavior. Nothing from the bundle, prompt, source,
diff, repo, paths, credentials, environment, or raw provider output.

Security bugs (secret exposure, injection, path traversal, sandbox escape,
bundle-boundary failures, malformed output accepted as clean) go to private
vulnerability reporting on the `uinaf/ffss` Security tab, same exclusions.
