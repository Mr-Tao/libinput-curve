// SPDX-License-Identifier: Apache-2.0 OR MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mr-Tao/libinput-curve/internal/config"
	"github.com/Mr-Tao/libinput-curve/internal/plan"
	"github.com/Mr-Tao/libinput-curve/internal/render"
	"github.com/Mr-Tao/libinput-curve/internal/xinput"
	"github.com/Mr-Tao/libinput-curve/internal/xorg"
)

var version = "0.1.0-dev"

const (
	exitOK          = 0
	exitDrift       = 1
	exitFailure     = 2
	exitUsage       = 64
	exitConfig      = 65
	exitUnavailable = 69
	exitIO          = 74
)

type application struct {
	stdout io.Writer
	stderr io.Writer
	getenv func(string) string
	client xinput.Client
}

type runtimeOptions struct {
	configPath    string
	format        string
	xinputCommand string
	motionScale   float64
	scrollScale   float64
	allowXWayland bool
	watchInterval time.Duration
}

func main() {
	app := application{
		stdout: os.Stdout,
		stderr: os.Stderr,
		getenv: os.Getenv,
		client: xinput.NewClient(),
	}
	os.Exit(app.run(os.Args[1:]))
}

func (app application) run(args []string) int {
	if len(args) == 0 {
		app.usage()
		return exitUsage
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			app.usage()
			return exitUsage
		}
		fmt.Fprintln(app.stdout, version)
		return exitOK
	case "validate":
		return app.runValidate(args[1:])
	case "devices":
		return app.runDevices(args[1:])
	case "plan", "status", "apply":
		return app.runPlanCommand(args[0], args[1:])
	case "watch":
		return app.runWatch(args[1:])
	case "render-xorg":
		return app.runRenderXorg(args[1:])
	case "help", "-h", "--help":
		app.usage()
		return exitOK
	default:
		fmt.Fprintf(app.stderr, "unknown command %q\n", args[0])
		app.usage()
		return exitUsage
	}
}

func (app application) runValidate(args []string) int {
	flags := app.newFlagSet("validate")
	configPath := flags.String("config", app.defaultConfigPath(), "configuration file")
	if !parseFlags(flags, args, app.stderr) {
		return exitUsage
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitConfig
	}
	fmt.Fprintf(
		app.stdout,
		"valid: schema=%s profiles=%d devices=%d\n",
		cfg.Schema,
		len(cfg.Profiles),
		len(cfg.Devices),
	)
	return exitOK
}

func (app application) runDevices(args []string) int {
	flags := app.newFlagSet("devices")
	format := flags.String("format", "human", "human or json")
	command := flags.String("xinput", "xinput", "xinput executable")
	allowXWayland := flags.Bool(
		"allow-xwayland",
		false,
		"inspect Xwayland devices even though compositor state is unaffected",
	)
	if !parseFlags(flags, args, app.stderr) {
		return exitUsage
	}
	if err := validateFormat(*format); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUsage
	}
	if err := app.requireXorg(*allowXWayland); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUnavailable
	}

	client := app.client
	client.Command = *command
	devices, err := client.ListDevices(context.Background())
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUnavailable
	}
	if err := render.Devices(app.stdout, summarizeDevices(devices), *format); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitIO
	}
	return exitOK
}

func (app application) runPlanCommand(command string, args []string) int {
	options, ok := app.parseRuntimeOptions(command, args, false)
	if !ok {
		return exitUsage
	}
	if err := app.requireXorg(options.allowXWayland); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUnavailable
	}
	cfg, err := loadConfig(options.configPath)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitConfig
	}
	overrides, err := scaleOverrides(options)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUsage
	}
	client := app.client
	client.Command = options.xinputCommand
	var lock *os.File
	if command == "apply" {
		lock, err = app.acquireMutationLock()
		if err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitFailure
		}
		defer lock.Close()
	}
	planned, err := collectPlan(context.Background(), &client, cfg, overrides)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUnavailable
	}

	switch command {
	case "plan":
		if err := render.Plan(app.stdout, planned, options.format); err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitIO
		}
		if planned.HasErrors() {
			return exitFailure
		}
		return exitOK
	case "status":
		if err := render.Plan(app.stdout, planned, options.format); err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitIO
		}
		if planned.HasErrors() {
			return exitFailure
		}
		if planned.OperationCount() > 0 || len(planned.UnmatchedRules) > 0 {
			return exitDrift
		}
		return exitOK
	case "apply":
		if planned.HasErrors() {
			_ = render.Plan(app.stdout, planned, options.format)
			return exitFailure
		}
		operationCount := planned.OperationCount()
		if err := plan.Apply(context.Background(), &client, planned); err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitFailure
		}
		verified, err := collectPlan(context.Background(), &client, cfg, overrides)
		if err != nil {
			fmt.Fprintf(app.stderr, "verify applied profile: %v\n", err)
			return exitFailure
		}
		if err := render.Plan(app.stdout, verified, options.format); err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitIO
		}
		if verified.HasErrors() || verified.OperationCount() > 0 {
			fmt.Fprintln(app.stderr, "verification found remaining drift")
			return exitFailure
		}
		fmt.Fprintf(app.stderr, "applied and verified %d property changes\n", operationCount)
		return exitOK
	}
	return exitUsage
}

func (app application) runWatch(args []string) int {
	options, ok := app.parseRuntimeOptions("watch", args, true)
	if !ok {
		return exitUsage
	}
	if options.watchInterval < 250*time.Millisecond {
		fmt.Fprintln(app.stderr, "watch interval must be at least 250ms")
		return exitUsage
	}
	if err := app.requireXorg(options.allowXWayland); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUnavailable
	}
	overrides, err := scaleOverrides(options)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUsage
	}
	client := app.client
	client.Command = options.xinputCommand
	lock, err := app.acquireMutationLock()
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitFailure
	}
	defer lock.Close()
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)
	defer stop()

	var previous []byte
	for {
		cfg, loadErr := loadConfig(options.configPath)
		if loadErr != nil {
			fmt.Fprintln(app.stderr, loadErr)
			return exitConfig
		}
		planned, collectErr := collectPlan(ctx, &client, cfg, overrides)
		if collectErr != nil {
			if errors.Is(collectErr, context.Canceled) {
				return exitOK
			}
			fmt.Fprintln(app.stderr, collectErr)
			return exitUnavailable
		}
		if planned.HasErrors() {
			_ = render.Plan(app.stdout, planned, options.format)
			return exitFailure
		}
		if planned.OperationCount() > 0 {
			if applyErr := plan.Apply(ctx, &client, planned); applyErr != nil {
				fmt.Fprintln(app.stderr, applyErr)
				return exitFailure
			}
			planned, collectErr = collectPlan(ctx, &client, cfg, overrides)
			if collectErr != nil {
				fmt.Fprintln(app.stderr, collectErr)
				return exitFailure
			}
			if planned.HasErrors() || planned.OperationCount() > 0 {
				_ = render.Plan(app.stdout, planned, options.format)
				fmt.Fprintln(app.stderr, "verification found remaining drift")
				return exitFailure
			}
		}

		fingerprint, _ := json.Marshal(planned)
		if !bytes.Equal(fingerprint, previous) {
			if renderErr := render.Plan(app.stdout, planned, options.format); renderErr != nil {
				fmt.Fprintln(app.stderr, renderErr)
				return exitIO
			}
			previous = fingerprint
		}

		timer := time.NewTimer(options.watchInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return exitOK
		case <-timer.C:
		}
	}
}

func (app application) runRenderXorg(args []string) int {
	flags := app.newFlagSet("render-xorg")
	configPath := flags.String("config", app.defaultConfigPath(), "configuration file")
	motionScale := flags.Float64("motion-scale", 1, "runtime motion multiplier")
	scrollScale := flags.Float64("scroll-scale", 1, "runtime scroll multiplier")
	outputPath := flags.String("output", "-", "output path or - for stdout")
	if !parseFlags(flags, args, app.stderr) {
		return exitUsage
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitConfig
	}
	options := runtimeOptions{motionScale: *motionScale, scrollScale: *scrollScale}
	overrides, err := scaleOverrides(options)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitUsage
	}
	content, err := xorg.Render(cfg, overrides)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitConfig
	}
	if *outputPath == "-" {
		if _, err := io.WriteString(app.stdout, content); err != nil {
			fmt.Fprintln(app.stderr, err)
			return exitIO
		}
		return exitOK
	}
	if err := writeAtomic(*outputPath, []byte(content), 0o644); err != nil {
		fmt.Fprintln(app.stderr, err)
		return exitIO
	}
	return exitOK
}

func (app application) parseRuntimeOptions(
	command string,
	args []string,
	watch bool,
) (runtimeOptions, bool) {
	flags := app.newFlagSet(command)
	options := runtimeOptions{}
	flags.StringVar(&options.configPath, "config", app.defaultConfigPath(), "configuration file")
	flags.StringVar(&options.format, "format", "human", "human or json")
	flags.StringVar(&options.xinputCommand, "xinput", "xinput", "xinput executable")
	flags.Float64Var(&options.motionScale, "motion-scale", 1, "runtime motion multiplier")
	flags.Float64Var(&options.scrollScale, "scroll-scale", 1, "runtime scroll multiplier")
	flags.BoolVar(
		&options.allowXWayland,
		"allow-xwayland",
		false,
		"change Xwayland devices even though compositor state is unaffected",
	)
	if watch {
		flags.DurationVar(&options.watchInterval, "interval", 2*time.Second, "poll interval")
	}
	if !parseFlags(flags, args, app.stderr) {
		return runtimeOptions{}, false
	}
	if err := validateFormat(options.format); err != nil {
		fmt.Fprintln(app.stderr, err)
		return runtimeOptions{}, false
	}
	return options, true
}

func (app application) newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	return flags
}

func (app application) defaultConfigPath() string {
	if path := app.getenv("XDG_CONFIG_HOME"); path != "" {
		return filepath.Join(path, "libinput-curve", "config.json")
	}
	if home := app.getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "libinput-curve", "config.json")
	}
	return "config.json"
}

func (app application) requireXorg(allowXWayland bool) error {
	sessionType := strings.ToLower(app.getenv("XDG_SESSION_TYPE"))
	if sessionType == "wayland" && !allowXWayland {
		return errors.New(
			"wayland session detected: xinput only configures Xwayland devices, not the compositor-owned libinput context; use a compositor-native backend or explicitly pass --allow-xwayland",
		)
	}
	return nil
}

func (app application) acquireMutationLock() (*os.File, error) {
	runtimeDirectory := app.getenv("XDG_RUNTIME_DIR")
	lockName := "libinput-curve.lock"
	if runtimeDirectory == "" {
		runtimeDirectory = os.TempDir()
		lockName = fmt.Sprintf("libinput-curve-%d.lock", os.Getuid())
	}
	path := filepath.Join(runtimeDirectory, lockName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open mutation lock %q: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf(
			"another libinput-curve apply/watch process owns %q: %w",
			path,
			err,
		)
	}
	return file, nil
}

func (app application) usage() {
	fmt.Fprintln(app.stderr, `usage: libinput-curve COMMAND [OPTIONS]

Commands:
  validate      Validate a strict JSON configuration
  devices       List Xorg libinput pointer devices
  plan          Show property changes without applying them
  status        Check whether matched devices are in sync
  apply         Apply a preflighted plan and verify the result
  watch         Reapply and verify profiles after XInput hotplug
  render-xorg   Render persistent xorg.conf.d InputClass sections
  version       Print the version

Use "libinput-curve COMMAND -h" for command-specific flags.`)
}

func loadConfig(path string) (*config.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()
	cfg, err := config.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func collectPlan(
	ctx context.Context,
	client *xinput.Client,
	cfg *config.Config,
	overrides config.ScaleOverrides,
) (plan.Plan, error) {
	devices, err := client.ListDevices(ctx)
	if err != nil {
		return plan.Plan{}, err
	}
	return plan.Build(cfg, devices, overrides)
}

func summarizeDevices(devices []xinput.Device) []render.DeviceSummary {
	var summaries []render.DeviceSummary
	for _, device := range devices {
		availableProperty, hasAvailable := device.Property("libinput Accel Profiles Available")
		if !hasAvailable {
			continue
		}
		availableValues, _ := availableProperty.Integers()
		enabledProperty, hasEnabled := device.Property("libinput Accel Profile Enabled")
		enabledValues, _ := enabledProperty.Integers()
		vendor, product, hasProduct := device.ProductID()
		hardwareID := ""
		if hasProduct {
			hardwareID = fmt.Sprintf("%04x:%04x", vendor, product)
		}
		summaries = append(summaries, render.DeviceSummary{
			ID:              device.ID,
			Name:            device.Name,
			HardwareID:      hardwareID,
			CustomAvailable: len(availableValues) >= 3 && availableValues[2] == 1,
			CustomEnabled:   hasEnabled && len(enabledValues) >= 3 && enabledValues[2] == 1,
		})
	}
	return summaries
}

func scaleOverrides(options runtimeOptions) (config.ScaleOverrides, error) {
	for name, value := range map[string]float64{
		"motion scale": options.motionScale,
		"scroll scale": options.scrollScale,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return config.ScaleOverrides{}, fmt.Errorf("%s must be finite and greater than zero", name)
		}
	}
	return config.ScaleOverrides{
		Motion: floatPointer(options.motionScale),
		Scroll: floatPointer(options.scrollScale),
	}, nil
}

func floatPointer(value float64) *float64 {
	return &value
}

func validateFormat(format string) error {
	if format != "human" && format != "json" {
		return fmt.Errorf("format must be human or json, got %q", format)
	}
	return nil
}

func parseFlags(flags *flag.FlagSet, args []string, stderr io.Writer) bool {
	if err := flags.Parse(args); err != nil {
		return false
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return false
	}
	return true
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	file, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tempPath := file.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect temporary output: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary output: %w", err)
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return fmt.Errorf("set output mode: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish output: %w", err)
	}
	remove = false
	return nil
}
