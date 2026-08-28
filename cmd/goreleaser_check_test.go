package cmd_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tolerance `.mise/tasks/goreleaser-check` implements is exactly one
// deprecation wide, and its own header says why. What this holds it to is that
// the width does not quietly grow: a gate that waves a second deprecation
// through while printing "tolerated" is worse than no gate.
//
// Guarded from Go rather than a shell test framework the repo does not have.
// The task is in Go(Test)'s glob so that editing the tolerance logic — exactly
// the change this is written for — re-runs it.
const (
	// Valid, no deprecated properties.
	cleanConfig = "version: 2\nproject_name: x\n"
	// Valid, one deprecated property that is not brews.
	otherDeprecationConfig = "version: 2\nproject_name: x\n" +
		"snapshot:\n  name_template: \"{{ .Version }}-SNAP\"\n"
	// The boundary the task exists to defend: brews AND something else. The
	// tolerated case with one more deprecation on top of it.
	brewsPlusOtherConfig = "version: 2\nproject_name: x\n" +
		"snapshot:\n  name_template: \"{{ .Version }}-SNAP\"\n" +
		"brews:\n  - repository:\n      owner: x\n      name: y\n"
	// Valid, and the one deprecation the task forgives.
	brewsOnlyConfig = "version: 2\nproject_name: x\n" +
		"brews:\n  - repository:\n      owner: x\n      name: y\n"
	// Not valid at all: goreleaser fails this at parse time.
	brokenConfig = "version: 2\nbuilds:\n  - nope: true\n"
)

// runTask runs .mise/tasks/goreleaser-check over body and reports its exit code
// and combined output.
func runTask(t *testing.T, body string) (int, string) {
	t.Helper()

	config := filepath.Join(t.TempDir(), "goreleaser.yaml")
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return runTaskOn(t, config)
}

// runTaskOn is runTask against a config that already exists on disk, so the
// repo's own file can be checked without copying it.
func runTaskOn(t *testing.T, config string) (int, string) {
	t.Helper()

	if _, err := exec.LookPath("goreleaser"); err != nil {
		t.Skip("goreleaser not on PATH; run under `mise run test`")
	}

	out, err := exec.Command("../.mise/tasks/goreleaser-check", config).CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run task: %v\n%s", err, out)
	}
	// Only on the failing path: a passing run's output says nothing the
	// assertion does not, and the failing one is what needs explaining.
	t.Logf("task output for %s:\n%s", config, out)
	return exit.ExitCode(), string(out)
}

// TestGoreleaserCheckTaskTolerance pins the one deprecation the gate forgives
// and proves it forgives nothing else.
func TestGoreleaserCheckTaskTolerance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		want   int
	}{
		{"clean config passes", cleanConfig, 0},
		{"the brews deprecation alone passes", brewsOnlyConfig, 0},
		{"a deprecation other than brews still fails", otherDeprecationConfig, 1},
		{"brews plus another deprecation still fails", brewsPlusOtherConfig, 1},
		{"an unparseable config still fails", brokenConfig, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, _ := runTask(t, tc.config); got != tc.want {
				t.Errorf("exit = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestGoreleaserCheckTaskOnlyClaimsToleranceWhenItTolerated pins the *claim*,
// not just the exit code. Exiting 0 is the right answer on a clean config too,
// so an exit code alone cannot tell a config that needed forgiving from one
// that did not — and "tolerated" printed over a config the task did not
// actually inspect is the failure that would look like success.
func TestGoreleaserCheckTaskOnlyClaimsToleranceWhenItTolerated(t *testing.T) {
	const claim = "brews is the only deprecation; tolerated"

	if _, out := runTask(t, brewsOnlyConfig); !strings.Contains(out, claim) {
		t.Errorf("brews-only output did not claim tolerance:\n%s", out)
	}
	if _, out := runTask(t, cleanConfig); strings.Contains(out, claim) {
		t.Errorf("clean config claimed tolerance it never needed:\n%s", out)
	}
}

// TestGoreleaserCheckTaskAcceptsThisRepo is the case the gate runs on every
// commit. It is separate from the table because it reads the real file: a
// fixture spelling `brews` by hand would keep passing after the config moved
// on, which is the drift this is here to catch.
func TestGoreleaserCheckTaskAcceptsThisRepo(t *testing.T) {
	if got, _ := runTaskOn(t, "../.goreleaser.yaml"); got != 0 {
		t.Errorf("exit = %d, want 0 — the repo's own config must pass its own gate", got)
	}
}
