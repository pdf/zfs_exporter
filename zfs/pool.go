package zfs

import (
	"bufio"
	"os/exec"
	"strings"
)

// PoolStatus enum contains status text
type PoolStatus string

const (
	// PoolOnline enum entry
	PoolOnline PoolStatus = `ONLINE`
	// PoolDegraded enum entry
	PoolDegraded PoolStatus = `DEGRADED`
	// PoolFaulted enum entry
	PoolFaulted PoolStatus = `FAULTED`
	// PoolOffline enum entry
	PoolOffline PoolStatus = `OFFLINE`
	// PoolUnavail enum entry
	PoolUnavail PoolStatus = `UNAVAIL`
	// PoolRemoved enum entry
	PoolRemoved PoolStatus = `REMOVED`
)

// Pool holds the properties for an individual pool
type Pool struct {
	Name       string
	Properties map[string]string
}

// processLine implements the handler interface
func (p Pool) processLine(pool string, line []string) error {
	if len(line) != 3 || line[0] != pool {
		return ErrInvalidOutput
	}
	p.Properties[line[1]] = line[2]

	return nil
}

// PoolNames returns a list of available pool names
func PoolNames() ([]string, error) {
	pools := make([]string, 0)
	cmd := exec.Command(`zpool`, `list`, `-Ho`, `name`)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(out)

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	for scanner.Scan() {
		pools = append(pools, scanner.Text())
	}
	if err = cmd.Wait(); err != nil {
		return nil, err
	}

	return pools, nil
}

// PoolProperties returns the requested properties for the given pool
func PoolProperties(pool string, properties ...string) (Pool, error) {
	handler := newPool(pool)
	if err := execute(pool, handler, `zpool`, `get`, `-Hpo`, `name,property,value`, strings.Join(properties, `,`)); err != nil {
		return handler, err
	}
	return handler, nil
}

func newPool(name string) Pool {
	return Pool{
		Name:       name,
		Properties: make(map[string]string),
	}
}
