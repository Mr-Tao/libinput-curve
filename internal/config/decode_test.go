// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"strings"
	"testing"
)

const validJSON = `{
	"schema": "io.github.mr-tao.libinput-curve/v1",
	"profiles": {
		"default": {
			"fallback": {"step": 0.5, "points": [0, 1, 2]}
		}
	},
	"devices": [{
		"id": "mouse",
		"match": {
			"name_regex": "^Example Mouse$",
			"vendor_id": "ABCD",
			"product_id": "00EF"
		},
		"profile": "default"
	}]
}`

func TestDecodeAppliesDefaultsAndNormalization(t *testing.T) {
	t.Parallel()

	cfg, err := Decode(strings.NewReader(validJSON))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if cfg.Schema != SchemaV1 {
		t.Errorf("Schema = %q, want %q", cfg.Schema, SchemaV1)
	}
	if got := cfg.Devices[0].MotionScale; got != 1 {
		t.Errorf("MotionScale = %v, want 1", got)
	}
	if got := cfg.Devices[0].ScrollScale; got != 1 {
		t.Errorf("ScrollScale = %v, want 1", got)
	}
	if got := cfg.Devices[0].Match.VendorID; got != "abcd" {
		t.Errorf("VendorID = %q, want %q", got, "abcd")
	}
	if got := cfg.Devices[0].Match.ProductID; got != "00ef" {
		t.Errorf("ProductID = %q, want %q", got, "00ef")
	}
}

func TestDecodePreservesExplicitScales(t *testing.T) {
	t.Parallel()

	input := strings.Replace(
		validJSON,
		`"profile": "default"`,
		`"profile": "default", "motion_scale": 1.25, "scroll_scale": 0.75`,
		1,
	)
	cfg, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got := cfg.Devices[0].MotionScale; got != 1.25 {
		t.Errorf("MotionScale = %v, want 1.25", got)
	}
	if got := cfg.Devices[0].ScrollScale; got != 0.75 {
		t.Errorf("ScrollScale = %v, want 0.75", got)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		oldFragment string
		newFragment string
	}{
		{
			name:        "root",
			oldFragment: `"schema":`,
			newFragment: `"unknown": true, "schema":`,
		},
		{
			name:        "profile",
			oldFragment: `"fallback":`,
			newFragment: `"unknown": true, "fallback":`,
		},
		{
			name:        "curve",
			oldFragment: `"step":`,
			newFragment: `"unknown": true, "step":`,
		},
		{
			name:        "device",
			oldFragment: `"id": "mouse",`,
			newFragment: `"id": "mouse", "unknown": true,`,
		},
		{
			name:        "match",
			oldFragment: `"name_regex":`,
			newFragment: `"unknown": true, "name_regex":`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := strings.Replace(
				validJSON,
				test.oldFragment,
				test.newFragment,
				1,
			)
			if _, err := Decode(strings.NewReader(input)); err == nil {
				t.Fatal("Decode() error = nil, want unknown-field error")
			} else if !strings.Contains(err.Error(), `unknown field "unknown"`) {
				t.Fatalf("Decode() error = %q, want unknown-field error", err)
			}
		})
	}
}

func TestDecodeRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		trailing string
	}{
		{name: "object", trailing: `{}`},
		{name: "scalar", trailing: `true`},
		{name: "malformed", trailing: `garbage`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Decode(strings.NewReader(validJSON + test.trailing)); err == nil {
				t.Fatal("Decode() error = nil, want trailing-JSON error")
			} else if !strings.Contains(err.Error(), "trailing JSON") {
				t.Fatalf("Decode() error = %q, want trailing-JSON error", err)
			}
		})
	}
}

func TestDecodeRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ``},
		{name: "null", input: `null`},
		{name: "array", input: `[]`},
		{name: "malformed", input: `{`},
		{
			name: "explicit zero scale",
			input: strings.Replace(
				validJSON,
				`"profile": "default"`,
				`"profile": "default", "motion_scale": 0`,
				1,
			),
		},
		{
			name: "wrong schema",
			input: strings.Replace(
				validJSON,
				SchemaV1,
				"io.github.mr-tao.libinput-curve/v2",
				1,
			),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Decode(strings.NewReader(test.input)); err == nil {
				t.Fatal("Decode() error = nil, want error")
			}
		})
	}
}

func TestDecodeRejectsNilReader(t *testing.T) {
	t.Parallel()

	if _, err := Decode(nil); err == nil {
		t.Fatal("Decode(nil) error = nil, want error")
	}
}
