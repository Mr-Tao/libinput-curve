// SPDX-License-Identifier: Apache-2.0 OR MIT

package xorg

import (
	"strings"
	"testing"

	"github.com/Mr-Tao/libinput-curve/internal/config"
)

func TestRender(t *testing.T) {
	cfg := renderConfig()
	scale := 2.0
	output, err := Render(cfg, config.ScaleOverrides{Motion: &scale})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`Identifier "libinput-curve-sculpt"`,
		`MatchUSBID "045e:07a5"`,
		`MatchProduct "Transceiver v9.0 Mouse"`,
		`Option "AccelProfile" "custom"`,
		`Option "AccelPointsMotion" "0.000000 2.000000 6.000000"`,
		`Option "AccelStepMotion" "0.500000"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("output does not contain %q:\n%s", expected, output)
		}
	}
}

func TestRenderRejectsUntranslatableAndDuplicateRules(t *testing.T) {
	cfg := renderConfig()
	cfg.Devices[0].Match.NameRegex = "Mouse$"
	if _, err := Render(cfg, config.ScaleOverrides{}); err == nil ||
		!strings.Contains(err.Error(), "cannot be translated") {
		t.Fatalf("unexpected regex error: %v", err)
	}

	cfg = renderConfig()
	cfg.Devices = append(cfg.Devices, cfg.Devices[0])
	cfg.Devices[1].ID = "duplicate"
	if _, err := Render(cfg, config.ScaleOverrides{}); err == nil ||
		!strings.Contains(err.Error(), "share MatchUSBID") {
		t.Fatalf("unexpected duplicate error: %v", err)
	}

	cfg = renderConfig()
	cfg.Devices[0].Match.VendorID = ""
	cfg.Devices[0].Match.ProductID = ""
	if _, err := Render(cfg, config.ScaleOverrides{}); err == nil ||
		!strings.Contains(err.Error(), "needs vendor_id") {
		t.Fatalf("unexpected hardware ID error: %v", err)
	}
}

func renderConfig() *config.Config {
	return &config.Config{
		Schema: config.SchemaV1,
		Profiles: map[string]config.Profile{
			"motion": {
				Motion: &config.Curve{Step: 0.5, Points: []float64{0, 1, 3}},
			},
		},
		Devices: []config.DeviceRule{{
			ID: "sculpt",
			Match: config.Match{
				ProductContains: "Transceiver v9.0 Mouse",
				VendorID:        "045e",
				ProductID:       "07a5",
			},
			Profile:     "motion",
			MotionScale: 1,
			ScrollScale: 1,
		}},
	}
}
