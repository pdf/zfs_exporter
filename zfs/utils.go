package zfs

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// List of ZFS properties to retrieve from zfs list command on a non-Solaris platform
var dsPropList = []string{"name", "origin", "used", "available", "mountpoint", "compression", "type", "volsize", "quota", "referenced", "written", "logicalused", "usedbydataset"}

var dsPropListOptions = strings.Join(dsPropList, ",")

// List of Zpool properties to retrieve from zpool list command on a non-Solaris platform
var zpoolPropList = []string{"name", "health", "allocated", "size", "free", "readonly", "dedupratio", "fragmentation", "freeing", "leaked"}
var zpoolPropListOptions = strings.Join(zpoolPropList, ",")
var zpoolArgs = []string{"get", "-p", zpoolPropListOptions}

type command struct {
	Command string
	Stdin   io.Reader
	Stdout  io.Writer
}

func (c *command) Run(arg ...string) ([][]string, error) {

	cmd := exec.Command(c.Command, arg...)

	var stdout, stderr bytes.Buffer

	if c.Stdout == nil {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = c.Stdout
	}

	if c.Stdin != nil {
		cmd.Stdin = c.Stdin

	}
	cmd.Stderr = &stderr

	joinedArgs := strings.Join(cmd.Args, " ")

	err := cmd.Run()

	if err != nil {
		return nil, &Error{
			Err:    err,
			Debug:  strings.Join([]string{cmd.Path, joinedArgs[1:]}, " "),
			Stderr: stderr.String(),
		}
	}

	// assume if you passed in something for stdout, that you know what to do with it
	if c.Stdout != nil {
		return nil, nil
	}

	lines := strings.Split(stdout.String(), "\n")

	//last line is always blank
	lines = lines[0 : len(lines)-1]
	output := make([][]string, len(lines))

	for i, l := range lines {
		output[i] = strings.Fields(l)
	}

	return output, nil
}

func setString(field *string, value string) {
	v := ""
	if value != "-" {
		v = value
	}
	*field = v
}

func setUint(field *uint64, value string) error {
	var v uint64
	if value != "-" {
		var err error
		v, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
	}
	*field = v
	return nil
}

func (ds *Dataset) parseLine(line []string) error {
	var err error

	if len(line) != len(dsPropList) {
		return errors.New("Output does not match what is expected on this platform")
	}
	setString(&ds.Name, line[0])
	setString(&ds.Origin, line[1])

	if err = setUint(&ds.Used, line[2]); err != nil {
		return err
	}
	if err = setUint(&ds.Avail, line[3]); err != nil {
		return err
	}

	setString(&ds.Mountpoint, line[4])
	setString(&ds.Compression, line[5])
	setString(&ds.Type, line[6])

	if err = setUint(&ds.Volsize, line[7]); err != nil {
		return err
	}
	if err = setUint(&ds.Quota, line[8]); err != nil {
		return err
	}
	if err = setUint(&ds.Referenced, line[9]); err != nil {
		return err
	}

	if runtime.GOOS == "solaris" {
		return nil
	}

	if err = setUint(&ds.Written, line[10]); err != nil {
		return err
	}
	if err = setUint(&ds.Logicalused, line[11]); err != nil {
		return err
	}
	if err = setUint(&ds.Usedbydataset, line[12]); err != nil {
		return err
	}

	return nil
}

func listByType(t, filter string) ([]*Dataset, error) {
	args := []string{"list", "-rHp", "-t", t, "-o", dsPropListOptions}

	if filter != "" {
		args = append(args, filter)
	}
	out, err := zfs(args...)
	if err != nil {
		return nil, err
	}

	var datasets []*Dataset

	name := ""
	var ds *Dataset
	for _, line := range out {
		if name != line[0] {
			name = line[0]
			ds = &Dataset{Name: name}
			datasets = append(datasets, ds)
		}
		if err := ds.parseLine(line); err != nil {
			return nil, err
		}
	}

	return datasets, nil
}

func (z *Zpool) parseLine(line []string) error {
	prop := line[1]
	val := line[2]

	var err error

	switch prop {
	case "name":
		setString(&z.Name, val)
	case "health":
		setString(&z.Health, val)
	case "allocated":
		err = setUint(&z.Allocated, val)
	case "size":
		err = setUint(&z.Size, val)
	case "free":
		err = setUint(&z.Free, val)
	case "fragmentation":
		// Trim trailing "%" before parsing uint
		i := strings.Index(val, "%")
		if i < 0 {
			i = len(val)
		}
		err = setUint(&z.Fragmentation, val[:i])
	case "readonly":
		z.ReadOnly = val == "on"
	case "freeing":
		err = setUint(&z.Freeing, val)
	case "leaked":
		err = setUint(&z.Leaked, val)
	case "dedupratio":
		// Trim trailing "x" before parsing float64
		z.DedupRatio, err = strconv.ParseFloat(val[:len(val)-1], 64)
	}
	return err
}
