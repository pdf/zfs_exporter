package basic_zfs

import (
	"os/exec"
	"strings"
)

func ListPools() ([]string, error) {
	out, err := exec.Command("zpool", "list", "-Hpo", "name").Output()
	if err != nil {
		return nil, err
	}

	output := string(out[:])
	return strings.Fields(output), nil
}
