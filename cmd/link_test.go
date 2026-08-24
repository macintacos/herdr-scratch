package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// stageBuild writes a build tree of the shape `link` copies out of — the binary
// under bin/, the tmux config beside it, the shell integration under shell/ —
// and returns the directory holding it.
//
// Every file carries the marker, so a test can tell one release's copy from the
// next without caring what is actually in them.
func stageBuild(t *testing.T, dir, marker string) string {
	t.Helper()
	for path, mode := range map[string]os.FileMode{
		filepath.Join("bin", "herdr-scratch"):        0o755,
		"tmux.conf":                                  0o644,
		filepath.Join("shell", "herdr-scratch.fish"): 0o644,
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(marker), mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// read is the installed copy of one entry, as bytes and mode.
func read(t *testing.T, path string) (string, os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), info.Mode().Perm()
}

func TestInstallBuildKeepsTheBinaryExecutable(t *testing.T) {
	// The manifest's pane command execs $HERDR_PLUGIN_ROOT/bin/herdr-scratch
	// directly, so a copy that lost the execute bit is a popup that never
	// opens.
	source := stageBuild(t, filepath.Join(t.TempDir(), "build"), "0.9.0")
	root := filepath.Join(t.TempDir(), "herdr-scratch")

	if err := installBuild(source, root); err != nil {
		t.Fatal(err)
	}

	if _, mode := read(t, filepath.Join(root, "bin", "herdr-scratch")); mode&0o111 == 0 {
		t.Errorf("installed binary mode = %v, want the execute bits set", mode)
	}
}

func TestInstallBuildSurvivesTheStagingDirectoryBeingReplaced(t *testing.T) {
	// The upgrade this command exists to withstand. A cask stages at
	// Caskroom/<token>/<version> and Homebrew deletes that directory on
	// upgrade, so anything the plugin root still points into goes with it.
	// Owning the copy is what makes the root independent of the staging path.
	caskroom := filepath.Join(t.TempDir(), "Caskroom", "herdr-scratch")
	root := filepath.Join(t.TempDir(), "herdr-scratch")

	staged := stageBuild(t, filepath.Join(caskroom, "1.0.0"), "1.0.0")
	if err := installBuild(staged, root); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(staged); err != nil {
		t.Fatal(err)
	}
	stageBuild(t, filepath.Join(caskroom, "1.1.0"), "1.1.0")

	for _, entry := range []string{
		filepath.Join("bin", "herdr-scratch"),
		"tmux.conf",
		filepath.Join("shell", "herdr-scratch.fish"),
	} {
		if got, _ := read(t, filepath.Join(root, entry)); got != "1.0.0" {
			t.Errorf("%s = %q after the staging directory was replaced, want %q", entry, got, "1.0.0")
		}
	}

	// And the re-link picks the new release up, which is what makes the
	// snapshot a snapshot rather than a one-time freeze.
	if err := installBuild(filepath.Join(caskroom, "1.1.0"), root); err != nil {
		t.Fatal(err)
	}
	if got, _ := read(t, filepath.Join(root, "bin", "herdr-scratch")); got != "1.1.0" {
		t.Errorf("binary = %q after re-linking, want %q", got, "1.1.0")
	}
}

func TestInstallBuildDoesNotCareWhatShapeTheSourcePathHas(t *testing.T) {
	// There is no longer a path this command rewrites, so a Homebrew prefix, a
	// git checkout, and a `herdr plugin install` build all install identically.
	// That equivalence is what replaced the Cellar special case.
	shapes := map[string]string{
		"homebrew": filepath.Join("Caskroom", "herdr-scratch", "2.0.0"),
		"checkout": filepath.Join("Users", "me", ".config", "herdr", "scratch"),
	}

	installed := map[string]map[string]string{}
	for name, shape := range shapes {
		source := stageBuild(t, filepath.Join(t.TempDir(), shape), "2.0.0")
		root := filepath.Join(t.TempDir(), "herdr-scratch")
		if err := installBuild(source, root); err != nil {
			t.Fatal(err)
		}
		installed[name] = walk(t, root)
	}

	if len(installed["homebrew"]) == 0 {
		t.Fatal("nothing was installed, so the comparison below proves nothing")
	}
	for entry, want := range installed["checkout"] {
		if got := installed["homebrew"][entry]; got != want {
			t.Errorf("%s = %q from a Homebrew-shaped source, want %q as from a checkout", entry, got, want)
		}
	}
	if len(installed["homebrew"]) != len(installed["checkout"]) {
		t.Errorf("installed %d entries from a Homebrew-shaped source, want the %d a checkout installs",
			len(installed["homebrew"]), len(installed["checkout"]))
	}
}

// walk describes an installed tree as relative path -> contents and mode, so
// two of them can be compared without caring which temp directory they sit in.
func walk(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, mode := read(t, path)
		tree[rel] = data + " " + mode.String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
