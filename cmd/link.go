package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/macintacos/herdr-scratch/internal/scratch"
)

// manifest is the one entry read before it is replaced, so `link` can say what
// the copy being overwritten was still carrying.
const manifest = "herdr-plugin.toml"

// installed are the entries copied out of the build: the binary, the tmux
// config it starts the session with, and the shell integration it sources.
var installed = []string{"bin", "tmux.conf", "shell"}

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Register this build with herdr, and install its manifest",
	Long: `Run after installing, and again after every upgrade.

herdr records a plugin by resolving its manifest and keeping the real directory
that holds it. Point herdr straight at a package manager's prefix and it records
a directory numbered by version — Cellar/herdr-scratch/<version> for a Homebrew
formula, Caskroom/herdr-scratch/<version> for a cask — which the next upgrade
deletes, taking the registration with it.

This builds a directory herdr can keep instead, and copies this release into it:
the binary, the tmux config, the shell integration, and the manifest. Every file
under it is a copy, so no upgrade can reach it. That cuts both ways — an upgraded
build reaches herdr when you run this, and not before.

The manifest is the plugin's, so every run installs this release's copy of it.
The settings you own live in config.toml instead, which this never writes to —
it only points out anything an older manifest was still carrying.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		source, err := buildRoot()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root := scratch.StableRoot(os.Getenv, home)

		if err := installBuild(source, root); err != nil {
			return err
		}

		dst := filepath.Join(root, manifest)
		shipped, err := os.ReadFile(filepath.Join(source, manifest))
		if err != nil {
			return err
		}
		// Read the one being replaced first: a manifest written before
		// config.toml existed is the last copy of whatever the user set in it.
		inPlace, err := os.ReadFile(dst)
		if err != nil && !os.IsNotExist(err) {
			// The file is about to be overwritten, so this is the only moment
			// the notice could have been produced. Say it went missing rather
			// than printing nothing and looking like there was nothing to say.
			slog.Error("could not read the manifest being replaced", "path", dst, "err", err)
		}
		carry := scratch.MigratedSettings(scratch.ManifestSettings(inPlace), scratch.ManifestSettings(shipped))
		if err := os.WriteFile(dst, shipped, 0o644); err != nil {
			return err
		}

		slog.Info("linking", "source", source, "root", root)

		register := exec.Command(herdrBin(), "plugin", "link", root)
		register.Stderr = os.Stderr
		if err := register.Run(); err != nil {
			return err
		}

		// Ignored deliberately, as everywhere else in this package: a write to
		// stdout that fails leaves nothing worth doing about it.
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed %s from %s\n", root, source)
		if len(carry) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"\nthe %s replaced here had settings of its own. Put them in\n%s:\n\n%s\n",
				manifest, scratch.ConfigPath(os.Getenv, home), strings.Join(carry, "\n"))
		}
		return nil
	},
}

// installBuild puts this release's copy of every entry the plugin needs into
// the directory herdr records, replacing whatever the last one left there.
//
// Copies rather than symlinks into the build, because the build is where the
// package manager staged it and every package manager numbers that directory by
// version. A link into one is dead the moment the version changes; a copy is
// only as stale as the last run of this command.
func installBuild(source, root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	// The copy in the plugin root is itself a runnable build, so running its
	// own `link` would hand this the same directory twice — removing each entry
	// immediately before trying to copy from it. Say so rather than emptying
	// the directory the popup runs out of.
	//
	// Both sides resolved, because comparing them as spelled does not catch it:
	// buildRoot reports the source through EvalSymlinks, while root is whatever
	// XDG_DATA_HOME or the home directory spells — and on macOS the same
	// directory reached both ways differs by /var -> /private/var alone.
	sourceReal, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if sourceReal == rootReal {
		return fmt.Errorf("%s holds the copy this command installs; run the build your package manager put on PATH", root)
	}
	// Opened once so the file entry is written through it rather than through a
	// path joined by hand. The tree entries cannot be — os.CopyFS takes a path,
	// not a root — so this is not a containment guarantee over the whole loop.
	owned, openErr := os.OpenRoot(root)
	if openErr != nil {
		return openErr
	}
	defer func() { _ = owned.Close() }()

	for _, name := range installed {
		// Replace outright rather than merge into. Up to 0.5.0 these entries
		// were symlinks into the build, so replacing them is what migrates an
		// existing install — RemoveAll unlinks one without following it, which
		// is what leaves the build those links point at alone. os.CopyFS also
		// refuses to write over a file that is already there.
		//
		// Deliberately not atomic: between the remove and the copy there is no
		// bin/herdr-scratch, so a popup opened just then dies, and a copy that
		// fails part-way leaves nothing to run. Re-running this command is the
		// repair, which is cheaper than staging every entry to rename into
		// place for a window this narrow.
		if err := owned.RemoveAll(name); err != nil {
			return err
		}
		if err := installEntry(owned, filepath.Join(source, name), name); err != nil {
			return err
		}
	}
	return nil
}

// installEntry copies one entry of the build into the directory this plugin
// owns: a file as itself, a directory as a tree.
//
// The two spellings do not agree on permissions. os.CopyFS keeps the source's
// execute bits and forces the rest to 0666 before umask — so a tree arrives as
// 0755/0644 under the usual umask and wider under a permissive one — while the
// file branch reproduces the source's mode exactly. The execute bit is the one
// that has to survive either way: the manifest's pane command execs
// bin/herdr-scratch directly.
func installEntry(owned *os.Root, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.CopyFS(filepath.Join(owned.Name(), name), os.DirFS(src))
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return owned.WriteFile(name, data, info.Mode().Perm())
}

// buildRoot is the directory this build's files sit in.
//
// os.Executable reports the path the binary was invoked by, and Homebrew puts a
// symlink on PATH — the manifest and tmux.conf sit beside the target of that
// link, not beside the link. Nothing rewrites the result: it is only ever read
// from, never recorded, so a versioned staging directory serves as well as any
// other.
//
// Deliberately does not consult HERDR_PLUGIN_ROOT the way pluginRoot does. The
// two answer opposite questions — where this build was staged, and where it was
// installed to — so unifying them would hand installBuild its own destination
// as the source.
func buildRoot() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(self)), nil // <root>/bin/herdr-scratch
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
