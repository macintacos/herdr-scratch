# 🐏 `herdr-scratch` 📜

A scratch shell for [herdr](https://herdr.dev), in a popup you toggle with one chord.

![A herdr session with the scratch popup open over it — a bordered window titled "scratch", running a shell in the pane's own directory.](docs/scratch-popup.png)

Press <kbd>prefix</kbd> + <kbd>'</kbd> and a shell opens over whatever you were doing,
starting in that pane's directory. Every other pane in the same space — herdr's word for a
workspace, the tabs and panes grouped under one — reaches that same shell from then on, so
it is there wherever you move to. Press it again and the popup goes away, but the shell
does not: start a dev server, put the popup away, bring it back an hour later and it is
still running, with the scrollback exactly where you left it.

macOS and Linux. Not Windows — it depends on tmux.

## Install

```sh
brew install macintacos/tap/herdr-scratch
herdr-scratch link
```

The formula brings herdr and tmux with it, and Go to build with, so there is nothing to
install first.

`link` is the only *registration* you ever do — `brew upgrade herdr-scratch` picks up the
new build without it.

> [!IMPORTANT]
> Run `herdr-scratch link` again after upgrading to 0.5.0. herdr keeps the manifest in the
> directory it recorded, and only `link` replaces it — so until you do, an older manifest
> is still passing `--dismiss`, which overrides the `dismiss` you set in `config.toml` and
> makes an edit there look like it did nothing. `link` prints anything it finds worth
> keeping before it replaces the file.

> [!NOTE]
> Upgrading pulls herdr up with it, since the formula depends on herdr, and a herdr server
> already running will not match the CLI it was just upgraded past. Restart herdr
> afterwards, or run `brew reinstall herdr-scratch` to leave your dependencies alone.

<details>
<summary>Why registering is a command at all, rather than something the formula does</summary>

herdr has no plugin search path: it learns about a plugin from `install` or `link` and
nothing else. A formula cannot make that call, because Homebrew runs install scripts with
`HOME` pointed at a temporary directory — herdr would write the registration somewhere that
is deleted moments later.

Nor can herdr simply be handed the Homebrew prefix. It resolves a plugin's manifest and
records the real directory holding it, which is `Cellar/herdr-scratch/<version>` — deleted
by the very next upgrade. That is the version-pinning `link` exists to avoid: it builds a
directory of its own, holding the manifest as a real file and symlinking the rest at
`opt/herdr-scratch`, the path Homebrew keeps pointed at whatever is current. herdr resolves
to somewhere that never moves, and what is under it always reaches the build you have now.

</details>

### Without Homebrew

You supply the dependencies yourself: herdr 0.8.0+ (`herdr --version` — earlier versions
have no `plugin pane` command), tmux, and Go 1.25+ to build with.

```sh
herdr plugin install macintacos/herdr-scratch
```

That compiles the binary for you. herdr runs build commands during a GitHub install — after
confirmation, before it registers the plugin — so a failed build leaves nothing
half-installed.

Prefer to manage the checkout yourself? `herdr plugin link` deliberately leaves a local
directory alone, which means it does **not** build. Do that once yourself:

```sh
git clone https://github.com/macintacos/herdr-scratch ~/.config/herdr/scratch
cd ~/.config/herdr/scratch && go build -o bin/herdr-scratch .
herdr plugin link ~/.config/herdr/scratch
```

Installing does not bind anything, however you did it. One step remains.

### Bind a key

In `~/.config/herdr/config.toml`, alongside any bindings you already have:

```toml
[[keys.command]]
key         = "prefix+'"
type        = "plugin_action"
command     = "user.scratch.toggle"
description = "scratch shell"
```

Then `herdr server reload-config` (start herdr first if it is not running).

`prefix` means whatever your herdr prefix key already is — <kbd>Ctrl</kbd> + <kbd>B</kbd>
unless you changed it. This names an **action**, not a path, so it keeps working wherever
herdr put the plugin. A binding of `type = "popup"` will not do here: it can only ever
*open* one, so pressing it again gets you "popup already open" rather than a toggle.

> [!IMPORTANT]
> Use any key you like in place of <kbd>'</kbd> — but tell the popup too, or it will not
> answer the chord you actually press. See [The dismiss chord](#configuring).

**That is the whole setup.** You never touch your shell config — the popup starts your
login shell, whatever `$SHELL` is, and the dismiss chord is tmux's rather than the
shell's, so it answers in any of them.

## Check it works

1. <kbd>prefix</kbd> + <kbd>'</kbd> → a bordered popup titled **scratch** opens, in the
   pane's directory.
2. Run something long: `while true; do echo tick; sleep 1; done`
3. <kbd>prefix</kbd> + <kbd>'</kbd> → popup closes.
4. <kbd>prefix</kbd> + <kbd>'</kbd> → same popup, ticks still counting, scrollback intact.
5. Close it, move to another pane in the same space and press it again → still the same
   shell, still ticking. Do the same in a pane in a *different* space and you get a new
   one.

If step 4 gives you a blank screen or a fresh prompt, see [Troubleshooting](#troubleshooting).

## Using it

A popup takes every keypress until it exits, so your herdr prefix cannot reach herdr while
one is up. The popup brings its own two keys instead:

| key | mode | what it does |
| --- | --- | --- |
| <kbd>Q</kbd> | vi normal mode, fish only | types out the dismiss command and runs it, so the scrollback says what happened |
| <kbd>prefix</kbd> + <kbd>'</kbd> | any shell, even mid-command | dismisses immediately, no prompt needed |

<kbd>Q</kbd> is for when you are sitting at a prompt. The chord is for when something is
running and there is no prompt to type at — it is the same chord that opened the popup, so
it matches the muscle memory.

### Shells other than fish

> [!NOTE]
> The chord needs nothing from your shell. It is a tmux binding, so it works the same in
> bash, zsh, nu or anything else the popup starts — including while a command is running,
> which is the case no shell binding can cover.

What is fish-only is the rest: the bare <kbd>Q</kbd>, and the finish notifications. Both
are loaded through `fish --init-command`, which is why fish costs you no shell config. bash
and zsh have no equivalent that avoids displacing your own rc file, so they are left
alone.

## How it works

Three pieces, each solving one problem herdr leaves open.

**A plugin pane, not a `type = "popup"` keybinding.** Only a plugin pane carries a title,
so the border reads `scratch` rather than the literal `popup`. And because the binding
invokes an action, it can decide between opening and closing — which is what makes one
chord toggle.

**A tmux session per space.** This is what survives the close. Dismissing detaches the tmux
client, and that client is the process herdr spawned — so herdr tears the popup down on its
own, while the session and everything in it carry on. Reopening attaches a new client, and
tmux repaints the screen it kept.

The session is keyed on herdr's space, so every pane in a space reaches the same scratch
shell. Open it in one pane, put it away, move two panes over and press the chord: the same
shell comes back, still running what you left. Only the first open picks a directory, and it
takes the one the pane you were in was sitting in.

It runs on its own tmux socket (`-L herdr-scratch`) with its own config, so it never touches
a tmux you started yourself — and that config drops tmux's prefix entirely, leaving the
dismiss chord as the one binding in the popup.

**Detach, not an API call.** Closing is a plain `tmux detach-client`, so the plugin needs no
socket client of its own — and it is a clean detach rather than herdr killing the client out
from under tmux.

## Notifications

When a command finishes in a popup you are **not** looking at, you get a notification. No
extra install, and nothing to turn on.

The shell inside the popup keeps running while detached, so fish keeps firing
`fish_postexec` after every command — that is what notices. tmux reports zero attached
clients exactly when the popup is closed, so a popup you are watching stays quiet.

herdr posts it, which is what makes it wear **your terminal's icon and name**, with a click
that focuses the terminal. It also means delivery follows your herdr config — a desktop
notification, an in-app toast, or off entirely — instead of this plugin deciding for you.

Most terminals suppress desktop notifications while they are focused, so expect these when
you are in another app — which is when they are worth having.

Commands shorter than 10 seconds are ignored. To change that, set `notify_after` in
milliseconds in [your config file](#configuring):

```toml
notify_after = 30000   # 30 seconds
```

A change here reaches the next scratch shell you *start*, not the next popup you open —
the threshold rides onto the tmux session so the shell can check it after every command
without asking this plugin. `tmux -L herdr-scratch kill-server` if you want it sooner.

A `set -g herdr_scratch_notify_after 30000` in your fish config still works, and still
wins over the file, so nothing you already have needs changing.

## Configuring

Everything you can set lives in one file:

```sh
~/.config/herdr/plugins/config/user.scratch/config.toml
```

It does not exist until you create it, and `herdr plugin config-dir user.scratch` prints
the directory it goes in — worth running first if your settings are not being picked up.

```toml
dismiss      = "C-b '"   # the two tmux keys that close the popup
notify_after = 10000     # milliseconds before a finished command notifies
width        = "70%"     # terminal cells as a number, or a percentage string
height       = "70%"
```

Those are the defaults, so a key you leave out is a key you have not changed. A file this
cannot read is ignored in favour of them rather than breaking the popup — the reason lands
in the [log](#troubleshooting).

> [!NOTE]
> `herdr-plugin.toml` is **not** where settings go. It is the plugin's own manifest, and
> `herdr-scratch link` installs each release's copy over the top of yours — which is what
> lets a release change it at all. If you had edited one, `link` prints the lines to paste
> here. Until you re-run `link`, a `--dismiss` still sitting on your pane command keeps
> winning over the file, so the popup carries on answering the chord it always did.

**The dismiss chord** — `dismiss`, in tmux's key syntax. It has to name the same chord
that *opens* the popup, and the default assumes herdr's default prefix with the binding
above. If your herdr prefix is <kbd>Ctrl</kbd> + <kbd>A</kbd>, or you bound something
other than <kbd>'</kbd>, change this to match — `"C-a ;"` for <kbd>Ctrl</kbd> +
<kbd>A</kbd> then <kbd>;</kbd>. Nothing can work it out for you: herdr has no way to
report its prefix, and the popup has to be told before it opens.

The new chord works on the next open. The old one keeps working alongside it until the
scratch tmux server exits, since tmux holds the binding rather than this plugin — end it
with `tmux -L herdr-scratch kill-server` if that bothers you.

**Size** — `width` and `height`. Terminal cells as numbers, or a percentage string like
`"70%"`. Leave both out for the shipped 70%.

**Sessions** — `tmux -L herdr-scratch ls` lists them, one per space you have used it in.

A session outlives every pane in its space, which is the point: panes come and go, the
scratch shell stays. It ends when the space does — herdr reports `workspace.closed` and the
plugin kills that space's session, so nothing is left holding a dev server nobody can reach.

Exiting the shell ends it too. The space is then back to having no scratch shell, and the
next press starts a fresh one.

## Uninstall

```sh
tmux -L herdr-scratch kill-server     # stop every scratch shell
herdr plugin unlink user.scratch      # or: herdr plugin uninstall user.scratch
rm -rf ~/.local/share/herdr-scratch   # what `herdr-scratch link` created
rm -rf ~/.config/herdr/plugins/config/user.scratch   # your settings
brew uninstall herdr-scratch          # if you installed it that way
```

Then delete the `[[keys.command]]` block from herdr's own `~/.config/herdr/config.toml`
and `herdr server reload-config`.

## Troubleshooting

> [!TIP]
> `herdr-scratch --version` reports the build herdr is actually loading. Worth checking
> first when the behaviour here does not match what you see.

**The popup opens and closes immediately.** tmux failed to start, or the binary is missing.
If you linked a local checkout, `herdr plugin link` does not build — run
`go build -o bin/herdr-scratch .` in the plugin directory.

**`herdr plugin list` shows a `Cellar/herdr-scratch/<version>` path.** herdr was pointed at
the Homebrew prefix directly, which pins it to a version an upgrade will delete. Run
`herdr-scratch link` to move the registration somewhere that survives.

**The chord opens the popup but will not close it.** Almost always the dismiss chord not
matching the one you press — see [The dismiss chord](#configuring). If
`echo $HERDR_SCRATCH_ROOT` inside the popup comes back empty, it is something else: that
shell was not started by the plugin, so close it and open a fresh popup.

**Nothing at all happens when you press it.** Read the log — every subcommand writes one,
because a keybinding's stderr goes to herdr and a popup's is inside a popup that is closing:

```sh
tail -f ~/.local/state/herdr/plugins/user.scratch/herdr-scratch.log
```

That is where herdr keeps this plugin's state, so it holds everything herdr invokes — the
keypresses, the popup, the notifications. `herdr-scratch link`, which you run yourself,
writes to `~/.local/state/herdr-scratch/herdr-scratch.log` instead — that is the one to
read when settings did not survive an upgrade.

Readable lines by default, JSON with `--debug`, which also turns on the debug records —
the space the chord fired from, what tmux said about its session, and the exact
`herdr plugin pane open` that followed. To capture a press that way, put `--debug` in the
action's command in `herdr-plugin.toml`:

```toml
command = ["./bin/herdr-scratch", "--debug", "toggle"]
```

That is a debugging edit rather than a setting, and the next `herdr-scratch link` writes
the shipped manifest back over it — put it back if you are still chasing something.

Nothing rotates the file. It gets a few lines per press, so it is a long while before that
matters; `rm` it when it does.

**<kbd>Q</kbd> does nothing in normal mode.** Almost always a `bind -M normal` somewhere in
your own config, which never fires — `default` is what fish calls vi's normal mode, and
`normal` names a mode fish never enters. Use `-M default`.

**No notification when a command finishes out of sight.** Most likely another
`fish_postexec` handler in your own config runs first and blocks. `done` is the common
one: it shells out to `terminal-notifier`, which hangs inside a popup, and fish runs
postexec handlers in order — so it starves this one and freezes the popup's prompt with
it. `functions -q __done_ended` inside the popup tells you whether it is loaded.

Everything else is visible from the outside: `herdr notification show test --body test`
proves delivery works on its own, and a plugin has no fallback if it does not — herdr does
not pass a raw OSC through from a pane, so asking herdr to post is the only route to a
notification wearing your terminal's icon.

**Reopening gives a blank screen or a fresh prompt.** Whatever is running the session is not
tmux. Check the `command` in `herdr-plugin.toml`.

## Development

```sh
go build -o bin/herdr-scratch .   # what `herdr plugin install` runs for you
go test ./...                     # the decision logic in internal/scratch
herdr plugin link "$PWD"          # point herdr at this checkout
```

> [!WARNING]
> herdr registers one copy of a plugin, so linking a checkout replaces whatever was
> registered before. A Homebrew install stops being the one herdr loads until you run
> `herdr-scratch link` again.

`internal/scratch` holds the choices worth testing — which space a binding fired from,
where a new shell should start, which shell argv to launch, how the notification is
assembled — kept apart from the processes `cmd/` runs so they can be exercised directly.

## License

MIT
