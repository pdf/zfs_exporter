//go:generate go tool go.uber.org/mock/mockgen -source=zfs.go -destination=mock_zfs/mock_zfs.go -package=mock_zfs

package zfs

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ErrInvalidOutput is returned on unparseable CLI output
var ErrInvalidOutput = errors.New(`invalid output executing command`)

// Client is the primary entrypoint
type Client interface {
	PoolNames() ([]string, error)
	Pool(name string) Pool
	Datasets(pool string, kind DatasetKind) Datasets
	PoolTopology(pool string) (PoolTopology, error)
}

// Pool allows querying pool properties
type Pool interface {
	Name() string
	Properties(props ...string) (PoolProperties, error)
}

// PoolProperties provides access to the properties for a pool
type PoolProperties interface {
	Properties() map[string]string
}

// Datasets allows querying properties for datasets in a pool
type Datasets interface {
	Pool() string
	Kind() DatasetKind
	Properties(props ...string) ([]DatasetProperties, error)
}

// DatasetProperties provides access to the properties for a dataset
type DatasetProperties interface {
	DatasetName() string
	Properties() map[string]string
}

type handler interface {
	processLine(pool string, line []string) error
}

// runner builds the command for a ZFS CLI invocation
type runner interface {
	command(name string, args ...string) *exec.Cmd
}

// directRunner runs the ZFS CLI found on the exporter's own PATH
type directRunner struct{}

func (directRunner) command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

type clientImpl struct {
	run       runner
	sysfsRoot string
}

func (z clientImpl) PoolNames() ([]string, error) {
	return poolNames(z.run)
}

func (z clientImpl) Pool(name string) Pool {
	return newPoolImpl(z.run, name)
}

func (z clientImpl) Datasets(pool string, kind DatasetKind) Datasets {
	return newDatasetsImpl(z.run, pool, kind)
}

func execute(run runner, pool string, h handler, cmd string, args ...string) error {
	c := run.command(cmd, append(args, pool)...)
	out, err := c.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := c.StderrPipe()
	if err != nil {
		return err
	}

	r := csv.NewReader(out)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.ReuseRecord = true
	r.FieldsPerRecord = 3

	if err = c.Start(); err != nil {
		return fmt.Errorf("failed to start command '%s': %w", c.String(), err)
	}

	for {
		line, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err = h.processLine(pool, line); err != nil {
			return err
		}
	}

	stde, _ := io.ReadAll(stderr)
	if err = c.Wait(); err != nil {
		return fmt.Errorf("failed to execute command '%s'; output: '%s' (%w)", c.String(), strings.TrimSpace(string(stde)), err)
	}
	return nil
}

// New instantiates a ZFS Client
func New() Client {
	return clientImpl{run: directRunner{}, sysfsRoot: "/"}
}
