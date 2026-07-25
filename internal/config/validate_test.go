// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"math"
	"strings"
	"testing"
)

func TestValidateAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Profiles["default"] = Profile{
		Fallback: &Curve{Step: 0.25, Points: []float64{0, 0.5}},
		Motion:   &Curve{Step: 0.5, Points: []float64{0, 1, 4}},
		Scroll:   &Curve{Step: 1, Points: []float64{0, 2}},
	}
	cfg.Devices[0].Match = Match{
		NameRegex: `(?i)mouse`,
		VendorID:  "ABCD",
		ProductID: "0123",
	}
	cfg.Devices[0].MotionScale = 0.5
	cfg.Devices[0].ScrollScale = 2

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Config)
		wantError string
	}{
		{
			name:      "wrong schema",
			mutate:    func(c *Config) { c.Schema = SchemaV1 + ".invalid" },
			wantError: "schema must be",
		},
		{
			name:      "nil profiles",
			mutate:    func(c *Config) { c.Profiles = nil },
			wantError: "profiles must not be empty",
		},
		{
			name:      "empty profiles",
			mutate:    func(c *Config) { c.Profiles = map[string]Profile{} },
			wantError: "profiles must not be empty",
		},
		{
			name: "empty profile name",
			mutate: func(c *Config) {
				c.Profiles = map[string]Profile{
					"": *profileWithFallback(),
				}
			},
			wantError: "profile name must not be empty",
		},
		{
			name: "profile without curve",
			mutate: func(c *Config) {
				c.Profiles["default"] = Profile{}
			},
			wantError: "must define at least one curve",
		},
		{
			name: "zero step",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Step = 0
			},
			wantError: ".step must be finite and greater than zero",
		},
		{
			name: "negative step",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Step = -1
			},
			wantError: ".step must be finite and greater than zero",
		},
		{
			name: "NaN step",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Step = math.NaN()
			},
			wantError: ".step must be finite and greater than zero",
		},
		{
			name: "infinite step",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Step = math.Inf(1)
			},
			wantError: ".step must be finite and greater than zero",
		},
		{
			name: "too few points",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points = []float64{0}
			},
			wantError: "must contain at least two values",
		},
		{
			name: "negative point",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points[1] = -1
			},
			wantError: "must be finite and non-negative",
		},
		{
			name: "NaN point",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points[1] = math.NaN()
			},
			wantError: "must be finite and non-negative",
		},
		{
			name: "infinite point",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points[1] = math.Inf(-1)
			},
			wantError: "must be finite and non-negative",
		},
		{
			name:      "nil devices",
			mutate:    func(c *Config) { c.Devices = nil },
			wantError: "devices must not be empty",
		},
		{
			name: "empty rule id",
			mutate: func(c *Config) {
				c.Devices[0].ID = ""
			},
			wantError: ".id must not be empty",
		},
		{
			name: "duplicate rule id",
			mutate: func(c *Config) {
				c.Devices = append(c.Devices, c.Devices[0])
			},
			wantError: "duplicates devices[0].id",
		},
		{
			name: "unknown profile",
			mutate: func(c *Config) {
				c.Devices[0].Profile = "missing"
			},
			wantError: "references unknown profile",
		},
		{
			name: "no match criteria",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{}
			},
			wantError: "must define at least one criterion",
		},
		{
			name: "vendor without product",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{VendorID: "abcd"}
			},
			wantError: "must be specified together",
		},
		{
			name: "product without vendor",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{ProductID: "1234"}
			},
			wantError: "must be specified together",
		},
		{
			name: "short vendor",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{VendorID: "abc", ProductID: "1234"}
			},
			wantError: "vendor_id must be exactly four hex digits",
		},
		{
			name: "non-hex vendor",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{VendorID: "xyz!", ProductID: "1234"}
			},
			wantError: "vendor_id must be exactly four hex digits",
		},
		{
			name: "long product",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{VendorID: "abcd", ProductID: "12345"}
			},
			wantError: "product_id must be exactly four hex digits",
		},
		{
			name: "non-hex product",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{VendorID: "abcd", ProductID: "12?4"}
			},
			wantError: "product_id must be exactly four hex digits",
		},
		{
			name: "invalid regex",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{NameRegex: `(`}
			},
			wantError: "name_regex is invalid",
		},
		{
			name: "unsafe product substring",
			mutate: func(c *Config) {
				c.Devices[0].Match = Match{ProductContains: `Mouse"`}
			},
			wantError: "unsafe for Xorg",
		},
		{
			name: "too many points",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points = make([]float64, MaxPoints+1)
			},
			wantError: "at most 64 values",
		},
		{
			name: "point above Xorg maximum",
			mutate: func(c *Config) {
				c.Profiles["default"].Fallback.Points[1] = MaxValue + 1
			},
			wantError: "maximum 10000",
		},
		{
			name: "zero motion scale",
			mutate: func(c *Config) {
				c.Devices[0].MotionScale = 0
			},
			wantError: "motion_scale must be finite and greater than zero",
		},
		{
			name: "negative motion scale",
			mutate: func(c *Config) {
				c.Devices[0].MotionScale = -1
			},
			wantError: "motion_scale must be finite and greater than zero",
		},
		{
			name: "NaN scroll scale",
			mutate: func(c *Config) {
				c.Devices[0].ScrollScale = math.NaN()
			},
			wantError: "scroll_scale must be finite and greater than zero",
		},
		{
			name: "infinite scroll scale",
			mutate: func(c *Config) {
				c.Devices[0].ScrollScale = math.Inf(1)
			},
			wantError: "scroll_scale must be finite and greater than zero",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig()
			test.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() error = %q, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestValidateRejectsNilConfig(t *testing.T) {
	t.Parallel()

	var cfg *Config
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func profileWithFallback() *Profile {
	return &Profile{
		Fallback: &Curve{
			Step:   1,
			Points: []float64{0, 1},
		},
	}
}
