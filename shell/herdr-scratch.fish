# fish integration for herdr-scratch.
#
# Source it from your fish config, guarded on the variable the plugin exports
# into the popup's shell — no path of your own to keep in sync:
#
#   if set -q HERDR_SCRATCH_ROOT
#       source $HERDR_SCRATCH_ROOT/shell/herdr-scratch.fish
#   end
#
# Outside the scratch popup HERDR_SCRATCH_ROOT is unset, so this file is never
# read and `q` and ctrl+b keep whatever they already meant to you.

status is-interactive; or return
set -q HERDR_SCRATCH_POPUP; or return

function herdr-scratch-dismiss --description "Close the scratch popup, leaving its shell running"
    command "$HERDR_SCRATCH_ROOT/bin/herdr-scratch-dismiss"
end

# `q` alone, in vi normal mode. Bound as `default` because that is what fish
# calls vi's command mode — `bind -M normal` is accepted but names a mode fish
# never enters, so it would silently never fire.
#
# It writes the command out and submits it rather than running it invisibly, so
# the scrollback says what happened instead of the popup just vanishing.
bind -M default q "commandline -r herdr-scratch-dismiss; commandline -f execute"

# The prefix chord, for when a command is running and there is no prompt to type
# `q` at. herdr never sees this in here — a popup takes all terminal input — so
# fish answers it directly. tmux.conf unbinds tmux's own prefix so ctrl+b gets
# this far.
bind -M insert ctrl-b,"'" herdr-scratch-dismiss
bind -M default ctrl-b,"'" herdr-scratch-dismiss

# Tell you when something finishes in a popup you are not looking at — the dev
# server that died while you were somewhere else.
#
# fish_postexec is what makes this possible: fish fires it after every command,
# and keeps firing it while detached, because the shell never stopped running.
# tmux reports 0 attached clients exactly when the popup is closed, so an open
# popup you are watching stays quiet.
function __herdr_scratch_notify --on-event fish_postexec
    # Read at call time, not load time, so setting it after the `source` above
    # still works.
    set -l after 10000
    set -q herdr_scratch_notify_after; and set after $herdr_scratch_notify_after

    test "$CMD_DURATION" -ge "$after"; or return
    test (tmux display-message -p '#{session_attached}' 2>/dev/null) = 0; or return

    command "$HERDR_SCRATCH_ROOT/bin/herdr-scratch-notify" \
        (string shorten --max 60 -- "$argv[1]") \
        "finished after "(math -s0 $CMD_DURATION / 1000)"s"
end
