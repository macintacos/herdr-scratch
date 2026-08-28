# Releasing

By hand, from a laptop — there is no CI. [goreleaser](https://goreleaser.com) builds
darwin and linux binaries for both architectures, publishes the GitHub release with
checksums and notes, and commits the formula to
[macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap). Its config is
[`.goreleaser.yaml`](../.goreleaser.yaml); goreleaser itself is pinned in
[`mise.toml`](../mise.toml).

The steps below are the by-hand path, and the reference for what each one is for. The
[`/release` skill](../.claude/skills/release/SKILL.md) runs the same sequence with the
version computed by [`svu`](https://github.com/caarlos0/svu) rather than typed, and the
notes drafted from the commit range — which is the usual way to cut one.

> [!IMPORTANT]
> **Once, at the first formula release.** goreleaser writes `Formula/herdr-scratch.rb`
> into the tap and leaves `Casks/herdr-scratch.rb` where it is. Delete the cask from
> [macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap) by hand. It blocks
> nothing — with both files present Homebrew resolves the bare token to the formula anyway
> — so do it whenever, before or after.

The version below is illustrative — `svu` computes the real one, and the `/release` skill
is what runs it.

```sh
$EDITOR herdr-plugin.toml                      # bump `version` to the tag you are cutting
git add herdr-plugin.toml && git commit -m "chore: 0.7.0" && git push
git status --porcelain                         # must be empty, untracked files included
git tag -a v0.7.0 -m v0.7.0 && git push origin v0.7.0
GITHUB_TOKEN=$(mise exec -- gh auth token) mise exec -- goreleaser release \
  --clean --release-notes /tmp/release-notes-v0.7.0.md
```

Stage the manifest by name rather than reaching for `commit -am`, and check the tree
before tagging: goreleaser refuses to run on a tree with **any** uncommitted change,
untracked files included, and it refuses at the last step — after the tag is public. The
notes file is written to `/tmp` for that same reason: drafted in the repo root it would be
one more untracked file, and it would fail the release at exactly that point. Dropping
`--release-notes` is fine too; goreleaser then generates the notes from the commit log
itself.

**Through `mise exec`** because the `before:` hook execs `taplo` and inherits only
goreleaser's own `PATH` — activated mise has it, `mise exec` has it either way.

The bump comes first because a `before:` hook compares `herdr-plugin.toml` against the tag
and fails the release when they disagree. herdr reads the manifest's version rather than
the tag, so a release whose manifest says something else is misreporting itself to the one
thing that looks. `mise run version-check 0.7.0` asks the same question without releasing
anything.

**The token** needs contents write on *both* repositories — `macintacos/herdr-scratch` to
create the release, `macintacos/homebrew-tap` to commit the formula. A classic PAT with
`repo` scope covers it, as does a fine-grained token scoped to the two. goreleaser reads
`GITHUB_TOKEN`, so it lives in the environment for exactly one command and is never
committed; `$(mise exec -- gh auth token)` is enough when `gh` is already authenticated.

**Rehearse first.** `goreleaser release --snapshot --clean --skip=publish` builds all four
targets into `dist/` and renders the formula to `dist/homebrew/Formula/herdr-scratch.rb`
without touching GitHub, and `mise run goreleaser-check` validates the config on its own.
Use the task rather than bare `goreleaser check`: the config needs `brews`, which
goreleaser deprecated in 2.16, and `check` exits non-zero on any deprecated property with
no flag to tolerate one. The task forgives that one deprecation and nothing else.

**Then lint the rendered formula, which costs nothing and catches most of it:**

```sh
brew tap-new local/rehearsal --no-git
cp dist/homebrew/Formula/herdr-scratch.rb "$(brew --repository)/Library/Taps/local/homebrew-rehearsal/Formula/"
brew style local/rehearsal/herdr-scratch
brew audit --strict local/rehearsal/herdr-scratch
brew untap local/rehearsal    # not optional; see below
```

`brew audit --strict` is the one with opinions worth having: it rejects `rm_rf` in
`post_install`, refuses a formula with no `test do`, and rejects a `livecheck` block
placed after `depends_on` — which is where goreleaser's `custom_block` would put one, and
why there is no `livecheck do skip` in the config.

The untap is not tidiness either. A rehearsal tap left behind holds a second formula named
`herdr-scratch`, and the next rehearsal's `brew style` then reports
`Lint/DuplicateMethods` against the stale copy — spurious offences on the very check whose
value is that it has opinions worth having.

**Then check by hand what only a real install shows.** `test do` covers the version stamp,
but `brew test` runs it only when asked:

```sh
brew install macintacos/tap/herdr-scratch
brew test macintacos/tap/herdr-scratch     # the `test do` version assertion
herdr-scratch --version                    # the tag you just cut — "dev" means the ldflags stamp broke
ls "$(brew --prefix)/share/herdr-scratch"  # bin, herdr-plugin.toml, shell, tmux.conf
"$(brew --prefix)/share/herdr-scratch/bin/herdr-scratch" --version
```

The two versions agreeing is the check, and the `ls` is the other half: `post_install`
replaces that root outright, so a missing entry means the `install` and `post_install`
stanzas have drifted from what the archive ships. All four are needed — the popup starts
tmux with `tmux.conf` and fish sources `shell/herdr-scratch.fish`.

And once there is a previous release to come from, the upgrade rather than the install —
`brew upgrade herdr-scratch`, the same checks, then press the chord. `post_install`
re-runs on upgrade, which is the whole reason nothing has to be re-linked; the popup
surviving is what proves the root herdr recorded was not renumbered. Worth one deliberate
check per release rather than an assumption.

A formula download is **not** quarantined — Homebrew quarantines cask downloads only — so
there is nothing to strip and nothing to check on a fresh machine.

## The Linux check

One-time rather than per-release. The whole point of a formula over a cask is that Linux
gets a brew path at all, and nothing on a macOS laptop exercises it — so it is worth
installing on Linux once, from a container.

Rewriting `url` / `sha256` to the local archive is the load-bearing step: without it
`brew install` fetches the rendered GitHub URLs, which for a snapshot point at a release
that does not exist, and the 404 does not look like a skipped step. So it lives in a
script rather than a comment inside a `bash -c`, where nothing would perform it. Write
`dist/linux-check.sh`:

```sh
#!/usr/bin/env bash
set -euo pipefail

case "$(uname -m)" in
aarch64 | arm64) arch=arm64 ;;
*) arch=amd64 ;;
esac
archive=$(ls /dist/herdr-scratch_*_linux_"$arch".tar.gz)
sha=$(sha256sum "$archive" | cut -d' ' -f1)

brew tap-new local/rehearsal --no-git
tapdir="$(brew --repository)/Library/Taps/local/homebrew-rehearsal/Formula"
cp /dist/homebrew/Formula/herdr-scratch.rb "$tapdir/"
sed -i "s|url \".*\"|url \"file://$archive\"|g; s|sha256 \".*\"|sha256 \"$sha\"|g" \
	"$tapdir/herdr-scratch.rb"

brew install local/rehearsal/herdr-scratch
brew info herdr | head -3                             # must say (bottled)
ls -la "$(brew --prefix)/share/herdr-scratch"         # all four entries
"$(brew --prefix)/share/herdr-scratch/bin/herdr-scratch" --version
brew test local/rehearsal/herdr-scratch
```

Then run it in the container, and delete it afterwards — `dist/` is gitignored, but the
next `--clean` clears it regardless:

```sh
container run --rm --volume "$PWD/dist:/dist" homebrew/brew bash /dist/linux-check.sh
rm dist/linux-check.sh
```

The arch detection is not padding: apple/container runs an **arm64** Linux VM on Apple
silicon, so the amd64 tarball fails at `Exec format error` rather than anywhere
informative. The two things this is here to confirm are in the script — that `herdr`
resolves to a **bottle** rather than a source build, and that `post_install` populated the
plugin root.

`v0.5.0` is tagged but deliberately has no GitHub release: it predates this config, so
goreleaser cannot build from it. The first published release is the next tag after it.
