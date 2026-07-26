// SPDX-License-Identifier: Apache-2.0 OR MIT

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Mr-Tao/libinput-curve/internal/xinput"
)

func TestValidateVersionAndWaylandBoundary(t *testing.T) {
	configPath := writeTestConfig(t)
	var stdout, stderr bytes.Buffer
	app := testApplication(&stdout, &stderr, newStateRunner())

	if code := app.run([]string{"validate", "--config", configPath}); code != exitOK {
		t.Fatalf("validate exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "profiles=1 devices=1") {
		t.Fatalf("unexpected validate output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"version"}); code != exitOK || strings.TrimSpace(stdout.String()) != version {
		t.Fatalf("version exit=%d stdout=%q", code, stdout.String())
	}

	app.getenv = func(name string) string {
		if name == "XDG_SESSION_TYPE" {
			return "wayland"
		}
		return ""
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"devices"}); code != exitUnavailable ||
		!strings.Contains(stderr.String(), "compositor-owned") {
		t.Fatalf("wayland exit=%d stderr=%s", code, stderr.String())
	}
}

func TestStatusApplyAndDevices(t *testing.T) {
	configPath := writeTestConfig(t)
	runner := newStateRunner()
	var stdout, stderr bytes.Buffer
	app := testApplication(&stdout, &stderr, runner)

	if code := app.run([]string{"status", "--config", configPath}); code != exitOK {
		t.Fatalf("initial status exit=%d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}

	runner.properties["libinput Accel Custom Motion Points"] = []string{"0", "0.5", "2"}
	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"status", "--config", configPath}); code != exitDrift {
		t.Fatalf("drift status exit=%d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "state: drift") {
		t.Fatalf("missing drift output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"apply", "--config", configPath}); code != exitOK {
		t.Fatalf("apply exit=%d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "state: in-sync") ||
		!strings.Contains(stderr.String(), "applied and verified 1 property changes") {
		t.Fatalf("unexpected apply output:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if !reflect.DeepEqual(
		runner.properties["libinput Accel Custom Motion Points"],
		[]string{"0.000000", "1.000000", "3.000000"},
	) {
		t.Fatalf("property not applied: %#v", runner.properties)
	}

	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"devices", "--format", "json"}); code != exitOK {
		t.Fatalf("devices exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"hardware_id": "045e:07a5"`) ||
		!strings.Contains(stdout.String(), `"custom_enabled": true`) {
		t.Fatalf("unexpected devices JSON: %s", stdout.String())
	}
}

func TestRenderXorgAtomicOutput(t *testing.T) {
	configPath := writeTestConfig(t)
	outputPath := filepath.Join(t.TempDir(), "nested", "90-libinput-curve.conf")
	var stdout, stderr bytes.Buffer
	app := testApplication(&stdout, &stderr, newStateRunner())

	if code := app.run([]string{
		"render-xorg",
		"--config", configPath,
		"--output", outputPath,
	}); code != exitOK {
		t.Fatalf("render exit=%d stderr=%s", code, stderr.String())
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `MatchUSBID "045e:07a5"`) {
		t.Fatalf("unexpected xorg output: %s", content)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode=%o, want 644", info.Mode().Perm())
	}
}

func TestMutationLockIsExclusive(t *testing.T) {
	runtimeDirectory := t.TempDir()
	app := application{getenv: func(name string) string {
		if name == "XDG_RUNTIME_DIR" {
			return runtimeDirectory
		}
		return ""
	}}
	first, err := app.acquireMutationLock()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	if second, err := app.acquireMutationLock(); err == nil {
		second.Close()
		t.Fatal("second writer acquired the same lock")
	}
}

func TestCompletionCommand(t *testing.T) {
	expectedMarkers := map[string][]string{
		"bash": {"complete -F _libinput_curve", "--motion-scale", "render-xorg"},
		"zsh":  {"#compdef libinput-curve", "--motion-scale", "render-xorg"},
		"fish": {"complete -c libinput-curve", "-l motion-scale", "render-xorg"},
	}

	for shell, markers := range expectedMarkers {
		t.Run(shell, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			app := testApplication(&stdout, &stderr, newStateRunner())
			if code := app.run([]string{"completion", shell}); code != exitOK {
				t.Fatalf("completion exit=%d stderr=%s", code, stderr.String())
			}
			for _, marker := range markers {
				if !strings.Contains(stdout.String(), marker) {
					t.Errorf("completion output does not contain %q", marker)
				}
			}
			for _, command := range completionCommands {
				if !strings.Contains(stdout.String(), command.name) {
					t.Errorf("completion output does not contain command %q", command.name)
				}
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", stderr.String())
			}
		})
	}

	var stdout, stderr bytes.Buffer
	app := testApplication(&stdout, &stderr, newStateRunner())
	if code := app.run([]string{"completion", "tcsh"}); code != exitUsage {
		t.Fatalf("unsupported shell exit=%d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "expected bash, zsh, or fish") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestCompletionSpecsMatchCommandFlags(t *testing.T) {
	flagPattern := regexp.MustCompile(`(?m)^  -([a-z][a-z0-9-]*)`)

	for _, command := range completionCommands {
		if len(command.options) == 0 {
			continue
		}
		t.Run(command.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			app := testApplication(&stdout, &stderr, newStateRunner())
			if code := app.run([]string{command.name, "-h"}); code != exitUsage {
				t.Fatalf("help exit=%d, want %d", code, exitUsage)
			}

			var actual []string
			for _, match := range flagPattern.FindAllStringSubmatch(stderr.String(), -1) {
				actual = append(actual, match[1])
			}
			var declared []string
			for _, option := range command.options {
				declared = append(declared, option.name)
			}
			slices.Sort(actual)
			slices.Sort(declared)
			if !slices.Equal(actual, declared) {
				t.Fatalf(
					"completion flags do not match parser flags:\n actual: %v\ndeclared: %v\nhelp:\n%s",
					actual,
					declared,
					stderr.String(),
				)
			}
		})
	}
}

func testApplication(
	stdout *bytes.Buffer,
	stderr *bytes.Buffer,
	runner *stateRunner,
) application {
	return application{
		stdout: stdout,
		stderr: stderr,
		getenv: func(name string) string {
			switch name {
			case "XDG_SESSION_TYPE":
				return "x11"
			case "HOME":
				return "/home/test"
			default:
				return ""
			}
		},
		client: xinput.Client{Command: "xinput", Runner: runner},
	}
}

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "schema": "io.github.mr-tao.libinput-curve/v1",
  "profiles": {
    "motion": {
      "motion": {"step": 0.5, "points": [0, 1, 3]}
    }
  },
  "devices": [{
    "id": "sculpt",
    "match": {
      "product_contains": "Example Mouse",
      "vendor_id": "045e",
      "product_id": "07a5"
    },
    "profile": "motion"
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type stateRunner struct {
	properties map[string][]string
}

func newStateRunner() *stateRunner {
	return &stateRunner{properties: map[string][]string{
		"Device Product ID":                   {"1118", "1957"},
		"libinput Accel Profiles Available":   {"1", "1", "1"},
		"libinput Accel Profile Enabled":      {"0", "0", "1"},
		"libinput Accel Custom Motion Points": {"0", "1", "3"},
		"libinput Accel Custom Motion Step":   {"0.5"},
	}}
}

func (r *stateRunner) Run(
	_ context.Context,
	_ string,
	args ...string,
) ([]byte, error) {
	if reflect.DeepEqual(args, []string{"list", "--short"}) {
		return []byte("Example Mouse id=30\n"), nil
	}
	if reflect.DeepEqual(args, []string{"list-props", "30"}) {
		var output strings.Builder
		output.WriteString("Device 'Example Mouse':\n")
		order := []string{
			"Device Product ID",
			"libinput Accel Profiles Available",
			"libinput Accel Profile Enabled",
			"libinput Accel Custom Motion Points",
			"libinput Accel Custom Motion Step",
		}
		for index, name := range order {
			fmt.Fprintf(
				&output,
				"\t%s (%d):\t%s\n",
				name,
				300+index,
				strings.Join(r.properties[name], ", "),
			)
		}
		return []byte(output.String()), nil
	}
	if len(args) >= 4 && args[0] == "set-prop" && args[1] == "30" {
		r.properties[args[2]] = append([]string(nil), args[3:]...)
		return nil, nil
	}
	return []byte("unexpected command"), fmt.Errorf("unexpected args: %v", args)
}
