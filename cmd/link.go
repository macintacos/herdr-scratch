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
the binary, the tmux config, the shell integration, and the manifest. Nothing
under it refers back to wherever the package was staged, so no upgrade can reach
it. That cuts both ways — an upgraded build reaches herdr when you run this, and
not before.

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
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "linked %s -> %s\n", root, source)
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
	// Opened once and written through, rather than joining each destination by
	// hand, so an entry can only ever land inside the directory this plugin
	// owns.
	owned, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = owned.Close() }()

	for _, name := range installed {
		// Replace outright rather than merge into: what is here is the previous
		// release's copy, and os.CopyFS refuses to write over a file that
		// already exists.
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
// Both spellings carry the source's permission bits across, which is what keeps
// bin/herdr-scratch executable — the manifest's pane command execs it directly.
// Only the file can go through owned; os.CopyFS takes a path, so the tree is
// joined against the same root rather than written through it.
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
