package zfs

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestPlanTargets(t *testing.T) {
	// Child filesystems and volumes of each dataset in a fake pool.
	tree := map[string][]string{
		`rpool`:                 {`rpool/ROOT`, `rpool/data`, `rpool/docker`},
		`rpool/ROOT`:            {`rpool/ROOT/pve-1`},
		`rpool/data`:            {`rpool/data/vm-100-disk-0`, `rpool/data/subvol-101`, `rpool/data/subvol-102`},
		`rpool/data/subvol-101`: {`rpool/data/subvol-101/a`},
		`rpool/docker`:          {`rpool/docker/layer1`, `rpool/docker/layer2`},
	}

	testCases := []struct {
		name      string
		prune     []string
		recursive []string
		shallow   []string
		listed    []string
	}{
		{
			name:      `no prunes`,
			recursive: []string{`rpool`},
		},
		{
			name:    `prune at pool root`,
			prune:   []string{`rpool`},
			shallow: []string{`rpool`},
		},
		{
			name:      `prune one level down`,
			prune:     []string{`rpool/docker`},
			recursive: []string{`rpool/ROOT`, `rpool/data`},
			shallow:   []string{`rpool`, `rpool/docker`},
			listed:    []string{`rpool`},
		},
		{
			name:      `prune two levels down`,
			prune:     []string{`rpool/data/subvol-101`},
			recursive: []string{`rpool/ROOT`, `rpool/data/vm-100-disk-0`, `rpool/data/subvol-102`, `rpool/docker`},
			shallow:   []string{`rpool`, `rpool/data`, `rpool/data/subvol-101`},
			listed:    []string{`rpool`, `rpool/data`},
		},
		{
			name:      `sibling prunes under one parent`,
			prune:     []string{`rpool/data/subvol-101`, `rpool/data/subvol-102`},
			recursive: []string{`rpool/ROOT`, `rpool/data/vm-100-disk-0`, `rpool/docker`},
			shallow:   []string{`rpool`, `rpool/data`, `rpool/data/subvol-101`, `rpool/data/subvol-102`},
			listed:    []string{`rpool`, `rpool/data`},
		},
		{
			name:      `nested prunes`,
			prune:     []string{`rpool/data`, `rpool/data/subvol-101`},
			recursive: []string{`rpool/ROOT`, `rpool/docker`},
			shallow:   []string{`rpool`, `rpool/data`},
			listed:    []string{`rpool`},
		},
		{
			name:      `prune that does not exist`,
			prune:     []string{`rpool/missing/child`},
			recursive: []string{`rpool/ROOT`, `rpool/data`, `rpool/docker`},
			shallow:   []string{`rpool`},
			listed:    []string{`rpool`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var listed []string
			list := func(node string) ([]string, error) {
				listed = append(listed, node)
				return tree[node], nil
			}
			recursive, shallow, err := planTargets(`rpool`, tc.prune, list)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(recursive, tc.recursive) {
				t.Errorf("recursive = %q, want %q", recursive, tc.recursive)
			}
			if !slices.Equal(shallow, tc.shallow) {
				t.Errorf("shallow = %q, want %q", shallow, tc.shallow)
			}
			if !slices.Equal(listed, tc.listed) {
				t.Errorf("listed = %q, want %q", listed, tc.listed)
			}
			assertDisjoint(t, recursive, shallow)
		})
	}
}

// assertDisjoint checks that no dataset can be returned by two queries: no
// target appears twice, and no target lies below a recursive target. Shallow
// targets do not descend, so targets below them are fine.
func assertDisjoint(t *testing.T, recursive, shallow []string) {
	t.Helper()
	all := slices.Concat(recursive, shallow)
	for i, a := range all {
		for j, b := range all {
			if i != j && a == b {
				t.Errorf("target %q appears twice", a)
			}
		}
	}
	for _, r := range recursive {
		for _, other := range all {
			if strings.HasPrefix(other, r+`/`) {
				t.Errorf("target %q lies below recursive target %q", other, r)
			}
		}
	}
}

func TestPlanTargetsListError(t *testing.T) {
	want := errors.New(`list failed`)
	_, _, err := planTargets(`rpool`, []string{`rpool/docker`}, func(string) ([]string, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestDescendantsIn(t *testing.T) {
	got := descendantsIn(`rpool`, []string{`rpool`, `rpool/docker`, `rpool2/docker`, `tank/docker`})
	want := []string{`rpool`, `rpool/docker`}
	if !slices.Equal(got, want) {
		t.Errorf("descendantsIn = %q, want %q", got, want)
	}
}

func TestCommandErrorNotExist(t *testing.T) {
	cmd := exec.Command(`zfs`, `get`)
	exit := errors.New(`exit status 1`)

	err := newCommandError(cmd, []byte("cannot open 'rpool/docker/x': dataset does not exist\n"), exit)
	if !errors.Is(err, errDatasetNotExist) {
		t.Errorf("errors.Is(%v, errDatasetNotExist) = false, want true", err)
	}
	if !errors.Is(err, exit) {
		t.Errorf("errors.Is(%v, exit) = false, want true", err)
	}

	err = newCommandError(cmd, []byte("permission denied\n"), exit)
	if errors.Is(err, errDatasetNotExist) {
		t.Errorf("errors.Is(%v, errDatasetNotExist) = true, want false", err)
	}
}
