package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

// manifest is the one file the stable directory owns outright. Everything
// beside it is a symlink into the build, which is what lets an upgrade reach
// the plugin without herdr being told about it again.
const manifest = "herdr-plugin.toml"

// linked are the entries symlinked back into the build: the binary, the tmux
// config it starts the session with, and the shell integration it sources.
var linked = []string{"bin", "tmux.conf", "shell"}

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Register this build with herdr, and install its manifest",
	Long: `Run after installing, and again after an upgrade that changes the manifest.

The registration itself survives an upgrade — that is what this command exists to
arrange. The manifest does not: herdr keeps the copy in the directory it recorded,
and only this command replaces it. A release whose manifest changed therefore
reaches you when you run this, and not before.

herdr records a plugin by resolving its manifest and keeping the real directory
that holds it. Point herdr straight at a Homebrew prefix and it records
Cellar/herdr-scratch/<version>, which the next upgrade deletes — so the
registration would have to be redone every time.

This builds a directory herdr can keep instead: the manifest as a real file, and
the rest symlinked at opt/herdr-scratch, the path Homebrew re-points at whatever
version is current. herdr resolves to a directory that never moves, and the
symlinks under it always reach the build that is installed now.

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

		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}

		for _, name := range linked {
			dst := filepath.Join(root, name)
			// Replace rather than skip: an existing link points at wherever the
			// last install was, which is the thing this command exists to fix.
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(filepath.Join(source, name), dst); err != nil {
				return err
			}
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

		fmt.Fprintf(cmd.OutOrStdout(), "linked %s -> %s\n", root, source)
		if len(carry) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nthe %s replaced here had settings of its own. Put them in\n%s:\n\n%s\n",
				manifest, scratch.ConfigPath(os.Getenv, home), strings.Join(carry, "\n"))
		}
		return nil
	},
}

// buildRoot is the directory this build's files sit in, spelled so it keeps
// resolving after an upgrade.
//
// os.Executable reports the path the binary was invoked by, and Homebrew puts a
// symlink on PATH — the manifest and tmux.conf sit beside the target of that
// link, not beside the link.
func buildRoot() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", err
	}
	return scratch.StableSource(filepath.Dir(filepath.Dir(self))), nil // <root>/bin/herdr-scratch
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
