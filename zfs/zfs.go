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

type handler interface {
	processLine(pool string, line []string) error
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
