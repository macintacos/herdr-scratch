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

[README.md](../../../README.md)'s **Releasing** section is this same procedure written for
a person, and it is the canonical explanation of *why* each step is ordered as it is. This
file is that runbook executed. When one changes, change both.

## Arguments

Exactly one bump word:

| Argument | From `v0.5.0` | Use when |
| -------- | ------------- | -------- |
| `patch`  | `v0.5.1`      | Fixes only; nothing a user has to do anything about. |
| `minor`  | `v0.6.0`      | New behaviour, or a change to what `link` / the popup does. |
| `major`  | `v1.0.0`      | A break — the manifest, the plugin root layout, or a removed command. |

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
git branch --show-current           # trunk
git status --porcelain              # empty
git fetch origin && git rev-parse HEAD origin/trunk   # two identical SHAs
gh api repos/macintacos/homebrew-tap/contents/Formula/herdr-scratch.rb   # must 404
```

- **goreleaser or svu missing** → the tool is pinned in [mise.toml](../../../mise.toml),
  so this means mise has not installed it. Say `mise install` and stop; do not fall back
  to a system copy, which is not the pinned version.
- **Not on `trunk`, dirty, or behind `origin/trunk`** → stop. A release is cut from the
  default branch's tip; a tag on anything else points at a tree nobody reviewed.
- **The token.** goreleaser reads `GITHUB_TOKEN` and it needs contents write on **both**
  `macintacos/herdr-scratch` (to create the release) and `macintacos/homebrew-tap` (to
  commit the cask). `$(gh auth token)` covers it when `gh` is authenticated. If neither
  `$GITHUB_TOKEN` nor `gh auth token` yields one, stop and say exactly that — do not start
  a sequence that will fail at its last and least recoverable step.
- **`Formula/herdr-scratch.rb` still exists in the tap** → **stop.** goreleaser writes
  `Casks/herdr-scratch.rb` and leaves the formula alone, and while both exist Homebrew
  resolves the bare `herdr-scratch` token to the **formula** and keeps building from
  source. The release would publish, the cask would be committed, and nobody would receive
  either. Deleting it is a manual step in the tap repo; ask the user to do it and stop. A
  `404` from that `gh api` call is the pass.

Then **ask the user to confirm the repo's checks were run** — `mise run preflight` covers
lint and tests in one. Their word is the gate (Invariant 2). If they have not, stop and
let them; there is no reason to race a release ahead of its own test suite.

## Draft

Nothing here writes to the repo.

**Compute both forms of the version from one call.** `svu` prints a `v` prefix; the git
tag keeps it and `herdr-plugin.toml` does not. Getting this backwards is the single most
likely mistake in this skill, and the parity hook catches it only in the good direction:

```sh
prev=$(mise exec -- svu current)      # v0.5.0 — the range's lower bound
tag=$(mise exec -- svu "$bump")       # v0.6.0 — what git gets
version="${tag#v}"                    # 0.6.0  — what herdr-plugin.toml gets
```

**Assemble the notes from the commit range.**

```sh
git log --no-merges --pretty='%s (%h)' "$prev..HEAD"
```

Write them to `notes=<somewhere>/release-notes-$tag.md`, **outside the repo** (the session
scratchpad, or `/tmp`) — the notes are an argument to goreleaser, not a file this repo
tracks. § Execute passes `$notes` straight through.

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
run and routes to § Recovery.

```sh
# 1. Bump the manifest. One `version` key (line 3); `min_herdr_version` is a
#    different key and stays put. Edit that line — taplo has `get` but no `set`.
# 2. Restore taplo's column alignment.
mise run format

# 3. Prove parity locally, before anything is public. goreleaser's `before:` hook
#    runs this same check, but by then the tag exists.
mise run version-check "$version"

# 4. The bump commit goes first, so the tag has a commit whose manifest matches it.
git add herdr-plugin.toml && git commit -m "chore: $version" && git push

# 5. The tag. Everything before this is local and freely undone; this is not.
git tag -a "$tag" -m "$tag" && git push origin "$tag"

# 6. Build, publish the release, commit the cask.
GITHUB_TOKEN="${GITHUB_TOKEN:-$(gh auth token)}" mise exec -- goreleaser release \
  --clean --release-notes "$notes"
```

The bump commit's message is fixed at `chore: <version>` — it records a version and
nothing else, so there is nothing to compose. Write it as given.

Step 3 is not redundant with goreleaser's hook. The hook fails the release *after* the tag
is pushed, which is the expensive place to find out; this catches the same mistake while
it still costs an amended commit.

## Recovery

Which recovery applies depends only on how far § Execute got.

| Failed at | What is public | What to do |
| --------- | -------------- | ---------- |
| 1–3 (bump, format, parity) | Nothing | Fix the manifest and re-run from step 1. `version-check` names both sides it compared. |
| 4 (bump commit) | The commit | Ordinary git. Fix forward with another commit, or revert it. No tag exists yet, so nothing references it. |
| 5 (tag push) | **The tag** | `git push --delete origin "$tag"` then `git tag -d "$tag"`. Leave the bump commit — it is correct and the retry needs it. |
| 6 (goreleaser) | The tag, maybe a partial release, maybe the cask | Below. |

A failure inside `goreleaser release` is the one worth spelling out, because it can leave
three things behind and they must come off in order:

```sh
gh release delete "$tag" --yes            # if it got as far as creating one
git push --delete origin "$tag"           # then the remote tag
git tag -d "$tag"                         # then the local one
```

Check the tap as well —
`gh api repos/macintacos/homebrew-tap/contents/Casks/herdr-scratch.rb`. If goreleaser
committed the cask before failing, it now points at a release that does not exist and
`brew install --cask` will 404 for anyone who tries. Revert that commit in the tap before
retrying.

**Never leave a pushed tag with no release behind it.** It is the one failure mode that
misleads silently: `svu` computes the *next* version from the newest tag, so an abandoned
tag makes every future release skip a version, and the tag itself looks to anyone browsing
the repo like a version that shipped.

## After the release

The checks a cask cannot make for itself are in [README.md](../../../README.md)'s
**Releasing** section — `brew install --cask`, `--version` (which must report the tag, not
`dev`), `link` exiting 0, and the upgrade cycle. Point the user at it rather than
restating it here; two copies of a checklist drift, and the README's is the one somebody
reads without an agent in the room.

## When NOT to Use

- **Building or testing locally.** That is `mise run build`, `mise run test`,
  `mise run preflight`.
- **Rehearsing the release machinery.**
  `goreleaser release --snapshot --clean --skip=publish` builds everything into `dist/`
  and touches nothing remote. No tag, no version, no gate — just run it.
- **Fixing the tap.** Editing `macintacos/homebrew-tap` by hand is what the cask config
  exists to end. The one exception is deleting the leftover formula, which is a
  precondition above.

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
