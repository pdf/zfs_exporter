package zfs

import (
	"encoding/csv"
	"errors"
	"io"
	"os/exec"
)

var (
	// ErrInvalidOutput is returned on unparseable CLI output
	ErrInvalidOutput = errors.New(`Invalid output executing command`)
)

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

type clientImpl struct {
}

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

	r := csv.NewReader(out)
	r.Comma = '\t'
	r.LazyQuotes = true
	r.ReuseRecord = true
	r.FieldsPerRecord = 3

	if err = c.Start(); err != nil {
		return err
	}

	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err = h.processLine(pool, line); err != nil {
			return err
		}
	}

	return c.Wait()
}

// New instantiates a ZFS Client
func New() Client {
	return clientImpl{}
}
