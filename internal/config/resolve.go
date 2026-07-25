// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// NormalizeHardwareID validates and normalizes vendor and product IDs.
func NormalizeHardwareID(vendorID, productID string) (HardwareID, error) {
	if !hardwareIDPartPattern.MatchString(vendorID) {
		return HardwareID{}, fmt.Errorf("vendor ID must be exactly four hex digits")
	}
	if !hardwareIDPartPattern.MatchString(productID) {
		return HardwareID{}, fmt.Errorf("product ID must be exactly four hex digits")
	}
	return HardwareID{
		VendorID:  strings.ToLower(vendorID),
		ProductID: strings.ToLower(productID),
	}, nil
}

func (m Match) NormalizedHardwareID() string {
	hardwareID, err := NormalizeHardwareID(m.VendorID, m.ProductID)
	if err != nil {
		return ""
	}
	return hardwareID.String()
}

// ResolveRule returns a copy of the first rule whose specified criteria all
// match. A zero HardwareID represents a device without hardware IDs.
func (c *Config) ResolveRule(name string, hardwareID HardwareID) (DeviceRule, bool) {
	rules := c.MatchingRules(name, hardwareID)
	if len(rules) > 0 {
		return rules[0], true
	}
	return DeviceRule{}, false
}

// MatchingRules returns independent copies of every rule whose specified
// criteria match. Callers that mutate devices should reject multiple matches.
func (c *Config) MatchingRules(name string, hardwareID HardwareID) []DeviceRule {
	if c == nil {
		return nil
	}

	normalizedID, hasHardwareID := normalizedHardwareID(hardwareID)
	var matches []DeviceRule
	for _, rule := range c.Devices {
		if ruleMatches(rule.Match, name, normalizedID, hasHardwareID) {
			matches = append(matches, rule)
		}
	}
	return matches
}

func normalizedHardwareID(id HardwareID) (HardwareID, bool) {
	if id.VendorID == "" && id.ProductID == "" {
		return HardwareID{}, false
	}
	normalized, err := NormalizeHardwareID(id.VendorID, id.ProductID)
	if err != nil {
		return HardwareID{}, false
	}
	return normalized, true
}

func ruleMatches(match Match, name string, hardwareID HardwareID, hasHardwareID bool) bool {
	hasCriterion := false
	if match.NameRegex != "" {
		hasCriterion = true
		expression, err := regexp.Compile(match.NameRegex)
		if err != nil || !expression.MatchString(name) {
			return false
		}
	}
	if match.ProductContains != "" {
		hasCriterion = true
		if !strings.Contains(name, match.ProductContains) {
			return false
		}
	}

	if match.VendorID != "" || match.ProductID != "" {
		hasCriterion = true
		if !hasHardwareID {
			return false
		}
		ruleHardwareID, err := NormalizeHardwareID(match.VendorID, match.ProductID)
		if err != nil || ruleHardwareID != hardwareID {
			return false
		}
	}
	return hasCriterion
}

// ProfileScaled resolves rule.Profile and returns a deep copy whose fallback
// and motion points use the motion factor and whose scroll points use the
// scroll factor. Runtime factors are multiplied with rule factors.
func (c *Config) ProfileScaled(rule DeviceRule, overrides ScaleOverrides) (Profile, error) {
	if c == nil {
		return Profile{}, fmt.Errorf("config: nil configuration")
	}

	profile, ok := c.Profiles[rule.Profile]
	if !ok {
		return Profile{}, fmt.Errorf("config: unknown profile %q", rule.Profile)
	}
	if err := validateProfile("profiles["+rule.Profile+"]", profile); err != nil {
		return Profile{}, err
	}
	if err := validatePositiveFinite("rule.motion_scale", rule.MotionScale); err != nil {
		return Profile{}, err
	}
	if err := validatePositiveFinite("rule.scroll_scale", rule.ScrollScale); err != nil {
		return Profile{}, err
	}

	motionFactor, err := combinedScale(
		"motion runtime override",
		rule.MotionScale,
		overrides.Motion,
	)
	if err != nil {
		return Profile{}, err
	}
	scrollFactor, err := combinedScale(
		"scroll runtime override",
		rule.ScrollScale,
		overrides.Scroll,
	)
	if err != nil {
		return Profile{}, err
	}

	fallback, err := curveScaled(profile.Fallback, motionFactor)
	if err != nil {
		return Profile{}, fmt.Errorf("config: scale fallback curve: %w", err)
	}
	motion, err := curveScaled(profile.Motion, motionFactor)
	if err != nil {
		return Profile{}, fmt.Errorf("config: scale motion curve: %w", err)
	}
	scroll, err := curveScaled(profile.Scroll, scrollFactor)
	if err != nil {
		return Profile{}, fmt.Errorf("config: scale scroll curve: %w", err)
	}

	return Profile{
		Fallback: fallback,
		Motion:   motion,
		Scroll:   scroll,
	}, nil
}

func combinedScale(path string, ruleScale float64, override *float64) (float64, error) {
	factor := ruleScale
	if override != nil {
		if err := validatePositiveFinite(path, *override); err != nil {
			return 0, err
		}
		factor *= *override
	}
	if !isFinite(factor) || factor <= 0 {
		return 0, fmt.Errorf("config: combined %s factor must be finite and greater than zero", path)
	}
	return factor, nil
}

func curveScaled(curve *Curve, factor float64) (*Curve, error) {
	if curve == nil {
		return nil, nil
	}

	scaled := &Curve{
		Step:   curve.Step,
		Points: make([]float64, len(curve.Points)),
	}
	for i, point := range curve.Points {
		scaledPoint := point * factor
		if math.IsNaN(scaledPoint) || math.IsInf(scaledPoint, 0) {
			return nil, fmt.Errorf("point %d is not finite after scaling", i)
		}
		if scaledPoint < 0 || scaledPoint > MaxValue {
			return nil, fmt.Errorf(
				"point %d is outside [0, %d] after scaling",
				i,
				MaxValue,
			)
		}
		scaled.Points[i] = scaledPoint
	}
	return scaled, nil
}
