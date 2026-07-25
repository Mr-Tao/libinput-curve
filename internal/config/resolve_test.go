// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeHardwareID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		vendorID  string
		productID string
		want      HardwareID
		wantError bool
	}{
		{
			name:      "lowercase",
			vendorID:  "046d",
			productID: "c539",
			want:      HardwareID{VendorID: "046d", ProductID: "c539"},
		},
		{
			name:      "uppercase",
			vendorID:  "ABCD",
			productID: "00EF",
			want:      HardwareID{VendorID: "abcd", ProductID: "00ef"},
		},
		{name: "short vendor", vendorID: "123", productID: "5678", wantError: true},
		{name: "prefixed vendor", vendorID: "0x12", productID: "5678", wantError: true},
		{name: "non-hex product", vendorID: "1234", productID: "zzzz", wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeHardwareID(test.vendorID, test.productID)
			if test.wantError {
				if err == nil {
					t.Fatalf("NormalizeHardwareID() = %#v, nil; want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeHardwareID() error = %v", err)
			}
			if got != test.want {
				t.Errorf("NormalizeHardwareID() = %#v, want %#v", got, test.want)
			}
			if got.String() != test.want.VendorID+":"+test.want.ProductID {
				t.Errorf("HardwareID.String() = %q", got.String())
			}
		})
	}

	if got := (HardwareID{}).String(); got != "" {
		t.Errorf("zero HardwareID.String() = %q, want empty string", got)
	}
}

func TestResolveRule(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Devices: []DeviceRule{
			{
				ID:      "combined",
				Match:   Match{NameRegex: `^Exact Mouse$`, VendorID: "ABCD", ProductID: "00EF"},
				Profile: "combined-profile",
			},
			{
				ID:      "hardware",
				Match:   Match{VendorID: "abcd", ProductID: "00ef"},
				Profile: "hardware-profile",
			},
			{
				ID:      "name",
				Match:   Match{NameRegex: `^Named Device$`},
				Profile: "name-profile",
			},
		},
	}

	tests := []struct {
		name       string
		deviceName string
		hardwareID HardwareID
		wantID     string
		wantMatch  bool
	}{
		{
			name:       "first combined rule wins",
			deviceName: "Exact Mouse",
			hardwareID: HardwareID{VendorID: "ABCD", ProductID: "00EF"},
			wantID:     "combined",
			wantMatch:  true,
		},
		{
			name:       "hardware only",
			deviceName: "Other",
			hardwareID: HardwareID{VendorID: "abcd", ProductID: "00ef"},
			wantID:     "hardware",
			wantMatch:  true,
		},
		{
			name:       "name only without hardware",
			deviceName: "Named Device",
			wantID:     "name",
			wantMatch:  true,
		},
		{
			name:       "all combined criteria are required",
			deviceName: "Exact Mouse",
			hardwareID: HardwareID{VendorID: "1111", ProductID: "2222"},
			wantMatch:  false,
		},
		{
			name:       "invalid runtime hardware ID does not match",
			deviceName: "Other",
			hardwareID: HardwareID{VendorID: "xyz", ProductID: "00ef"},
			wantMatch:  false,
		},
		{
			name:       "name rule ignores invalid runtime hardware ID",
			deviceName: "Named Device",
			hardwareID: HardwareID{VendorID: "xyz", ProductID: "00ef"},
			wantID:     "name",
			wantMatch:  true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := cfg.ResolveRule(test.deviceName, test.hardwareID)
			if ok != test.wantMatch {
				t.Fatalf("ResolveRule() matched = %v, want %v; rule = %#v", ok, test.wantMatch, got)
			}
			if ok && got.ID != test.wantID {
				t.Errorf("ResolveRule().ID = %q, want %q", got.ID, test.wantID)
			}
		})
	}
}

func TestResolveRuleHandlesNilAndInvalidRules(t *testing.T) {
	t.Parallel()

	var nilConfig *Config
	if _, ok := nilConfig.ResolveRule("anything", HardwareID{}); ok {
		t.Fatal("nil Config.ResolveRule() matched, want no match")
	}

	cfg := &Config{
		Devices: []DeviceRule{
			{ID: "empty", Match: Match{}},
			{ID: "regex", Match: Match{NameRegex: `(`}},
			{ID: "hardware", Match: Match{VendorID: "bad", ProductID: "0001"}},
		},
	}
	if _, ok := cfg.ResolveRule("anything", HardwareID{VendorID: "0001", ProductID: "0001"}); ok {
		t.Fatal("ResolveRule() matched invalid rule, want no match")
	}
}

func TestProfileScaledReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Profiles: map[string]Profile{
			"all": {
				Fallback: &Curve{Step: 0.25, Points: []float64{0, 1, 2}},
				Motion:   &Curve{Step: 0.5, Points: []float64{0, 2, 4}},
				Scroll:   &Curve{Step: 1, Points: []float64{0, 3, 6}},
			},
		},
	}
	rule := DeviceRule{Profile: "all", MotionScale: 2, ScrollScale: 3}
	overrides := ScaleOverrides{
		Motion: floatPointer(0.5),
		Scroll: floatPointer(2),
	}
	before := deepCopyProfile(cfg.Profiles["all"])

	got, err := cfg.ProfileScaled(rule, overrides)
	if err != nil {
		t.Fatalf("ProfileScaled() error = %v", err)
	}

	want := Profile{
		Fallback: &Curve{Step: 0.25, Points: []float64{0, 1, 2}},
		Motion:   &Curve{Step: 0.5, Points: []float64{0, 2, 4}},
		Scroll:   &Curve{Step: 1, Points: []float64{0, 18, 36}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProfileScaled() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(cfg.Profiles["all"], before) {
		t.Fatal("ProfileScaled() mutated source profile")
	}

	got.Fallback.Points[1] = 999
	got.Motion.Step = 999
	got.Scroll.Points[1] = 999
	if !reflect.DeepEqual(cfg.Profiles["all"], before) {
		t.Fatal("mutating returned profile mutated source profile")
	}
}

func TestProfileScaledPreservesAbsentCurves(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	got, err := cfg.ProfileScaled(cfg.Devices[0], ScaleOverrides{})
	if err != nil {
		t.Fatalf("ProfileScaled() error = %v", err)
	}
	if got.Fallback == nil {
		t.Fatal("ProfileScaled().Fallback = nil, want curve")
	}
	if got.Motion != nil || got.Scroll != nil {
		t.Fatalf("ProfileScaled() created absent curves: %#v", got)
	}
	if got.Fallback == cfg.Profiles["default"].Fallback {
		t.Fatal("ProfileScaled().Fallback aliases source curve")
	}
}

func TestProfileScaledRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func() (*Config, DeviceRule, ScaleOverrides)
		wantError string
	}{
		{
			name: "nil config",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				return nil, DeviceRule{}, ScaleOverrides{}
			},
			wantError: "nil configuration",
		},
		{
			name: "unknown profile",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				return validConfig(), DeviceRule{
					Profile:     "missing",
					MotionScale: 1,
					ScrollScale: 1,
				}, ScaleOverrides{}
			},
			wantError: "unknown profile",
		},
		{
			name: "invalid source profile",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				cfg.Profiles["default"].Fallback.Step = 0
				return cfg, cfg.Devices[0], ScaleOverrides{}
			},
			wantError: ".step must be finite",
		},
		{
			name: "zero rule motion scale",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				rule := cfg.Devices[0]
				rule.MotionScale = 0
				return cfg, rule, ScaleOverrides{}
			},
			wantError: "rule.motion_scale",
		},
		{
			name: "infinite rule scroll scale",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				rule := cfg.Devices[0]
				rule.ScrollScale = math.Inf(1)
				return cfg, rule, ScaleOverrides{}
			},
			wantError: "rule.scroll_scale",
		},
		{
			name: "zero motion override",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				return cfg, cfg.Devices[0], ScaleOverrides{Motion: floatPointer(0)}
			},
			wantError: "motion runtime override",
		},
		{
			name: "NaN scroll override",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				return cfg, cfg.Devices[0], ScaleOverrides{Scroll: floatPointer(math.NaN())}
			},
			wantError: "scroll runtime override",
		},
		{
			name: "combined factor overflow",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				rule := cfg.Devices[0]
				rule.MotionScale = math.MaxFloat64
				return cfg, rule, ScaleOverrides{Motion: floatPointer(2)}
			},
			wantError: "combined motion runtime override factor",
		},
		{
			name: "scaled point overflow",
			configure: func() (*Config, DeviceRule, ScaleOverrides) {
				cfg := validConfig()
				cfg.Profiles["default"].Fallback.Points[1] = 6000
				rule := cfg.Devices[0]
				rule.MotionScale = 2
				return cfg, rule, ScaleOverrides{}
			},
			wantError: "outside [0, 10000] after scaling",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, rule, overrides := test.configure()
			_, err := cfg.ProfileScaled(rule, overrides)
			if err == nil {
				t.Fatal("ProfileScaled() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ProfileScaled() error = %q, want substring %q", err, test.wantError)
			}
		})
	}
}

func deepCopyProfile(profile Profile) Profile {
	copyCurve := func(curve *Curve) *Curve {
		if curve == nil {
			return nil
		}
		return &Curve{
			Step:   curve.Step,
			Points: append([]float64(nil), curve.Points...),
		}
	}
	return Profile{
		Fallback: copyCurve(profile.Fallback),
		Motion:   copyCurve(profile.Motion),
		Scroll:   copyCurve(profile.Scroll),
	}
}
