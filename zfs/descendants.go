package zfs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// errDatasetNotExist matches a command that failed because a dataset named on
// its command line no longer exists.
var errDatasetNotExist = errors.New(`dataset does not exist`)

// commandError describes a failed CLI invocation. It matches
// errDatasetNotExist when the CLI reported a missing dataset.
type commandError struct {
	cmd      string
	stderr   string
	err      error
	notExist bool
}

func (e *commandError) Error() string {
	return fmt.Sprintf("failed to execute command '%s'; output: '%s' (%s)", e.cmd, e.stderr, e.err)
}

func (e *commandError) Unwrap() []error {
	if e.notExist {
		return []error{e.err, errDatasetNotExist}
	}
	return []error{e.err}
}

func newCommandError(c *exec.Cmd, stderr []byte, err error) error {
	msg := strings.TrimSpace(string(stderr))
	return &commandError{
		cmd:      c.String(),
		stderr:   msg,
		err:      err,
		notExist: strings.Contains(msg, errDatasetNotExist.Error()),
	}
}

// descendantsIn returns the paths that lie in pool: the pool itself, or a
// dataset below it.
func descendantsIn(pool string, paths []string) []string {
	var result []string
	for _, p := range paths {
		if p == pool || strings.HasPrefix(p, pool+`/`) {
			result = append(result, p)
		}
	}
	return result
}

// planTargets splits pool into targets that are safe to query recursively,
// and targets that must be queried without descending, so that nothing below
// a path in prune is visited. Each path in prune lands in shallow, along with
// each of its ancestors. list returns the child filesystems and volumes of a
// dataset, and is only called on those ancestors.
func planTargets(pool string, prune []string, list func(string) ([]string, error)) (recursive, shallow []string, err error) {
	var walk func(node string) error
	walk = func(node string) error {
		if slices.Contains(prune, node) {
			shallow = append(shallow, node)
			return nil
		}
		ancestor := slices.ContainsFunc(prune, func(p string) bool {
			return strings.HasPrefix(p, node+`/`)
		})
		if !ancestor {
			recursive = append(recursive, node)
			return nil
		}
		shallow = append(shallow, node)
		children, err := list(node)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(pool); err != nil {
		return nil, nil, err
	}
	return recursive, shallow, nil
}

// listChildren returns the child filesystems and volumes of node.
func listChildren(run runner, node string) ([]string, error) {
	c := run.command(`zfs`, `list`, `-H`, `-o`, `name`, `-d`, `1`, `-t`, `filesystem,volume`, node)
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err = c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command '%s': %w", c.String(), err)
	}

	var children []string
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if name := scanner.Text(); name != node {
			children = append(children, name)
		}
	}

	stde, _ := io.ReadAll(stderr)
	if err = c.Wait(); err != nil {
		return nil, newCommandError(c, stde, err)
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return children, nil
}
