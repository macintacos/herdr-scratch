# herdr-scratch

A scratch shell for [herdr](https://herdr.dev), in a popup you toggle with one chord.

Press it and a shell opens over whatever you were doing, in that pane's directory.
Press it again and the popup goes away — but the shell does not. Start a dev server,
put the popup away, bring it back an hour later and it is still running, with the
scrollback exactly where you left it.

```
prefix + '     →  scratch shell opens, 70% of the screen, cwd of the pane you were in
prefix + '     →  popup closes; whatever is running keeps running
prefix + '     →  same shell, same output, same scroll position
```

macOS and Linux. Not Windows — it depends on a Unix domain socket and on tmux.

## Requirements

| | |
| --- | --- |
| **herdr** | 0.8.0+ (`herdr --version`). Earlier versions have no `plugin pane` command |
| **tmux** | `brew install tmux` / `apt install tmux`. It is what keeps the shell alive *and* the screen intact |
| **nc** | with `-U` support. Preinstalled on macOS; on Linux `apt install netcat-openbsd` |
| **terminal-notifier** | optional, macOS only, for finish notifications |

## Install

```sh
herdr plugin install macintacos/herdr-scratch
```

Or clone and link it, if you would rather manage the checkout yourself:

```sh
git clone https://github.com/macintacos/herdr-scratch ~/.config/herdr/scratch
herdr plugin link ~/.config/herdr/scratch
```

Installing does not bind anything. Two steps remain: a key, and — if you want to close
the popup from inside it — the shell integration.

### 1. Bind a key

In `~/.config/herdr/config.toml`, appended alongside any bindings you already have:

```toml
[[keys.command]]
key         = "prefix+'"
type        = "plugin_action"
command     = "user.scratch.toggle"
description = "scratch shell"
```

Then `herdr server reload-config` (start herdr first if it is not running).

`prefix` means whatever your herdr prefix key already is — `ctrl+b` unless you changed
it. Pick any key you like in place of `'`.

This names an **action**, not a path, so it keeps working wherever herdr put the plugin.
If you linked a local clone and would rather point at the script, this works too:

```toml
type    = "shell"
command = '"$HOME/.config/herdr/scratch/bin/herdr-scratch-toggle"'
```

What you cannot use is `type = "popup"`. A popup binding can only ever *open* one — press
it again and herdr answers "popup already open", so it never toggles.

### 2. Shell integration

**Why this is a separate step.** A herdr popup, per the herdr docs, "receives all terminal
input, including Escape, until its command exits." Your prefix key does not reach herdr
while a popup is up. So the chord above opens the popup but cannot close it — only
something running *inside* the popup can do that.

The plugin exports `HERDR_SCRATCH_ROOT` into the popup's shell, pointing at wherever it is
installed. Guard on it and you need no path of your own:

```fish
if set -q HERDR_SCRATCH_ROOT
    source $HERDR_SCRATCH_ROOT/shell/herdr-scratch.fish
end
```

Open a new shell for that to take effect. Inside the scratch popup you then get:

| key | mode | what it does |
| --- | --- | --- |
| `q` | vi normal mode | writes out `herdr-scratch-dismiss` and runs it |
| `ctrl+b` `'` | any mode | dismisses immediately, no prompt needed |

`q` is for when you are sitting at a prompt. The chord is for when something is running and
there is no prompt to type at — it is the same chord that opened the popup, so it matches
the muscle memory, just answered by fish instead of herdr.

Outside the popup `HERDR_SCRATCH_ROOT` is unset, the file is never read, and `q` and
`ctrl+b` keep whatever they already meant to you.

**If you use vi mode, note the mode name.** Bindings go on `-M default`, because `default`
is what fish calls vi's normal mode. `bind -M normal …` is accepted without error and then
never fires, because it creates a mode fish never enters.

### Other shells

`bin/herdr-scratch-dismiss` is a plain POSIX script with no fish in it. Bind it however
your shell binds things — but **keep the guard**, or you will bind the chord in every shell
you open, not just the popup:

```bash
# bash — ctrl+b then '  (\047 is the apostrophe, which avoids quoting trouble)
if [ -n "${HERDR_SCRATCH_ROOT:-}" ]; then
    bind -x '"\C-b\047": "$HERDR_SCRATCH_ROOT/bin/herdr-scratch-dismiss"'
fi
```

```zsh
# zsh — ctrl+b then '
if [[ -n ${HERDR_SCRATCH_ROOT:-} ]]; then
    herdr-scratch-dismiss() { "$HERDR_SCRATCH_ROOT/bin/herdr-scratch-dismiss" }
    zle -N herdr-scratch-dismiss
    bindkey "^b'" herdr-scratch-dismiss
fi
```

There is no bash or zsh equivalent of the bare `q` binding shipped here — it relies on
fish's vi mode having a distinct command mode to bind into. In zsh's `vi-cmd-mode` you
could do the same with `bindkey -M vicmd q herdr-scratch-dismiss`; you are on your own for
the details. The notification hook is likewise fish-only, since it hangs off
`fish_postexec`.

## Check it works

1. `<prefix>'` → a bordered popup titled **scratch** opens, in the pane's directory.
2. Run something long: `while true; do echo tick; sleep 1; done`
3. `<prefix>'` → popup closes.
4. `<prefix>'` → same popup, ticks still counting, scrollback intact.

If step 4 gives you a blank screen or a fresh prompt, see Troubleshooting.

## How it works

Three pieces, each solving one problem herdr leaves open.

**A plugin pane, not a popup keybinding.** Only a plugin pane carries a title, so the
border reads `scratch` rather than the literal `popup`. And because the binding invokes a
*command*, it can ask herdr `popup.close` first and open only when the answer is
`popup_not_open` — which is what makes one chord toggle.

**A tmux session per pane.** This is what survives the close. When you dismiss the popup,
herdr kills the process it spawned — which is a tmux *client*. The session that client was
attached to keeps running, along with everything in it. Reopening attaches a new client to
the same session, and tmux repaints the screen it kept.

It has to be tmux, or something else that emulates a terminal. abduco and dtach are smaller
and hold a process open just fine, but they only pipe bytes — they keep no screen, so
reattaching gives you a bare prompt and the previous output is gone. If you only need "the
process survives", they are enough. For "it comes back exactly as I left it", they are not.

The session is keyed on herdr's pane id, so each pane gets its own scratch shell starting
in that pane's directory. It runs on its own tmux socket (`-L herdr-scratch`) with its own
config, so it never touches a tmux you started yourself — and that config unbinds tmux's
prefix, which is otherwise `ctrl+b` and would eat the dismiss chord.

**A socket call to close it.** herdr has no CLI for `popup.close`, so both scripts speak its
socket API directly over `nc -U "$HERDR_SOCKET_PATH"` — herdr sets that variable for
everything it runs. That is the whole reason `nc` is a requirement.

## Notifications

Optional, macOS only, fish only, and silent unless `terminal-notifier` is installed.

```sh
brew install terminal-notifier
```

When a command finishes in a popup you are **not** looking at, you get a desktop
notification. The shell inside the popup keeps running while detached, so fish keeps firing
`fish_postexec` after every command — that is what notices. tmux reports zero attached
clients exactly when the popup is closed, so a popup you are watching stays quiet.

The notification wears **your terminal's icon**, and clicking it activates your terminal.
That comes from passing `-sender` with the host app's bundle id, which macOS provides in
`__CFBundleIdentifier` and which survives down through herdr into the popup — so it follows
whichever terminal is actually attached rather than being pinned to one. Without `-sender`
the notification arrives under terminal-notifier's own identity: a stock icon, and no way
to change it.

Commands shorter than 10 seconds are ignored. To change that, set the threshold in
milliseconds anywhere in your fish config:

```fish
set -g herdr_scratch_notify_after 30000   # 30 seconds
```

## Configuring

**Size** — `width` and `height` in `herdr-plugin.toml`. Terminal cells as numbers, or a
percentage string like `"70%"`. Omit both for herdr's default half-size popup.

**Sessions** — `tmux -L herdr-scratch ls` lists them, one per pane you have used it in.

They outlive the pane that created them. Close that herdr pane and its session keeps
running, but nothing will attach to it again: the next pane gets a new id, so it gets a new
session. Orphans sit there until you kill them or reboot. If you leave heavy things running
in scratch shells, check that list now and then.

## Uninstall

```sh
tmux -L herdr-scratch kill-server     # stop every scratch shell
herdr plugin uninstall user.scratch   # or: herdr plugin unlink user.scratch
```

Then delete the `[[keys.command]]` block from `config.toml` and the `source` block from
your shell config, and `herdr server reload-config`.

## Troubleshooting

**The popup opens and closes immediately.** tmux failed to start. Run it by hand to see the
error, substituting your install path:
`tmux -L herdr-scratch -f <plugin>/tmux.conf new-session -A -s test`

**The chord opens the popup but will not close it.** The shell integration is not loaded.
Inside the popup, check `HERDR_SCRATCH_ROOT` is set — if it is unset, this shell was not
started by the plugin; if it is set, your shell config is not sourcing the file, or you have
not opened a new shell since adding it.

**`q` does nothing in normal mode.** Almost always a `bind -M normal` somewhere in your own
config, which never fires. Use `-M default`.

**Reopening gives a blank screen or a fresh prompt.** Whatever is running the session is not
tmux. Check the `command` in `herdr-plugin.toml` — a session manager that does not emulate a
terminal cannot repaint one.

**`nc: invalid option -- 'U'`.** Your `nc` is the GNU or Nmap build. Install
`netcat-openbsd`, or put the BSD one earlier on `PATH`.

## License

MIT
