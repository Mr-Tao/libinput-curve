// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type configJSON struct {
	Schema   string             `json:"schema"`
	Profiles map[string]Profile `json:"profiles"`
	Devices  []deviceRuleJSON   `json:"devices"`
}

type deviceRuleJSON struct {
	ID          string   `json:"id"`
	Match       Match    `json:"match"`
	Profile     string   `json:"profile"`
	MotionScale *float64 `json:"motion_scale,omitempty"`
	ScrollScale *float64 `json:"scroll_scale,omitempty"`
}

// Decode reads exactly one JSON value, rejects unknown fields, applies v1
// defaults, and validates the resulting configuration.
func Decode(r io.Reader) (*Config, error) {
	if r == nil {
		return nil, errors.New("config: nil reader")
	}

	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var raw *configJSON
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("config: decode JSON: %w", err)
	}
	if raw == nil {
		return nil, errors.New("config: top-level JSON value must be an object")
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("config: trailing JSON value")
		}
		return nil, fmt.Errorf("config: trailing JSON: %w", err)
	}

	cfg := &Config{
		Schema:   raw.Schema,
		Profiles: raw.Profiles,
		Devices:  make([]DeviceRule, len(raw.Devices)),
	}
	for i, rawRule := range raw.Devices {
		motionScale := 1.0
		if rawRule.MotionScale != nil {
			motionScale = *rawRule.MotionScale
		}
		scrollScale := 1.0
		if rawRule.ScrollScale != nil {
			scrollScale = *rawRule.ScrollScale
		}

		cfg.Devices[i] = DeviceRule{
			ID: rawRule.ID,
			Match: Match{
				NameRegex:       rawRule.Match.NameRegex,
				ProductContains: rawRule.Match.ProductContains,
				VendorID:        strings.ToLower(rawRule.Match.VendorID),
				ProductID:       strings.ToLower(rawRule.Match.ProductID),
			},
			Profile:     rawRule.Profile,
			MotionScale: motionScale,
			ScrollScale: scrollScale,
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
