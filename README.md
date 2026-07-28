# gh-codeowners-ls

[![ci](https://github.com/cybo42/gh-codeowners-ls/actions/workflows/ci.yml/badge.svg)](https://github.com/cybo42/gh-codeowners-ls/actions/workflows/ci.yml)

A [GitHub CLI](https://cli.github.com) extension that maps a diff's changed
files to their `CODEOWNERS`, and marks exactly which files are yours — by
matching your login **and your GitHub team memberships**.

```
$ gh codeowners-ls
Diff: PR #482 "Add billing webhooks"  •  4 files  •  CODEOWNERS @ .github/CODEOWNERS (base: main)

@acme/platform   2 files  ← you
@acme/web        1 file
(unowned)        1 file

STATUS  FILE                          OWNERS            MINE
M       internal/billing/webhook.go   @acme/platform    ✓
A       internal/billing/retry.go     @acme/platform    ✓
M       web/src/Checkout.tsx          @acme/web
M       scripts/tmp.sh                (unowned)
```

## Install

```
gh extension install cybo42/gh-codeowners-ls
```

This downloads a precompiled binary for your platform from the latest
release — no local Go toolchain needed. Keep it up to date with:

```
gh extension upgrade codeowners-ls
```

To pin a specific version instead of tracking latest:

```
gh extension install cybo42/gh-codeowners-ls --pin v0.1.0
```

Optionally verify the binary's build provenance before running it:

```
gh attestation verify $(gh extension list --json path -q '.[] | select(.name=="codeowners-ls").path') --owner cybo42
```

Or, from a checkout of this repo (builds from source):

```
make install
```

## Usage

Run with no arguments inside a repo with an open PR for the current branch —
it auto-detects the PR, falling back to a local diff against the default
branch's merge-base when there's no PR.

```
gh codeowners-ls [<base>...<head>] [flags]

Diff selection
  --pr <number|url>     use a pull request's file list
  --base <ref>          base ref for a local diff (default: merge-base w/ default branch)
  --head <ref>          head ref for a local diff (default: HEAD)
  --staged              include only staged changes
  --include-uncommitted include working-tree changes in a local diff (default true)

Filtering
  --mine                only files owned by you (or your teams)
  --unowned             only files with no owner
  --owner <@o>          only files owned by the given user/team (repeatable)

Output
  --group               owner summary only
  --files               per-file table only
  --rules               show the matching CODEOWNERS pattern + line number
  --json                machine-readable output
  --no-color            disable color

Validation & CI
  --validate            report CODEOWNERS syntax/permission errors from GitHub
  --fail-on-unowned     exit 3 if any changed file has no owner
  --fail-on-empty       exit 4 if a filter (e.g. --mine) matches nothing

Misc
  --repo <[HOST/]O/R>   target repository
  --codeowners <path>   explicit CODEOWNERS file
```

Exit codes: `0` success, `1` runtime error, `2` usage error, `3` unowned files
present with `--fail-on-unowned`, `4` empty result with `--fail-on-empty`.
Filters never change the exit code on their own, so it's safe to pipe.

### Examples

```
# only the files that are actually yours to review
gh codeowners-ls --mine

# show which CODEOWNERS line matched
gh codeowners-ls --rules

# fail CI when a change touches unowned files
gh codeowners-ls --fail-on-unowned

# machine-readable output for scripting
gh codeowners-ls --json | jq '.files[] | select(.mine)'
```

### Missing team memberships?

If `--mine` looks wrong, it's likely a token scope issue — team matching
needs `read:org`:

```
gh auth refresh -s read:org
```

The tool degrades gracefully without it (login-only matching, with a
warning), rather than failing.

## Development

```
make build    # build the binary
make test     # run unit tests with the race detector
make check    # fmt, vet, tidy, test, lint — everything CI would run
make install  # build + install as a local gh extension
make smoke    # quick sanity check against this repo
make dist     # cross-compile the full release matrix into dist/
```

See `make help` for the full list of targets.

### Releasing

Releases are cut by pushing an annotated tag; `.github/workflows/release.yml`
(via `cli/gh-extension-precompile`) builds every platform, creates the GitHub
release, and uploads one binary per platform:

```
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Asset names must end in `{os}-{arch}` (`.exe` on Windows) — that's the suffix
`gh extension install` matches against, so don't rename them.

To rehearse a release without affecting the `install`/`upgrade` path, push a
tag with a hyphen, e.g. `v0.1.0-rc.1` — GitHub (and therefore `gh`) treats
any tag containing `-` as a prerelease, which `gh extension install` ignores
when resolving `latest`.

## Design

- CODEOWNERS parsing and gitignore-style matching come from
  [`hmarr/codeowners`](https://github.com/hmarr/codeowners) — GitHub's
  dialect, last-match-wins.
- GitHub auth, repo/PR resolution, and REST access come from
  [`cli/go-gh`](https://github.com/cli/go-gh).
- For PR sources, CODEOWNERS is read from the PR's **base** ref via the
  contents API — the file GitHub itself evaluates — not the local working
  tree.
