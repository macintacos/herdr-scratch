# fish integration for herdr-scratch.
#
# Sourced automatically: the popup starts fish with
# `--init-command "source <this file>"`, so there is nothing to add to your own
# fish config. Everything here is scoped to the popup's own shell and cannot
# reach any other pane.

status is-interactive; or return
set -q HERDR_SCRATCH_POPUP; or return

function herdr-scratch-dismiss --description "Close the scratch popup, leaving its shell running"
    command "$HERDR_SCRATCH_ROOT/bin/herdr-scratch" dismiss
end

# `q` alone, in vi normal mode. Bound as `default` because that is what fish
# calls vi's command mode — `bind -M normal` is accepted but names a mode fish
# never enters, so it would silently never fire.
#
# It writes the command out and submits it rather than running it invisibly, so
# the scrollback says what happened instead of the popup just vanishing.
bind -M default q "commandline -r herdr-scratch-dismiss; commandline -f execute"

# Tell you when something finishes in a popup you are not looking at — the dev
# server that died while you were somewhere else.
#
# fish_postexec is what makes this possible: fish fires it after every command,
# and keeps firing while detached, because the shell never stopped running. tmux
# reports 0 attached clients exactly when the popup is closed, so a popup you
# are watching stays quiet.
function __herdr_scratch_notify --on-event fish_postexec
    # Lowest precedence first. HERDR_SCRATCH_NOTIFY_AFTER carries notify_after
    # from config.toml: the popup puts it on the tmux session, so every shell in
    # the space shares one threshold without any of them reading the file. The
    # fish variable is set last and so wins, which leaves a
    # herdr_scratch_notify_after somebody already has doing what it always did.
    #
    # Both read at call time, not load time, so setting either anywhere in your
    # config takes effect.
    set -l after 10000
    set -q HERDR_SCRATCH_NOTIFY_AFTER; and set after $HERDR_SCRATCH_NOTIFY_AFTER
    set -q herdr_scratch_notify_after; and set after $herdr_scratch_notify_after

    test "$CMD_DURATION" -ge "$after"; or return
    test (tmux display-message -p '#{session_attached}' 2>/dev/null) = 0; or return

    command "$HERDR_SCRATCH_ROOT/bin/herdr-scratch" notify \
        (string shorten --max 60 -- "$argv[1]") \
        "finished after "(math -s0 $CMD_DURATION / 1000)"s"
end
