# Releasing

By hand, from a laptop — there is no CI. [goreleaser](https://goreleaser.com) builds
darwin and linux binaries for both architectures, publishes the GitHub release with
checksums and notes, and commits the cask to
[macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap). Its config is
[`.goreleaser.yaml`](../.goreleaser.yaml); goreleaser itself is pinned in
[`mise.toml`](../mise.toml).

The steps below are the by-hand path, and the reference for what each one is for. The
[`/release` skill](../.claude/skills/release/SKILL.md) runs the same sequence with the
version computed by [`svu`](https://github.com/caarlos0/svu) rather than typed, and the
notes drafted from the commit range — which is the usual way to cut one.

> [!IMPORTANT]
> **Once, before the first cask release.** goreleaser writes `Casks/herdr-scratch.rb` into
> the tap and leaves `Formula/herdr-scratch.rb` exactly where it is. Delete that formula
> in [macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap) by hand. While
> both files exist Homebrew resolves the bare token to the **formula** — it prints
> "Treating macintacos/tap/herdr-scratch as a formula" and builds from source — so until
> it is gone, a release can look shipped while nobody is actually getting the binary.

```sh
$EDITOR herdr-plugin.toml                      # bump `version` to the tag you are cutting
git add herdr-plugin.toml && git commit -m "chore: 0.6.0" && git push
git status --porcelain                         # must be empty, untracked files included
git tag -a v0.6.0 -m v0.6.0 && git push origin v0.6.0
GITHUB_TOKEN=$(gh auth token) mise exec -- goreleaser release \
  --clean --release-notes /tmp/release-notes-v0.6.0.md
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
thing that looks. `mise run version-check 0.6.0` asks the same question without releasing
anything.

**The token** needs contents write on *both* repositories — `macintacos/herdr-scratch` to
create the release, `macintacos/homebrew-tap` to commit the cask. A classic PAT with
`repo` scope covers it, as does a fine-grained token scoped to the two. goreleaser reads
`GITHUB_TOKEN`, so it lives in the environment for exactly one command and is never
committed; `$(gh auth token)` is enough when `gh` is already authenticated.

**Rehearse first.** `goreleaser release --snapshot --clean --skip=publish` builds all four
targets into `dist/` and renders the cask to `dist/homebrew/Casks/herdr-scratch.rb`
without touching GitHub, and `goreleaser check` validates the config on its own.

**Then check by hand what a cask cannot check for itself.** A formula has `test do`; a
cask has no equivalent, so these are yours:

```sh
brew install --cask macintacos/tap/herdr-scratch
herdr-scratch --version   # the tag you just cut — "dev" means the ldflags stamp broke
herdr-scratch link        # exits 0; `stat .../bin` means the archive layout regressed
```

And once there is a previous release to come from, the upgrade rather than the install —
`brew upgrade --cask herdr-scratch` followed by `herdr-scratch link`, then press the
chord. The popup surviving that is the thing a versioned Caskroom path would break, so it
is worth one deliberate check per release rather than an assumption.

Worth one look on a fresh machine as well: Homebrew quarantines what a cask downloads, and
the cask's `postflight` strips the attribute back off. If macOS refuses to run
`herdr-scratch` anyway, `xattr -p com.apple.quarantine "$(which herdr-scratch)"` says
whether the hook ran.

`v0.5.0` is tagged but deliberately has no GitHub release: it predates this config, so
goreleaser cannot build from it. The first published release is the next tag after it.
