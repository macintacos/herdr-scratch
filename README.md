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

Needs herdr 0.8.0 or newer. Linux and macOS take the same three commands.

```sh
brew install macintacos/tap/herdr-scratch
herdr plugin link "$(brew --prefix)/share/herdr-scratch"
herdr server reload-config
```

A prebuilt binary, so nothing compiles and Go is not needed — the formula brings herdr and
tmux with it, and that is the whole dependency list.

`herdr plugin link` is one-time. That path is refreshed on every upgrade, so from here on
`brew upgrade` is the whole procedure.

> [!IMPORTANT]
> **Coming from the cask.** `brew upgrade` will not carry you across, and installing the
> formula while the cask is still there leaves you on the cask: Homebrew unpacks the keg
> but prints `herdr-scratch cask is installed, skipping link`, so the `herdr-scratch` on
> your PATH does not change. Uninstall the cask first.
>
> ```sh
> brew uninstall --cask herdr-scratch
> brew install macintacos/tap/herdr-scratch
> herdr plugin link "$(brew --prefix)/share/herdr-scratch"
> rm -rf ~/.local/share/herdr-scratch
> ```
>
> The `link` line is not optional. The cask registered `~/.local/share/herdr-scratch`, and
> nothing refreshes that any more — `herdr-scratch link`, which used to write it, is gone
> as of 0.7.0. Until you repoint herdr, the popup keeps running the build the cask left
> there.

> [!NOTE]
> Upgrading pulls herdr up with it, and a herdr server already running will not match the
> CLI it was just upgraded past. Restart herdr afterwards, or use
> `brew reinstall herdr-scratch` to leave your dependencies alone.

<details>
<summary>Installing without Homebrew</summary>

Two of them, cheapest first. Both still want herdr 0.8.0+ (earlier versions have no
`plugin pane` command) and tmux.

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
> every upgrade installs that release's copy over the top of yours.

## Uninstall

```sh
tmux -L herdr-scratch kill-server                    # stop every scratch shell
herdr plugin unlink user.scratch                     # or: herdr plugin uninstall user.scratch
brew uninstall herdr-scratch                         # if you installed it that way
rm -rf "$(brew --prefix)/share/herdr-scratch"        # the plugin root brew leaves behind
rm -rf ~/.config/herdr/plugins/config/user.scratch   # your settings
```

The third and fourth lines are two steps rather than one because a formula has no `zap`:
`brew uninstall` owns the keg and nothing else.

Then delete the `[[keys.command]]` block from herdr's own `~/.config/herdr/config.toml`
and `herdr server reload-config`.

## Troubleshooting

> [!TIP]
> Worth checking first when what you see does not match what is documented here: which
> build is herdr actually loading?
>
> ```sh
> herdr plugin list | grep user.scratch    # the root herdr recorded
> "$(brew --prefix)/share/herdr-scratch/bin/herdr-scratch" --version
> ```
>
> Ask the copy under the root herdr names, not the `herdr-scratch` on your PATH. On a
> Homebrew install the two are refreshed together and always agree; a root herdr recorded
> somewhere *else* is refreshed by nothing, and the gap is exactly what that looks like.

**The chord opens the popup but will not close it.** Almost always
[`dismiss`](#configuring) not matching the chord you press. If you upgraded recently and
herdr is registered against a root the upgrade did not refresh, it is still reading the
old manifest — see the two entries below.

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
`Caskroom/`. herdr was pointed at a keg directly, which pins it to a directory the next
upgrade deletes. Point it at the root that survives instead:

```sh
herdr plugin link "$(brew --prefix)/share/herdr-scratch"
```

**The popup runs a release you already upgraded past.** herdr is registered against a root
that upgrading does not refresh. Most often `~/.local/share/herdr-scratch` — what the cask
era's `herdr-scratch link` wrote, and what nothing maintains now that the command is gone.
The same `herdr plugin link` above repoints it; delete the stale directory afterwards.

**Nothing at all happens when you press it.** Read the log — keypresses have nowhere else
to report:

```sh
tail -f ~/.local/state/herdr/plugins/user.scratch/herdr-scratch.log
```

A binary run by hand rather than by herdr writes to
`~/.local/state/herdr-scratch/herdr-scratch.log` instead. For much more detail, put
`--debug` in the action's command in `herdr-plugin.toml`:

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

> [!WARNING]
> herdr registers one copy of a plugin, so linking a checkout replaces whatever was
> registered before — a Homebrew install stops being the one herdr loads until you run
> `herdr plugin link "$(brew --prefix)/share/herdr-scratch"` again.

### Releasing

Cutting a release is [`docs/RELEASING.md`](docs/RELEASING.md): goreleaser builds the
binaries, publishes the GitHub release with checksums and notes, and commits the Homebrew
formula to [macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap).

## License

MIT
