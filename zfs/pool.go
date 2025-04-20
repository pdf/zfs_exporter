package zfs

import (
	"bufio"
	"fmt"
	"io"
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
	// PoolSuspended enum entry
	PoolSuspended PoolStatus = `SUSPENDED`
)

type poolImpl struct {
	name string
}

func (p poolImpl) Name() string {
	return p.name
}

func (p poolImpl) Properties(props ...string) (PoolProperties, error) {
	handler := newPoolPropertiesImpl()
	if err := execute(p.name, handler, `zpool`, `get`, `-Hpo`, `name,property,value`, strings.Join(props, `,`)); err != nil {
		return handler, err
	}
	return handler, nil
}

type poolPropertiesImpl struct {
	properties map[string]string
}

func (p *poolPropertiesImpl) Properties() map[string]string {
	return p.properties
}

// processLine implements the handler interface
func (p *poolPropertiesImpl) processLine(pool string, line []string) error {
	if len(line) != 3 || line[0] != pool {
		return ErrInvalidOutput
	}
	p.properties[line[1]] = line[2]

	return nil
}

// PoolNames returns a list of available pool names
func poolNames() ([]string, error) {
	pools := make([]string, 0)
	cmd := exec.Command(`zpool`, `list`, `-Ho`, `name`)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(out)

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command '%s': %w", cmd.String(), err)
	}

	for scanner.Scan() {
		pools = append(pools, scanner.Text())
	}

	stde, _ := io.ReadAll(stderr)
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute command '%s'; output: '%s' (%w)", cmd.String(), strings.TrimSpace(string(stde)), err)
	}

	return pools, nil
}

func newPoolImpl(name string) poolImpl {
	return poolImpl{
		name: name,
	}
}

func newPoolPropertiesImpl() *poolPropertiesImpl {
	return &poolPropertiesImpl{
		properties: make(map[string]string),
	}
}
