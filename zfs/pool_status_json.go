package zfs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

type VdevStatusT struct {
	Name           string `json:"name"`
	VdevType       string `json:"vdev_type"`
	Guid           uint64 `json:"guid"`
	Path           string `json:"path"`
	PhysPath       string `json:"phys_path"`
	Devid          string `json:"devid"`
	Class          string `json:"class"`
	State          string `json:"state"`
	Parent         string `json:"parent"`
	RepDevSize     int    `json:"rep_dev_size"`
	SelfHealed     int    `json:"self_healed,omitempty"`
	PhysSpace      int    `json:"phys_space"`
	ReadErrors     int    `json:"read_errors"`
	WriteErrors    int    `json:"write_errors"`
	ChecksumErrors int    `json:"checksum_errors"`
	ScanProcessed  int    `json:"scan_processed,omitempty"`
	SlowIos        int    `json:"slow_ios"`
}

func (o VdevStatusT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", o.Name),
		slog.String("vdev_type", o.VdevType),
		slog.Uint64("guid", o.Guid),
		slog.String("path", o.Path),
		slog.String("phys_path", o.PhysPath),
		slog.String("devid", o.Devid),
		slog.String("class", o.Class),
		slog.String("state", o.State),
		slog.String("parent", o.Parent),
		slog.Int("rep_dev_size", o.RepDevSize),
		slog.Int("self_healed", o.SelfHealed),
		slog.Int("phys_space", o.PhysSpace),
		slog.Int("read_errors", o.ReadErrors),
		slog.Int("write_errors", o.WriteErrors),
		slog.Int("checksum_errors", o.ChecksumErrors),
		slog.Int("scan_processed", o.ScanProcessed),
		slog.Int("slow_ios", o.SlowIos),
	)
}

type ScanStatsT struct {
	Function           string `json:"function"`
	State              string `json:"state"`
	StartTime          int    `json:"start_time"`
	EndTime            int    `json:"end_time"`
	ToExamine          int    `json:"to_examine"`
	Examined           int    `json:"examined"`
	Skipped            int    `json:"skipped"`
	Processed          int    `json:"processed"`
	Errors             int    `json:"errors"`
	BytesPerScan       int    `json:"bytes_per_scan"`
	PassStart          int    `json:"pass_start"`
	ScrubPause         int    `json:"scrub_pause"`
	ScrubSpentPaused   int    `json:"scrub_spent_paused"`
	IssuedBytesPerScan int    `json:"issued_bytes_per_scan"`
	Issued             int    `json:"issued"`
}

func (o ScanStatsT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("function", o.Function),
		slog.String("state", o.State),
		slog.Int("start_time", o.StartTime),
		slog.Int("end_time", o.EndTime),
		slog.Int("to_examine", o.ToExamine),
		slog.Int("examined", o.Examined),
		slog.Int("skipped", o.Skipped),
		slog.Int("processed", o.Processed),
		slog.Int("errors", o.Errors),
		slog.Int("BytesPerScan", o.BytesPerScan),
		slog.Int("pass_start", o.PassStart),
		slog.Int("scrub_pause", o.ScrubPause),
		slog.Int("scrub_spent_paused", o.ScrubSpentPaused),
		slog.Int("issued_bytes_per_scan", o.IssuedBytesPerScan),
	)
}

type PoolStatusT struct {
	Name       string                 `json:"name"`
	State      string                 `json:"state"`
	PoolGuid   uint64                 `json:"pool_guid"`
	Txg        int                    `json:"txg"`
	SpaVersion int                    `json:"spa_version"`
	ZplVersion int                    `json:"zpl_version"`
	Status     string                 `json:"status"`
	Action     string                 `json:"action"`
	Moreinfo   string                 `json:"moreinfo"`
	ErrorCount int                    `json:"error_count"`
	ScanStats  ScanStatsT             `json:"scan_stats"`
	Vdevs      map[string]VdevStatusT `json:"vdevs"`
}

func (o PoolStatusT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", o.Name),
		slog.String("state", o.State),
		slog.Uint64("pool_guid", o.PoolGuid),
		slog.Int("txg", o.Txg),
		slog.Int("spa_version", o.SpaVersion),
		slog.Int("zpl_version", o.ZplVersion),
		slog.String("status", o.Status),
		slog.String("action", o.Action),
		slog.String("more_info", o.Moreinfo),
		slog.Int("error_count", o.ErrorCount),
		slog.Int("num_vdevs", len(o.Vdevs)),
	)
}

type ZpoolStatusOutputT struct {
	OutputVersion ZFSCommandOutputVersionT `json:"output_version"`
	Pools         map[string]PoolStatusT   `json:"pools"`
}

func (o ZpoolStatusOutputT) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("output_version.command", o.OutputVersion.Command),
		slog.Int("output_version.major", o.OutputVersion.Major),
		slog.Int("output_version.minor", o.OutputVersion.Minor),
		slog.Int("num_pools", len(o.Pools)),
	)
}

func ZpoolStatusViaJSON(logger *slog.Logger) (*map[string]PoolStatusT, error) {
	cmd := exec.Command(`zpool`, `status`, `--json`, `--json-int`)

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
	var o ZpoolStatusOutputT
	if err := json.Unmarshal(stdo, &o); err != nil {
		return nil, fmt.Errorf("failed to read output of '%s'; output: (%w)", cmd.String(), err)
	}
	logger.Debug("Zpool Status Output Parsed", "output", o)
	return &o.Pools, nil
}
