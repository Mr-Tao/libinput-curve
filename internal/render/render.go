// SPDX-License-Identifier: Apache-2.0 OR MIT

package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Mr-Tao/libinput-curve/internal/plan"
)

type DeviceSummary struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	HardwareID      string `json:"hardware_id,omitempty"`
	CustomAvailable bool   `json:"custom_available"`
	CustomEnabled   bool   `json:"custom_enabled"`
}

func Plan(writer io.Writer, planned plan.Plan, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(planned)
	case "human":
		return renderHumanPlan(writer, planned)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func Devices(writer io.Writer, devices []DeviceSummary, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(devices)
	case "human":
		if len(devices) == 0 {
			_, err := fmt.Fprintln(writer, "No configurable libinput pointer devices found.")
			return err
		}
		for _, device := range devices {
			hardware := device.HardwareID
			if hardware == "" {
				hardware = "unknown"
			}
			fmt.Fprintf(
				writer,
				"id=%d hardware=%s custom_available=%t custom_enabled=%t name=%s\n",
				device.ID,
				hardware,
				device.CustomAvailable,
				device.CustomEnabled,
				device.Name,
			)
		}
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func renderHumanPlan(writer io.Writer, planned plan.Plan) error {
	fmt.Fprintf(writer, "backend: %s\n", planned.Backend)
	if len(planned.Devices) == 0 {
		fmt.Fprintln(writer, "matched devices: none")
	}
	for _, device := range planned.Devices {
		state := "drift"
		switch {
		case len(device.Errors) > 0:
			state = "error"
		case device.InSync:
			state = "in-sync"
		}
		hardware := device.HardwareID
		if hardware == "" {
			hardware = "unknown"
		}
		fmt.Fprintf(
			writer,
			"device %d: %s\n  hardware: %s\n  rule: %s profile: %s state: %s\n",
			device.ID,
			device.Name,
			hardware,
			valueOrDash(device.RuleID),
			valueOrDash(device.Profile),
			state,
		)
		for _, problem := range device.Errors {
			fmt.Fprintf(writer, "  error: %s\n", problem)
		}
		for _, operation := range device.Operations {
			fmt.Fprintf(
				writer,
				"  set %s: %s -> %s\n",
				operation.Property,
				summarizeValues(operation.Current),
				summarizeValues(operation.Desired),
			)
		}
	}
	if len(planned.UnmatchedRules) > 0 {
		fmt.Fprintf(
			writer,
			"unmatched rules: %s\n",
			strings.Join(planned.UnmatchedRules, ", "),
		)
	}
	return nil
}

func summarizeValues(values []string) string {
	switch len(values) {
	case 0:
		return "<none>"
	case 1, 2, 3, 4, 5, 6:
		return strings.Join(values, ",")
	default:
		return fmt.Sprintf("<%d values>", len(values))
	}
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
