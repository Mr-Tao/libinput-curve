// SPDX-License-Identifier: Apache-2.0 OR MIT

package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mr-Tao/libinput-curve/internal/plan"
)

func TestPlanHumanAndJSON(t *testing.T) {
	planned := plan.Plan{
		Backend: "xinput",
		Devices: []plan.Device{{
			ID:         30,
			Name:       "Example Mouse",
			HardwareID: "045e:07a5",
			RuleID:     "mouse",
			Profile:    "main",
			Operations: []plan.Operation{{
				Property: "points",
				Current:  []string{"0", "1"},
				Desired:  []string{"0", "2"},
			}},
		}},
		UnmatchedRules: []string{"away"},
	}
	var output bytes.Buffer
	if err := Plan(&output, planned, "human"); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"device 30: Example Mouse",
		"state: drift",
		"set points: 0,1 -> 0,2",
		"unmatched rules: away",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q:\n%s", expected, output.String())
		}
	}

	output.Reset()
	if err := Plan(&output, planned, "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"hardware_id": "045e:07a5"`) {
		t.Fatalf("unexpected JSON:\n%s", output.String())
	}
}

func TestDevicesAndUnsupportedFormat(t *testing.T) {
	var output bytes.Buffer
	if err := Devices(&output, []DeviceSummary{{
		ID: 1, Name: "Mouse", CustomAvailable: true,
	}}, "human"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "custom_available=true") {
		t.Fatalf("unexpected output: %s", output.String())
	}
	if err := Devices(&output, nil, "yaml"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
