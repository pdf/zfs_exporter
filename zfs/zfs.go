//go:generate go tool go.uber.org/mock/mockgen -source=zfs.go -destination=mock_zfs/mock_zfs.go -package=mock_zfs

package zfs

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// ErrInvalidOutput is returned on unparseable CLI output
var ErrInvalidOutput = errors.New(`invalid output executing command`)

// Client is the primary entrypoint
type Client interface {
	PoolNames() ([]string, error)
	Pool(name string) Pool
	Datasets(pool string, kind DatasetKind) Datasets
}

// Pool allows querying pool properties
type Pool interface {
	Name() string
	Properties(props ...string) (PoolProperties, error)
	Scan() (PoolScan, error)
}

// PoolProperties provides access to the properties for a pool
type PoolProperties interface {
	Properties() map[string]string
}

// ScanFunction enum contains scan function text
type ScanFunction string

const (
	// ScanFunctionNone enum entry
	ScanFunctionNone ScanFunction = `none`
	// ScanFunctionScrub enum entry
	ScanFunctionScrub ScanFunction = `scrub`
	// ScanFunctionResilver enum entry
	ScanFunctionResilver ScanFunction = `resilver`
)

// ScanState enum contains scan state text
type ScanState string

const (
	// ScanStateNone enum entry
	ScanStateNone ScanState = `none`
	// ScanStateInProgress enum entry
	ScanStateInProgress ScanState = `in_progress`
	// ScanStateFinished enum entry
	ScanStateFinished ScanState = `finished`
	// ScanStateCanceled enum entry
	ScanStateCanceled ScanState = `canceled`
	// ScanStatePaused enum entry
	ScanStatePaused ScanState = `paused`
)

// PoolScan describes the most recent scrub or resilver for a pool
type PoolScan struct {
	Function    ScanFunction
	State       ScanState
	Errors      uint64
	Repaired    uint64
	CompletedAt time.Time
	Total       uint64
	Scanned     uint64
	Issued      uint64
	Rate        uint64
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

type clientImpl struct{}

func (z clientImpl) PoolNames() ([]string, error) {
	return poolNames()
}

func (z clientImpl) Pool(name string) Pool {
	return newPoolImpl(name)
}

func (z clientImpl) Datasets(pool string, kind DatasetKind) Datasets {
	return newDatasetsImpl(pool, kind)
}

func execute(pool string, h handler, cmd string, args ...string) error {
	c := exec.Command(cmd, append(args, pool)...)
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
	return clientImpl{}
}
