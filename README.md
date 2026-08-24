# 🐏 `herdr-scratch` 📜

A scratch shell for [herdr](https://herdr.dev), in a popup you toggle with one chord.

![A herdr session with the scratch popup open over it — a bordered window titled "scratch", running a shell in the pane's own directory.](docs/assets/scratch-popup.png)

Press <kbd>prefix</kbd> + <kbd>'</kbd> and a shell opens over whatever you were doing,
starting in that pane's directory. Press it again and the popup goes away — but the shell
does not. Start a dev server, put the popup away, bring it back an hour later and it is
still running, with the scrollback exactly where you left it.

One shell per space (herdr's word for a workspace), shared by every pane in it. macOS and
Linux — not Windows, since it depends on tmux.

## Install

```sh
brew install macintacos/tap/herdr-scratch
```

A prebuilt binary, so nothing compiles and Go is not needed — the cask brings herdr and
tmux with it, and that is the whole dependency list. **macOS only**: Homebrew has no cask
support on Linux, so a Linux install goes through one of the non-Homebrew paths below —
including a prebuilt tarball, so Linux does not have to compile either.

> [!IMPORTANT]
> If you installed this before it was a cask, uninstall the formula first —
> `brew uninstall herdr-scratch`, then `brew install --cask macintacos/tap/herdr-scratch`.
> `brew upgrade` will not carry you across on its own, and while the tap still carries the
> old formula Homebrew resolves the bare name to it rather than to the cask, which is why
> `--cask` is spelled out here. The popup should keep working throughout: since 0.5.0 it
> runs out of `~/.local/share/herdr-scratch`, which holds copies rather than links into
> the Homebrew prefix, and the cask re-points it at its own build on install.

The cask registers the build with herdr from its post-install hook, so an upgrade needs
nothing from you: `brew upgrade --cask herdr-scratch` copies the new release into the
directory herdr records — a directory no upgrade can delete, which is the point.
`herdr-scratch link` does that same registration by hand, for a hook that failed or an
install that did not come from the cask.

One exception, if you set them: Homebrew strips `XDG_DATA_HOME`, `XDG_CONFIG_HOME` and
`XDG_STATE_HOME` out of the environment it runs hooks in, so the hook uses the `$HOME`
defaults rather than your paths. Running `herdr-scratch link` yourself puts all three back
where you asked for them, and the next upgrade moves them again.

> [!IMPORTANT]
> Re-run `herdr-scratch link` after upgrading to 0.5.0. Settings moved out of the plugin's
> manifest into a config file, and until you re-link, the old manifest keeps overriding it
> — an edit to `dismiss` will look like it did nothing. `link` prints anything worth
> keeping before it replaces the file.

> [!NOTE]
> Upgrading pulls herdr up with it, and a herdr server already running will not match the
> CLI it was just upgraded past. Restart herdr afterwards, or use
> `brew reinstall --cask herdr-scratch` to leave your dependencies alone.

<details>
<summary>Installing without Homebrew — and the way in on Linux</summary>

The non-Homebrew paths, and on Linux the only ones. Three of them, cheapest first.

**Take the prebuilt tarball.** Every
[release](https://github.com/macintacos/herdr-scratch/releases) ships `linux_amd64` and
`linux_arm64` archives that are already a plugin root — `bin/herdr-scratch` beside the
manifest, `tmux.conf` and `shell/` — so nothing is compiled and Go is not needed. You
still supply herdr 0.8.0+ (earlier versions have no `plugin pane` command) and tmux.

```sh
mkdir -p ~/.local/opt/herdr-scratch
tar -xzf herdr-scratch_0.6.0_linux_arm64.tar.gz -C ~/.local/opt/herdr-scratch
herdr plugin link ~/.local/opt/herdr-scratch
```

**Build from source.** Add Go 1.25+ to that dependency list; herdr compiles the binary for
you as part of installing.

```sh
herdr plugin install macintacos/herdr-scratch
```

**Manage the checkout yourself.** Note that `herdr plugin link` deliberately does **not**
build — do that once by hand:

```sh
git clone https://github.com/macintacos/herdr-scratch ~/.config/herdr/scratch
cd ~/.config/herdr/scratch && go build -o bin/herdr-scratch .
herdr plugin link ~/.config/herdr/scratch
```

</details>

### Bind a key

Installing does not bind anything. In `~/.config/herdr/config.toml`, alongside any
bindings you already have:

```toml
[[keys.command]]
key         = "prefix+'"
type        = "plugin_action"
command     = "user.scratch.toggle"
description = "scratch shell"
```

Then `herdr server reload-config` (start herdr first if it is not running). `prefix` means
whatever your herdr prefix already is — <kbd>Ctrl</kbd> + <kbd>B</kbd> unless you changed
it.

> [!IMPORTANT]
> Use any key you like in place of <kbd>'</kbd> — but set [`dismiss`](#configuring) to
> match, or the popup will not answer the chord you actually press.

**That is the whole setup.** Your shell config is untouched: the popup starts your login
shell, and the chord that closes it belongs to tmux rather than the shell.

## Using it

A popup takes every keypress until it exits, so your herdr prefix cannot reach herdr while
one is up. Two keys close it instead:

| key | when | what it does |
| --- | --- | --- |
| <kbd>prefix</kbd> + <kbd>'</kbd> | any shell, even mid-command | closes it immediately |
| <kbd>Q</kbd> | at a fish prompt, in vi normal mode | the same, written into the scrollback so it says what happened |

The chord is a tmux binding, so it works in bash, zsh, nu, or anything else — including
while a command is running, which no shell binding can cover. <kbd>Q</kbd> and the
notifications below are fish-only; both load through `fish --init-command`, which is why
fish costs you no setup.

`tmux -L herdr-scratch ls` lists your scratch shells, one per space. A shell ends when its
space closes, or when you exit it — the next press then starts a fresh one.

The mouse wheel scrolls the popup's own scrollback. copy-mode is the only viewport tmux
has, so the first notch up enters it and scrolling back to the bottom leaves it again.
Typing while scrolled up goes to copy-mode rather than the shell — <kbd>Enter</kbd> drops
you back at the prompt.

### Check it works

1. <kbd>prefix</kbd> + <kbd>'</kbd> → a bordered popup titled **scratch** opens, in the
   pane's directory.
2. Run something long: `while true; do echo tick; sleep 1; done`
3. Press the chord twice → the popup closes, then comes back with the ticks still counting
   and the scrollback intact.
4. Move to another pane in the same space and press it → the same shell. A pane in a
   *different* space gets one of its own.

If step 3 gives you a blank screen or a fresh prompt, see
[Troubleshooting](#troubleshooting).

## Notifications

When a command finishes in a popup you are **not** looking at, you get a notification. No
extra install, and nothing to turn on. herdr posts it, so it wears your terminal's icon
and name, a click focuses that terminal, and delivery follows whatever you already told
herdr to do with notifications.

Commands shorter than 10 seconds are ignored — change that with
[`notify_after`](#configuring). Most terminals suppress notifications while they are
focused, so expect these when you are in another app, which is when they are worth having.

A `set -g herdr_scratch_notify_after 30000` in your fish config still works, and still
wins over the config file.

## Configuring

Everything you can set lives in one file:

```sh
~/.config/herdr/plugins/config/user.scratch/config.toml
```

It does not exist until you create it, and `herdr plugin config-dir user.scratch` prints
the directory it goes in — worth running first if your settings are not being picked up.

| key | what it sets | format | default |
| --- | --- | --- | --- |
| `dismiss` | the chord that closes the popup | two tmux keys, separated by a space | `"C-b '"` |
| `notify_after` | how long a command must run before it notifies | milliseconds | `10000` |
| `width` | how wide the popup opens | terminal cells as a number, or a percentage string | `"70%"` |
| `height` | how tall it opens | the same | `"70%"` |

Set only what you are changing — a key you leave out keeps its default:

```toml
dismiss = "C-a ;"
width   = "80%"
```

A file this cannot read is ignored in favour of the defaults rather than breaking the
popup, and so is a key it does not recognise — a `dismis` typo is reported by name in the
[log](#troubleshooting) instead of quietly doing nothing.

**`dismiss` is the one worth reading twice.** It has to name the same chord that *opens*
the popup. If your herdr prefix is <kbd>Ctrl</kbd> + <kbd>A</kbd>, or you bound something
other than <kbd>'</kbd>, change it to match — `"C-a ;"` for <kbd>Ctrl</kbd> + <kbd>A</kbd>
then <kbd>;</kbd>. Nothing can work it out for you: herdr has no way to report its prefix.

A new chord works on the next open; the old one keeps working until the scratch tmux
server exits, so run `tmux -L herdr-scratch kill-server` if that bothers you. A
`notify_after` change reaches the next shell you start rather than the next popup you
open.

> [!NOTE]
> `herdr-plugin.toml` is **not** where settings go. That is the plugin's own manifest, and
> `herdr-scratch link` installs each release's copy over the top of yours. If you had
> edited one, `link` prints the lines to paste here.

## Uninstall

```sh
tmux -L herdr-scratch kill-server                    # stop every scratch shell
herdr plugin unlink user.scratch                     # or: herdr plugin uninstall user.scratch
rm -rf ~/.local/share/herdr-scratch                  # what `herdr-scratch link` created
rm -rf ~/.config/herdr/plugins/config/user.scratch   # your settings
brew uninstall --cask herdr-scratch                  # if you installed it that way
```

Then delete the `[[keys.command]]` block from herdr's own `~/.config/herdr/config.toml`
and `herdr server reload-config`.

## Troubleshooting

> [!TIP]
> Worth checking first when what you see does not match what is documented here: which
> build is herdr actually loading?
>
> ```sh
> "${XDG_DATA_HOME:-$HOME/.local/share}"/herdr-scratch/bin/herdr-scratch --version
> ```
>
> Ask that copy, not the `herdr-scratch` on your PATH. The one on PATH is whatever
> Homebrew installed most recently; the one above is what `link` last put where herdr
> reads it, and the gap between them is exactly what a missed re-link looks like.

**The chord opens the popup but will not close it.** Almost always
[`dismiss`](#configuring) not matching the chord you press. If you upgraded recently, the
cask's hook may not have installed the new manifest — run `herdr-scratch link` to install
it by hand.

**The popup opens and closes immediately.** tmux failed to start, or the binary is
missing. If you linked a local checkout, run `go build -o bin/herdr-scratch .` in it —
`herdr plugin link` does not build.

**Reopening gives a blank screen or a fresh prompt.** Whatever is running the session is
not tmux. Check the `command` in `herdr-plugin.toml`.

**<kbd>Q</kbd> does nothing in normal mode.** Almost always a `bind -M normal` in your own
config, which never fires — `default` is what fish calls vi's normal mode. Use
`-M default`.

**No notification when a command finishes out of sight.** Most likely another
`fish_postexec` handler in your config runs first and blocks. `done` is the common one: it
shells out to `terminal-notifier`, which hangs inside a popup and starves this one.
`functions -q __done_ended` inside the popup tells you whether it is loaded. To check
delivery on its own, run `herdr notification show test --body test`.

**`herdr plugin list` shows a path with a version number in it** — under `Cellar/` or
`Caskroom/`. herdr was pointed at the Homebrew prefix directly, which pins it to a
directory the next upgrade deletes. Run `herdr-scratch link` to move the registration
somewhere that survives.

**The popup runs a release you already upgraded past.** The cask's post-install hook
registers each build, so seeing this means the hook did not run, the install did not come
from the cask, or you set `XDG_DATA_HOME` — which Homebrew does not pass to the hook, so
it registered `~/.local/share/herdr-scratch` instead of yours. Run `herdr-scratch link` to
register the build on your PATH. The version check in the tip above is the one that shows
this — `herdr-scratch --version` on its own asks the copy on your PATH, which is already
the new build and so never disagrees.

**Nothing at all happens when you press it.** Read the log — keypresses have nowhere else
to report:

```sh
tail -f ~/.local/state/herdr/plugins/user.scratch/herdr-scratch.log
```

`herdr-scratch link` writes to `~/.local/state/herdr-scratch/herdr-scratch.log` instead,
which is the one to read when settings did not survive an upgrade. For much more detail,
put `--debug` in the action's command in `herdr-plugin.toml`:

```toml
command = ["./bin/herdr-scratch", "--debug", "toggle"]
```

## Development

```sh
go build -o bin/herdr-scratch .   # what `herdr plugin install` runs for you
go test ./...                     # the logic under test
herdr plugin link "$PWD"          # point herdr at this checkout
```

`herdr plugin link` points herdr at the checkout itself, so a rebuild is live immediately.
`herdr-scratch link` is the packaged path and copies instead, which would leave you
re-running it after every build — use the former while developing.

> [!WARNING]
> herdr registers one copy of a plugin, so linking a checkout replaces whatever was
> registered before — a Homebrew install stops being the one herdr loads until you run
> `herdr-scratch link` again.

### Releasing

Cutting a release is [`docs/RELEASING.md`](docs/RELEASING.md): goreleaser builds the
binaries, publishes the GitHub release with checksums and notes, and commits the Homebrew
cask to [macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap).

## License

MIT
