package zfs

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Error is an error which is returned when the `zfs` or `zpool` shell
// commands return with a non-zero exit code.
type Error struct {
	Err    error
	Debug  string
	Stderr string
}

// Error returns the string representation of an Error.
func (e Error) Error() string {
	return fmt.Sprintf("%s: %q => %s", e.Err, e.Debug, e.Stderr)
}

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
			Err: err,
			//Debug:  strings.Join([]string{cmd.Path, joinedArgs[1:]}, " "),
			Debug:  strings.Join([]string{joinedArgs}, " "),
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
