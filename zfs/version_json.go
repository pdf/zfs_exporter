package zfs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

type ZFSCommandOutputVersionT struct {
	Command string `json:"command"`
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
}

func (o ZFSCommandOutputVersionT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("command", o.Command),
		slog.Int("major", o.Major),
		slog.Int("minor", o.Minor),
	)
}

type ZFSVersionT struct {
	Userland string `json:"userland"`
	Kernel   string `json:"kernel"`
}

type ZFSVersionOutputT struct {
	OutputVersion ZFSCommandOutputVersionT `json:"output_version"`
	ZFSVersion    ZFSVersionT              `json:"zfs_version"`
}

func (o ZFSVersionOutputT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("output_version.command", o.OutputVersion.Command),
		slog.Int("output_version.major", o.OutputVersion.Major),
		slog.Int("output_version.minor", o.OutputVersion.Minor),
		slog.String("zfs_version.userland", o.ZFSVersion.Userland),
		slog.String("zfs_version.kernel", o.ZFSVersion.Kernel),
	)
}

func GetZFSVersionViaJSON(logger *slog.Logger) (*string, error) {
	cmd := exec.Command(`zfs`, `version`, `--json`)

	// Setup pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// command begin
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command '%s': %w", cmd.String(), err)
	}

	// stdout
	stdo, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read output of '%s'; output: (%w)", cmd.String(), err)
	}
	logger.Debug("ZFS Command Output", "stdout", stdo)

	// stderr
	stde, _ := io.ReadAll(stderr)
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute command '%s'; output: '%s' (%w)", cmd.String(), strings.TrimSpace(string(stde)), err)
	}

	// unmarshal JSON into Go objects
	var o ZFSVersionOutputT
	if err := json.Unmarshal(stdo, &o); err != nil {
		return nil, fmt.Errorf("failed to read output of '%s'; output: (%w)", cmd.String(), err)
	}
	logger.Debug("ZFS Command Output Parsed", "output", o)
	return &o.ZFSVersion.Userland, nil
}
