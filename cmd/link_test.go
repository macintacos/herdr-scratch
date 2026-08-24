package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// stageBuild writes a build tree of the shape `link` copies out of, into dir:
// the binary under bin/, the tmux config beside it, the shell integration under
// shell/.
//
// Every file carries the marker, so a test can tell one release's copy from the
// next without caring what is actually in them.
func stageBuild(t *testing.T, dir, marker string) {
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
}

// readInstalled is the installed copy of one entry, as bytes and mode.
//
// Lstat rather than Stat, and a real-file assertion before either: os.Stat
// reports the target's mode through a symlink, so every check built on it would
// pass just as happily against the symlinks this change exists to replace.
func readInstalled(t *testing.T, path string) (string, os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %v, want a real file rather than something that resolves to one", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), info.Mode().Perm()
}

func TestInstallBuildKeepsTheBinaryExecutable(t *testing.T) {
	// The manifest's pane command execs $HERDR_PLUGIN_ROOT/bin/herdr-scratch
	// directly, so a copy that lost the execute bit is a popup that never
	// opens.
	source := filepath.Join(t.TempDir(), "build")
	stageBuild(t, source, "0.9.0")
	root := filepath.Join(t.TempDir(), "herdr-scratch")

	if err := installBuild(source, root); err != nil {
		t.Fatal(err)
	}

	if _, mode := readInstalled(t, filepath.Join(root, "bin", "herdr-scratch")); mode&0o111 == 0 {
		t.Errorf("installed binary mode = %v, want the execute bits set", mode)
	}
}

func TestInstallBuildReplacesTheSymlinksAnOlderReleaseLeft(t *testing.T) {
	// Releases up to 0.5.0 symlinked these entries into the build, so every
	// upgrade to this one starts from links rather than from copies. Removing
	// one has to unlink it and leave the build it points at alone — following
	// it would delete the Homebrew install the link resolves into.
	build := filepath.Join(t.TempDir(), "opt", "herdr-scratch")
	stageBuild(t, build, "0.5.0")
	root := filepath.Join(t.TempDir(), "herdr-scratch")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range installed {
		if err := os.Symlink(filepath.Join(build, name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := installBuild(build, root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(build, "bin", "herdr-scratch")); err != nil {
		t.Errorf("the build the old links pointed at lost a file: %v", err)
	}
	for _, name := range installed {
		if _, err := os.Lstat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	// readInstalled fails anything that is not a real file, so this is the
	// assertion that the link became a copy.
	if got, _ := readInstalled(t, filepath.Join(root, "tmux.conf")); got != "0.5.0" {
		t.Errorf("tmux.conf = %q, want %q", got, "0.5.0")
	}
}

func TestInstallBuildRefusesToInstallOverItself(t *testing.T) {
	// The installed copy is a runnable build, so `link` can be invoked from it.
	// Left unguarded that removes each entry just before copying from it, which
	// empties the directory the popup runs out of.
	root := filepath.Join(t.TempDir(), "herdr-scratch")
	stageBuild(t, root, "1.0.0")

	// Spelled two ways as well as one. buildRoot resolves the source through
	// EvalSymlinks while root keeps whatever XDG_DATA_HOME said, so a guard that
	// compares them as written misses the real case entirely — on macOS the two
	// spellings of a temp directory differ by /var -> /private/var.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}

	for _, source := range []string{root, alias} {
		if err := installBuild(source, root); err == nil {
			t.Errorf("installBuild(%q, root) = nil, want an error rather than a wiped plugin root", source)
		}
		if got, _ := readInstalled(t, filepath.Join(root, "bin", "herdr-scratch")); got != "1.0.0" {
			t.Fatalf("the binary was disturbed by installBuild(%q, root): %q, want %q", source, got, "1.0.0")
		}
	}
}

func TestInstallBuildSurvivesTheStagingDirectoryBeingReplaced(t *testing.T) {
	// The upgrade this command exists to withstand. A cask stages at
	// Caskroom/<token>/<version> and Homebrew deletes that directory on
	// upgrade, so anything the plugin root still points into goes with it.
	// Owning the copy is what makes the root independent of the staging path.
	caskroom := filepath.Join(t.TempDir(), "Caskroom", "herdr-scratch")
	root := filepath.Join(t.TempDir(), "herdr-scratch")

	staged := filepath.Join(caskroom, "1.0.0")
	stageBuild(t, staged, "1.0.0")
	if err := installBuild(staged, root); err != nil {
		t.Fatal(err)
	}
	// An entry this release ships and the next one drops, to prove the install
	// replaces each tree outright rather than merging into what is there.
	if err := os.WriteFile(filepath.Join(root, "shell", "gone.fish"), []byte("1.0.0"), 0o644); err != nil {
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
		if got, _ := readInstalled(t, filepath.Join(root, entry)); got != "1.0.0" {
			t.Errorf("%s = %q after the staging directory was replaced, want %q", entry, got, "1.0.0")
		}
	}

	// And the re-link picks the new release up, which is what makes the
	// snapshot a snapshot rather than a one-time freeze.
	if err := installBuild(filepath.Join(caskroom, "1.1.0"), root); err != nil {
		t.Fatal(err)
	}
	if got, _ := readInstalled(t, filepath.Join(root, "bin", "herdr-scratch")); got != "1.1.0" {
		t.Errorf("binary = %q after re-linking, want %q", got, "1.1.0")
	}
	if _, err := os.Lstat(filepath.Join(root, "shell", "gone.fish")); !os.IsNotExist(err) {
		t.Errorf("shell/gone.fish survived the release that dropped it: %v", err)
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

	trees := map[string]map[string]string{}
	for name, shape := range shapes {
		source := filepath.Join(t.TempDir(), shape)
		stageBuild(t, source, "2.0.0")
		root := filepath.Join(t.TempDir(), "herdr-scratch")
		if err := installBuild(source, root); err != nil {
			t.Fatal(err)
		}
		trees[name] = walk(t, root)
	}

	if len(trees["homebrew"]) == 0 {
		t.Fatal("nothing was installed, so the comparison below proves nothing")
	}
	for entry, want := range trees["checkout"] {
		if got := trees["homebrew"][entry]; got != want {
			t.Errorf("%s = %q from a Homebrew-shaped source, want %q as from a checkout", entry, got, want)
		}
	}
	if len(trees["homebrew"]) != len(trees["checkout"]) {
		t.Errorf("installed %d entries from a Homebrew-shaped source, want the %d a checkout installs",
			len(trees["homebrew"]), len(trees["checkout"]))
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
		data, mode := readInstalled(t, path)
		tree[rel] = data + " " + mode.String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
