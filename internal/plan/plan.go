// SPDX-License-Identifier: Apache-2.0 OR MIT

package plan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr-Tao/libinput-curve/internal/config"
	"github.com/Mr-Tao/libinput-curve/internal/xinput"
)

const (
	propertyProfilesAvailable = "libinput Accel Profiles Available"
	propertyProfileEnabled    = "libinput Accel Profile Enabled"
)

type Operation struct {
	DeviceID   int      `json:"device_id"`
	DeviceName string   `json:"device_name"`
	RuleID     string   `json:"rule_id"`
	Property   string   `json:"property"`
	Current    []string `json:"current"`
	Desired    []string `json:"desired"`
}

type Device struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	HardwareID string      `json:"hardware_id,omitempty"`
	RuleID     string      `json:"rule_id"`
	Profile    string      `json:"profile"`
	InSync     bool        `json:"in_sync"`
	Operations []Operation `json:"operations"`
	Errors     []string    `json:"errors"`
}

type Plan struct {
	Backend        string   `json:"backend"`
	Devices        []Device `json:"devices"`
	UnmatchedRules []string `json:"unmatched_rules"`
}

func Build(
	cfg *config.Config,
	devices []xinput.Device,
	overrides config.ScaleOverrides,
) (Plan, error) {
	if cfg == nil {
		return Plan{}, errors.New("plan: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return Plan{}, err
	}

	result := Plan{Backend: "xinput"}
	matchedRules := make(map[string]bool)
	for _, inputDevice := range devices {
		hardwareID := deviceHardwareID(inputDevice)
		rules := cfg.MatchingRules(inputDevice.Name, hardwareID)
		if len(rules) == 0 {
			continue
		}

		devicePlan := Device{
			ID:         inputDevice.ID,
			Name:       inputDevice.Name,
			HardwareID: hardwareID.String(),
			InSync:     false,
			Operations: []Operation{},
			Errors:     []string{},
		}
		if len(rules) > 1 {
			ids := make([]string, len(rules))
			for index, rule := range rules {
				ids[index] = rule.ID
				matchedRules[rule.ID] = true
			}
			devicePlan.Errors = append(devicePlan.Errors,
				"device matches multiple rules: "+strings.Join(ids, ", "))
			result.Devices = append(result.Devices, devicePlan)
			continue
		}

		rule := rules[0]
		matchedRules[rule.ID] = true
		devicePlan.RuleID = rule.ID
		devicePlan.Profile = rule.Profile
		profile, err := cfg.ProfileScaled(rule, overrides)
		if err != nil {
			devicePlan.Errors = append(devicePlan.Errors, err.Error())
			result.Devices = append(result.Devices, devicePlan)
			continue
		}
		buildDevicePlan(inputDevice, rule, profile, &devicePlan)
		devicePlan.InSync = len(devicePlan.Errors) == 0 && len(devicePlan.Operations) == 0
		result.Devices = append(result.Devices, devicePlan)
	}

	for _, rule := range cfg.Devices {
		if !matchedRules[rule.ID] {
			result.UnmatchedRules = append(result.UnmatchedRules, rule.ID)
		}
	}
	sort.Strings(result.UnmatchedRules)
	sort.Slice(result.Devices, func(i, j int) bool {
		return result.Devices[i].ID < result.Devices[j].ID
	})
	return result, nil
}

func buildDevicePlan(
	inputDevice xinput.Device,
	rule config.DeviceRule,
	profile config.Profile,
	result *Device,
) {
	available, ok := inputDevice.Property(propertyProfilesAvailable)
	if !ok {
		result.Errors = append(result.Errors, "custom acceleration profiles are unavailable")
		return
	}
	availableValues, err := available.Integers()
	if err != nil || len(availableValues) < 3 || availableValues[2] != 1 {
		result.Errors = append(result.Errors, "the custom acceleration profile is not supported")
		return
	}

	curves := []struct {
		name   string
		curve  *config.Curve
		points string
		step   string
	}{
		{
			"fallback",
			profile.Fallback,
			"libinput Accel Custom Fallback Points",
			"libinput Accel Custom Fallback Step",
		},
		{
			"motion",
			profile.Motion,
			"libinput Accel Custom Motion Points",
			"libinput Accel Custom Motion Step",
		},
		{
			"scroll",
			profile.Scroll,
			"libinput Accel Custom Scroll Points",
			"libinput Accel Custom Scroll Step",
		},
	}
	for _, candidate := range curves {
		if candidate.curve == nil {
			continue
		}
		addFloatOperation(
			inputDevice,
			rule.ID,
			candidate.points,
			formatFloats(candidate.curve.Points),
			result,
		)
		addFloatOperation(
			inputDevice,
			rule.ID,
			candidate.step,
			[]string{formatFloat(candidate.curve.Step)},
			result,
		)
	}
	addIntegerOperation(
		inputDevice,
		rule.ID,
		propertyProfileEnabled,
		[]string{"0", "0", "1"},
		result,
	)
}

func addFloatOperation(
	device xinput.Device,
	ruleID string,
	propertyName string,
	desired []string,
	result *Device,
) {
	current, ok := device.Property(propertyName)
	if !ok {
		result.Errors = append(result.Errors, fmt.Sprintf("required property %q is unavailable", propertyName))
		return
	}
	if floatValuesEqual(current.Values, desired) {
		return
	}
	result.Operations = append(result.Operations, Operation{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		RuleID:     ruleID,
		Property:   propertyName,
		Current:    append([]string(nil), current.Values...),
		Desired:    append([]string(nil), desired...),
	})
}

func addIntegerOperation(
	device xinput.Device,
	ruleID string,
	propertyName string,
	desired []string,
	result *Device,
) {
	current, ok := device.Property(propertyName)
	if !ok {
		result.Errors = append(result.Errors, fmt.Sprintf("required property %q is unavailable", propertyName))
		return
	}
	if integerValuesEqual(current.Values, desired) {
		return
	}
	result.Operations = append(result.Operations, Operation{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		RuleID:     ruleID,
		Property:   propertyName,
		Current:    append([]string(nil), current.Values...),
		Desired:    append([]string(nil), desired...),
	})
}

func (p Plan) HasErrors() bool {
	for _, device := range p.Devices {
		if len(device.Errors) > 0 {
			return true
		}
	}
	return false
}

func (p Plan) OperationCount() int {
	total := 0
	for _, device := range p.Devices {
		total += len(device.Operations)
	}
	return total
}

type PropertySetter interface {
	SetProperty(context.Context, int, string, []string) error
}

func Apply(ctx context.Context, setter PropertySetter, planned Plan) error {
	if planned.HasErrors() {
		return errors.New("plan contains device errors; no properties were changed")
	}
	for _, device := range planned.Devices {
		for _, operation := range device.Operations {
			if err := setter.SetProperty(
				ctx,
				operation.DeviceID,
				operation.Property,
				operation.Desired,
			); err != nil {
				return fmt.Errorf(
					"apply device %d property %q: %w",
					operation.DeviceID,
					operation.Property,
					err,
				)
			}
		}
	}
	return nil
}

func deviceHardwareID(device xinput.Device) config.HardwareID {
	vendor, product, ok := device.ProductID()
	if !ok {
		return config.HardwareID{}
	}
	return config.HardwareID{
		VendorID:  fmt.Sprintf("%04x", vendor),
		ProductID: fmt.Sprintf("%04x", product),
	}
}

func formatFloats(values []float64) []string {
	formatted := make([]string, len(values))
	for index, value := range values {
		formatted[index] = formatFloat(value)
	}
	return formatted
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func floatValuesEqual(current, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	for index := range current {
		currentValue, currentErr := strconv.ParseFloat(current[index], 64)
		desiredValue, desiredErr := strconv.ParseFloat(desired[index], 64)
		if currentErr != nil || desiredErr != nil ||
			math.Abs(currentValue-desiredValue) > 0.000002 {
			return false
		}
	}
	return true
}

func integerValuesEqual(current, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	for index := range current {
		currentValue, currentErr := strconv.ParseInt(current[index], 10, 64)
		desiredValue, desiredErr := strconv.ParseInt(desired[index], 10, 64)
		if currentErr != nil || desiredErr != nil || currentValue != desiredValue {
			return false
		}
	}
	return true
}
