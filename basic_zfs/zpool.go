package basic_zfs

import (
	"os/exec"
	"strings"
)

const (
	ZpoolOnline   = "ONLINE"
	ZpoolDegraded = "DEGRADED"
	ZpoolFaulted  = "FAULTED"
	ZpoolOffline  = "OFFLINE"
	ZpoolUnavail  = "UNAVAIL"
	ZpoolRemoved  = "REMOVED"
)

func ListPools() ([]string, error) {
	out, err := exec.Command("zpool", "list", "-Hpo", "name").Output()
	if err != nil {
		return nil, err
	}

	output := string(out[:])
	return strings.Fields(output), nil
}

func PoolProperties(pools []string, poolProperties []string) ([][]string, error) {
	joinedDsProps := strings.Join(poolProperties, ",")
	zpoolArgs := []string{"list", "-Hpo", joinedDsProps}
	zpoolArgs = append(zpoolArgs, pools...)
	outBytes, err := exec.Command("zpool", zpoolArgs...).Output()
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
