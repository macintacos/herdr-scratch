---
name: release
description: Use when the user wants to cut a release of herdr-scratch — "release a patch", "ship a minor", "cut a major", "tag a new version", "publish a release". Computes the next version with svu, drafts notes from the commit range, and runs the tag-and-publish sequence only after an explicit OK.
argument-hint: patch | minor | major
---

# Release

Take a bump word and turn it into a published release: a GitHub release with checksums and
notes, and a Homebrew cask in
[macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap) pointing at it.

The version is **derived** from the last tag by `svu`, never chosen. That is the whole
reason this skill exists: goreleaser runs on whatever tag is already there and never
creates one, so without a tool the next version is a number somebody types — and a wrong
tag is public the instant it is pushed.

[docs/RELEASING.md](../../../docs/RELEASING.md) is this same procedure written for a
person, and it is the canonical explanation of *why* each step is ordered as it is. This
file is that runbook executed. When one changes, change both.

## Arguments

The user invoked: `/release $ARGUMENTS`. `$ARGUMENTS` is exactly one bump word, and it is
`<bump>` everywhere below.

| `<bump>` | Example, from `v0.5.0` | Use when |
| -------- | ---------------------- | -------- |
| `patch`  | `v0.5.1`               | Fixes only; nothing a user has to do anything about. |
| `minor`  | `v0.6.0`               | New behaviour, or a change to what `link` / the popup does. |
| `major`  | `v1.0.0`               | A break — the manifest, the plugin root layout, or a removed command. |

The examples are illustrative — `v0.5.0` is whatever `svu current` reports at the time,
not a fact about the repo.

Anything else — prose, a version number, empty — **stops** and asks for one of the three.
A user who names a version directly ("release 0.7.0") is asking for the one thing this
skill will not do; say so and offer the bump word that produces it.

## Invariants

1. **The version is computed, never inferred.** It comes from `svu <bump>` and nothing
   else. Do not read `git tag -l` and reason about it, do not use `svu next` (which infers
   the bump from commit messages — the bump word is the user's call, not the log's), and
   do not accept a version the user types.
2. **The repo's checks are the user's job.** This skill does **not** run
   `mise run preflight`, `mise run lint`, or `go test`. It asks whether they were run and
   takes the answer. Running them "just to be safe" is the helpful instinct that violates
   this — don't.
3. **Nothing mutates before the OK.** Everything in § Draft is reads and a scratch file.
   The first mutation is § Execute step 1.
4. **A failing step stops the run.** Report what failed and what to do about it; never
   continue to the next step hoping it resolves.
5. **The tag is the point of no return.** Before it is pushed, everything is local and
   recoverable by ordinary means. After it, recovery is a public deletion — see §
   Recovery.

## Preconditions

Check all of these **before** computing anything, and report every failure plainly rather
than working around it. The last one is the difference between a release that ships and
one that only looks like it did.

```sh
mise exec -- goreleaser --version   # installed via mise.toml
mise exec -- svu --version          # same
mise exec -- goreleaser check       # the config still validates
git branch --show-current           # trunk
git status --porcelain              # empty — including untracked files
git fetch --prune --prune-tags origin && git rev-parse HEAD origin/trunk   # identical SHAs
gh api repos/macintacos/homebrew-tap/contents/Formula --jq '.[].name'
```

- **goreleaser or svu missing** → the tool is pinned in [mise.toml](../../../mise.toml),
  so this means mise has not installed it. Say `mise install` and stop; do not fall back
  to a system copy, which is not the pinned version.
- **Not on `trunk`, dirty, or ahead of / behind `origin/trunk`** → stop. A release is cut
  from the default branch's tip; a tag on anything else points at a tree nobody reviewed.
  *Dirty* means what goreleaser means by it: `git status --porcelain` empty,
  **untracked files included**. It refuses to release from a tree with so much as one
  stray file, and it refuses at Execute step 7 — after the tag is public.
- **Tags out of date** → the `--prune-tags` above is deliberate. `svu` computes from the
  newest tag it can see, so a local tag that was deleted from the remote (§ Recovery does
  exactly that) would silently push the next version a step too far. Pruning is safe here
  precisely because the checks above already demand a clean `trunk` level with
  `origin/trunk`, so there is no legitimate unpushed local tag to lose.
- **The token.** goreleaser reads `GITHUB_TOKEN` and it needs contents write on **both**
  `macintacos/herdr-scratch` (to create the release) and `macintacos/homebrew-tap` (to
  commit the cask). `$(gh auth token)` covers it when `gh` is authenticated. If neither
  `$GITHUB_TOKEN` nor `gh auth token` yields one, stop and say exactly that — do not start
  a sequence that will fail at its last and least recoverable step.
- **`herdr-scratch.rb` appears in that `Formula` listing** → **stop.** goreleaser writes
  `Casks/herdr-scratch.rb` and leaves the formula alone, and while both exist Homebrew
  resolves the bare `herdr-scratch` token to the **formula** and keeps building from
  source. The release would publish, the cask would be committed, and nobody would receive
  either. Deleting it is a manual step in the tap repo; ask the user to do it and stop.

  The check lists the directory rather than probing the file for a `404` on purpose. A
  missing file and an unreachable repository both come back `404` — so would a typo, an
  expired token, or a renamed tap — and reading "error" as "pass" is the worst possible
  failure on the one precondition that decides whether anyone receives the release. The
  listing **succeeds** when the tap is reachable, so its output is the assertion: the name
  present means stop, absent means pass, and a non-zero exit means the check did not run.

Then **ask the user to confirm the repo's checks were run** — `mise run preflight` covers
lint and tests in one. Their word is the gate (Invariant 2). If they have not, stop and
let them; there is no reason to race a release ahead of its own test suite.

## Draft

Nothing here writes to the repo.

**Compute both forms of the version in one shell.** `svu` prints a `v` prefix; the git tag
keeps it and `herdr-plugin.toml` does not. Getting this backwards is the single most
likely mistake in this skill, and the parity hook catches it only in the good direction:

```sh
prev=$(mise exec -- svu current --tag.mode current)          # v0.5.0 — the range's lower bound
tag=$(mise exec -- svu <bump> --tag.mode current)            # v0.6.0 — what git gets
version="${tag#v}"                                           # 0.6.0  — what the manifest gets
printf 'prev=%s tag=%s version=%s\n' "$prev" "$tag" "$version"
```

`--tag.mode current` on both calls, because `svu` defaults to `all` — it reads tags from
every branch, not just the one being released. This repo routinely carries a stack of
unmerged branches, and a tag pushed on any of them would silently become the baseline for
both the version and the notes range, which is exactly the inference Invariant 1 forbids.

**Then write the three values down.** Everything below runs in a *different* shell — each
step is its own process, and § The gate is a hard stop in between — so `$prev`, `$tag` and
`$version` do not survive to § Execute. That is why the `printf` is there and why every
snippet from here on uses literal `<tag>` / `<version>` placeholders: substitute the real
values as you go. A stale `$tag` that expands to nothing is not a loud failure — step 4's
`git commit -m "chore: "` succeeds and pushes a truncated commit to `trunk`.

**Assemble the notes from the commit range.**

```sh
git log --no-merges --pretty='%s (%h)' <prev>..HEAD
```

Write them to `<somewhere>/release-notes-<tag>.md`, and it **must be outside the repo**
(the session scratchpad, or `/tmp`). This is not tidiness: goreleaser refuses to release
from a tree with any uncommitted change, untracked files included, so a notes file left in
the repo root aborts Execute step 7 — after the tag is public.

`--release-notes` replaces goreleaser's own changelog generation entirely, so this file is
the whole body of the GitHub release. Write it for somebody deciding whether to upgrade,
not for somebody reading `git log`:

- Lead with what changed for a user of the popup or the CLI. A commit that renamed an
  internal helper does not earn a line; one that changed what `herdr-scratch link` does
  earns the first one.
- Group related commits into one entry rather than transcribing each. Seven commits are
  often three changes.
- Call out anything requiring action — a re-`link` after upgrading, a config change, a
  removed flag — under its own heading. This is the part people actually need.
- Keep the conventional-commit prefixes out of the prose; they are metadata, not English.

## The gate

Present two things and stop:

1. **The version** — `<prev> → <tag>`, and the fact that `herdr-plugin.toml` will get
   `<version>` without the prefix.
2. **The notes**, in full, as they will appear on the release.

Then wait for an **explicit** go-ahead. Not an inferred one: silence, "sounds good", or a
question about something else is not approval. This is the last moment at which nothing
has happened, which is exactly what makes it worth stopping at.

If the user wants the notes changed, change them and present again. The version is not
negotiable at this gate (Invariant 1) — if it is wrong, the bump word was wrong, so start
over with the right one.

## Execute

In this order. Each step is a precondition of the next; a failure at any step stops the
run and routes to § Recovery. Substitute the real `<tag>`, `<version>` and `<notes>` from
§ Draft — nothing carries over from that shell.

**Step 1 is an edit, not a command.** Set the top-level `version` key in
[herdr-plugin.toml](../../../herdr-plugin.toml) to `<version>` — the bare form, no `v`.
`min_herdr_version` is a different key and stays put. Edit it directly: `taplo` reads this
key (`taplo get -f herdr-plugin.toml version`) but has no `set`.

```sh
# 2. Restore taplo's column alignment on the file just edited — and only that file.
mise exec -- taplo format herdr-plugin.toml

# 3. Prove parity locally, before anything is public.
mise run version-check <version>

# 4. The bump commit goes first, so the tag has a commit whose manifest matches it.
git add herdr-plugin.toml && git commit -m "chore: <version>" && git push

# 5. Confirm nothing else is uncommitted; goreleaser refuses a dirty tree at step 7,
#    which is after the tag. Any output here is a stop.
git status --porcelain

# 6. The tag. Everything before this is local and freely undone; this is not.
git tag -a <tag> -m <tag> && git push origin <tag>

# 7. Build, publish the release, commit the cask.
GITHUB_TOKEN="${GITHUB_TOKEN:-$(gh auth token)}" mise exec -- goreleaser release \
  --clean --release-notes <notes>
```

The bump commit's message is fixed at `chore: <version>` — it records a version and
nothing else, so there is nothing to compose. Write it as given.

Step 2 formats **one file** rather than running `mise run format`, which is
`hk fix --all --no-stage`: every formatter over the whole repo, deliberately staging
nothing. Step 4 stages only `herdr-plugin.toml`, so anything else a repo-wide pass rewrote
would sit unstaged in the tree and abort step 7 — in the one window this skill exists to
protect. Step 5 is the backstop for the same class of mistake from any other source.

Step 3 is not redundant with goreleaser's hook, and it is not a second implementation of
the parity check either — the check itself stays where
[.goreleaser.yaml](../../../.goreleaser.yaml)'s `before:` hook owns it, and this runs that
same `.mise/tasks/version-check`, only earlier. The hook fails the release *after* the tag
is pushed, which is the expensive place to find out; this catches the same mistake while
it still costs an amended commit.

## Recovery

Key the recovery on **the state the repo is actually in**, not on the step number that
reported the failure. The two come apart at step 6, which is two commands: a failed
`git push origin <tag>` leaves a local tag and nothing public, and reaching for the remote
deletion there errors on a ref that never existed. Check with
`git ls-remote --tags origin <tag>` when it is not obvious.

| State | What to do |
| ----- | ---------- |
| Nothing committed (steps 1–3, 5) | Fix the manifest and re-run from step 1. `version-check` names both sides it compared. |
| Bump commit pushed, no tag (step 4) | Ordinary git. Fix forward with another commit, or revert it. Nothing references it yet. |
| Local tag only (step 6's `git tag` ran, its push did not) | `git tag -d <tag>`, fix the cause, retry step 6. Nothing is public. Note the repo's `pre-push` hook runs `go test ./...`, so a red suite is a likely cause. |
| **Tag pushed**, no release (step 7 failed early) | `git push --delete origin <tag>` then `git tag -d <tag>`, in that order. Leave the bump commit — it is correct and the retry needs it. |
| Tag pushed, release and/or cask created (step 7 failed late) | Below. |

A failure partway through `goreleaser release` is the one worth spelling out, because it
can leave three things behind and they must come off in order:

```sh
gh release delete <tag> --yes             # if it got as far as creating one
git push --delete origin <tag>            # then the remote tag
git tag -d <tag>                          # then the local one
```

Check the tap as well —
`gh api repos/macintacos/homebrew-tap/contents/Casks --jq '.[].name'`, listing the
directory rather than probing the file for the same reason § Preconditions does. If
goreleaser committed the cask before failing, it now points at a release that does not
exist and `brew install --cask` will 404 for anyone who tries.

**Retrying the same version needs no tap edit** — the next run rewrites
`Casks/herdr-scratch.rb` from scratch. Reverting that commit by hand is only for a version
being *abandoned* rather than retried, and it is the one case § When NOT to Use's "don't
hand-edit the tap" gives way to.

**Never leave a pushed tag with no release behind it.** It is the one failure mode that
misleads silently: `svu` computes the *next* version from the newest tag, so an abandoned
tag makes every future release skip a version, and the tag itself looks to anyone browsing
the repo like a version that shipped.

## After the release

The checks a cask cannot make for itself are in
[docs/RELEASING.md](../../../docs/RELEASING.md) — `brew install --cask`, `--version`
(which must report the tag, not `dev`), `link` exiting 0, and the upgrade cycle. Point the
user at it rather than restating it here; two copies of a checklist drift, and that one is
what somebody reads without an agent in the room.

## When NOT to Use

- **Building or testing locally.** That is `mise run build`, `mise run test`,
  `mise run preflight`.
- **Rehearsing the release machinery.**
  `goreleaser release --snapshot --clean --skip=publish` builds everything into `dist/`
  and touches nothing remote. No tag, no version, no gate — just run it.
- **Fixing the tap.** Editing `macintacos/homebrew-tap` by hand is what the cask config
  exists to end. Two exceptions, both named above: deleting the leftover formula (§
  Preconditions), and reverting a cask commit for a version being abandoned rather than
  retried (§ Recovery).

## Common Mistakes

- **Putting the `v` in the manifest, or leaving it off the tag.** `herdr-plugin.toml` gets
  `0.6.0`; git gets `v0.6.0`. Derive both from one `svu` call as § Draft shows.
- **Tagging before committing the bump.** The tag would point at a commit whose manifest
  still says the old version, and goreleaser's `before:` hook would fail the release with
  the tag already pushed.
- **Using `svu next`.** It infers the bump from commit messages. The bump word is the
  user's decision (Invariant 1).
- **Running the repo's checks.** Invariant 2. Ask, don't run.
- **Treating the gate as a formality.** It is the only point where the version and the
  notes can still be wrong for free.
- **Releasing while the tap still has the formula.** The most expensive mistake here,
  because everything reports success — the release exists, the cask is committed, and
  `brew install herdr-scratch` still compiles from source.
