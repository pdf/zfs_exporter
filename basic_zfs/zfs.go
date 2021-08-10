package basic_zfs

import (
	"os/exec"
	"strings"
)

const (
	DatasetFilesystem = "filesystem"
	DatasetSnapshot   = "snapshot"
	DatasetVolume     = "volume"
)

func DatasetProperties(pool string, dsType string, dsProperties []string) ([][]string, error) {
	joinedDsProps := strings.Join(dsProperties, ",")
	outBytes, err := exec.Command(
		"zfs", "list", "-Hprt", dsType, "-o", joinedDsProps, pool,
	).Output()
	if err != nil {
		return nil, err
	}

	output := string(outBytes)
	lines := strings.Split(output, "\n")
	results := make([][]string, 0, len(lines))
	for _, line := range lines {
		// NOTE: last line is empty
		if len(line) > 0 {
			results = append(results, strings.Fields(line))
		}
	}
	return results, nil
}

func FilesystemProperties(pool string, properties []string) ([][]string, error) {
	return DatasetProperties(pool, DatasetFilesystem, properties)
}

func SnapshotProperties(pool string, properties []string) ([][]string, error) {
	return DatasetProperties(pool, DatasetSnapshot, properties)
}

func VolumeProperties(pool string, properties []string) ([][]string, error) {
	return DatasetProperties(pool, DatasetVolume, properties)
}
