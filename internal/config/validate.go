// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

var (
	hardwareIDPartPattern = regexp.MustCompile(`^[0-9A-Fa-f]{4}$`)
	identifierPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// Validate checks all v1 schema invariants. It does not mutate the
// configuration.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config: nil configuration")
	}
	if c.Schema != SchemaV1 {
		return fmt.Errorf("config: schema must be %q, got %q", SchemaV1, c.Schema)
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("config: profiles must not be empty")
	}

	profileNames := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)

	for _, name := range profileNames {
		if name == "" {
			return fmt.Errorf("config: profile name must not be empty")
		}
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("config: profile name %q is not a safe identifier", name)
		}
		if err := validateProfile("profiles["+name+"]", c.Profiles[name]); err != nil {
			return err
		}
	}

	if len(c.Devices) == 0 {
		return fmt.Errorf("config: devices must not be empty")
	}

	seenIDs := make(map[string]int, len(c.Devices))
	for i, rule := range c.Devices {
		path := fmt.Sprintf("devices[%d]", i)
		if rule.ID == "" {
			return fmt.Errorf("config: %s.id must not be empty", path)
		}
		if !identifierPattern.MatchString(rule.ID) {
			return fmt.Errorf("config: %s.id %q is not a safe identifier", path, rule.ID)
		}
		if previous, ok := seenIDs[rule.ID]; ok {
			return fmt.Errorf(
				"config: %s.id %q duplicates devices[%d].id",
				path,
				rule.ID,
				previous,
			)
		}
		seenIDs[rule.ID] = i

		if _, ok := c.Profiles[rule.Profile]; !ok {
			return fmt.Errorf(
				"config: %s.profile references unknown profile %q",
				path,
				rule.Profile,
			)
		}
		if err := validateMatch(path+".match", rule.Match); err != nil {
			return err
		}
		if err := validatePositiveFinite(path+".motion_scale", rule.MotionScale); err != nil {
			return err
		}
		if err := validatePositiveFinite(path+".scroll_scale", rule.ScrollScale); err != nil {
			return err
		}
	}

	return nil
}

func validateProfile(path string, profile Profile) error {
	if profile.Fallback == nil && profile.Motion == nil && profile.Scroll == nil {
		return fmt.Errorf("config: %s must define at least one curve", path)
	}

	curves := []struct {
		name  string
		curve *Curve
	}{
		{name: "fallback", curve: profile.Fallback},
		{name: "motion", curve: profile.Motion},
		{name: "scroll", curve: profile.Scroll},
	}
	for _, candidate := range curves {
		if candidate.curve == nil {
			continue
		}
		if err := validateCurve(path+"."+candidate.name, candidate.curve); err != nil {
			return err
		}
	}
	return nil
}

func validateCurve(path string, curve *Curve) error {
	if !isFinite(curve.Step) || curve.Step <= 0 || curve.Step > MaxValue {
		return fmt.Errorf(
			"config: %s.step must be finite and greater than zero (maximum %d)",
			path,
			MaxValue,
		)
	}
	if len(curve.Points) < 2 || len(curve.Points) > MaxPoints {
		return fmt.Errorf(
			"config: %s.points must contain at least two values and at most %d values",
			path,
			MaxPoints,
		)
	}
	for i, point := range curve.Points {
		if !isFinite(point) || point < 0 || point > MaxValue {
			return fmt.Errorf(
				"config: %s.points[%d] must be finite and non-negative (maximum %d)",
				path,
				i,
				MaxValue,
			)
		}
	}
	return nil
}

func validateMatch(path string, match Match) error {
	hasName := match.NameRegex != ""
	hasProductSubstring := match.ProductContains != ""
	hasVendor := match.VendorID != ""
	hasProduct := match.ProductID != ""

	if hasVendor != hasProduct {
		return fmt.Errorf(
			"config: %s.vendor_id and %s.product_id must be specified together",
			path,
			path,
		)
	}
	if !hasName && !hasProductSubstring && !hasVendor {
		return fmt.Errorf("config: %s must define at least one criterion", path)
	}
	if hasName {
		if _, err := regexp.Compile(match.NameRegex); err != nil {
			return fmt.Errorf("config: %s.name_regex is invalid: %w", path, err)
		}
	}
	if hasProductSubstring &&
		(strings.ContainsAny(match.ProductContains, "\"\r\n|") ||
			strings.TrimSpace(match.ProductContains) == "") {
		return fmt.Errorf(
			"config: %s.product_contains contains characters unsafe for Xorg matching",
			path,
		)
	}
	if hasVendor {
		if !hardwareIDPartPattern.MatchString(match.VendorID) {
			return fmt.Errorf("config: %s.vendor_id must be exactly four hex digits", path)
		}
		if !hardwareIDPartPattern.MatchString(match.ProductID) {
			return fmt.Errorf("config: %s.product_id must be exactly four hex digits", path)
		}
	}
	return nil
}

func validatePositiveFinite(path string, value float64) error {
	if !isFinite(value) || value <= 0 {
		return fmt.Errorf("config: %s must be finite and greater than zero", path)
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
