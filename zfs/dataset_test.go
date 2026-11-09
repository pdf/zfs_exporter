package zfs

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const helperEnv = `ZFS_EXPORTER_TEST_HELPER`

// TestHelperProcess is not a test: fakeRunner runs the test binary with
// helperEnv set, and this prints the canned response it was given.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != `1` {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv(`HELPER_STDOUT`))
	fmt.Fprint(os.Stderr, os.Getenv(`HELPER_STDERR`))
	code, _ := strconv.Atoi(os.Getenv(`HELPER_EXIT`))
	os.Exit(code)
}

type fakeResponse struct {
	stdout string
	stderr string
	exit   int
}

// fakeRunner records each command, and answers it with the next canned
// response for its argv. The last response for an argv repeats.
type fakeRunner struct {
	t         *testing.T
	responses map[string][]fakeResponse
	calls     []string
}

func (f *fakeRunner) command(name string, args ...string) *exec.Cmd {
	argv := strings.Join(append([]string{name}, args...), ` `)
	f.calls = append(f.calls, argv)

	responses, ok := f.responses[argv]
	if !ok {
		f.t.Errorf("unexpected command: %s", argv)
		responses = []fakeResponse{{stderr: `unexpected command`, exit: 99}}
	}
	r := responses[0]
	if len(responses) > 1 {
		f.responses[argv] = responses[1:]
	}

	cmd := exec.Command(os.Args[0], `-test.run=^TestHelperProcess$`)
	cmd.Env = append(os.Environ(),
		helperEnv+`=1`,
		`HELPER_STDOUT=`+r.stdout,
		`HELPER_STDERR=`+r.stderr,
		`HELPER_EXIT=`+strconv.Itoa(r.exit),
	)
	return cmd
}

func (f *fakeRunner) count(argv string) int {
	n := 0
	for _, c := range f.calls {
		if c == argv {
			n++
		}
	}
	return n
}

const (
	listRpool = `zfs list -H -o name -d 1 -t filesystem,volume rpool`
	rpoolTree = "rpool\nrpool/ROOT\nrpool/data\nrpool/docker\n"
)

func rows(names ...string) string {
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s\tused\t1024\n", n)
	}
	return b.String()
}

func TestDatasetsProperties(t *testing.T) {
	testCases := []struct {
		name      string
		kind      DatasetKind
		prune     []string
		responses map[string][]fakeResponse
		calls     []string
		datasets  []string
	}{
		{
			name: `no prunes runs the original command`,
			kind: DatasetFilesystem,
			responses: map[string][]fakeResponse{
				`zfs get -Hprt filesystem -o name,property,value used rpool`: {{stdout: rows(`rpool`, `rpool/docker`, `rpool/docker/a`)}},
			},
			calls: []string{
				`zfs get -Hprt filesystem -o name,property,value used rpool`,
			},
			datasets: []string{`rpool`, `rpool/docker`, `rpool/docker/a`},
		},
		{
			name:  `prunes in other pools run the original command`,
			kind:  DatasetFilesystem,
			prune: []string{`rpool2/docker`, `tank`},
			responses: map[string][]fakeResponse{
				`zfs get -Hprt filesystem -o name,property,value used rpool`: {{stdout: rows(`rpool`)}},
			},
			calls: []string{
				`zfs get -Hprt filesystem -o name,property,value used rpool`,
			},
			datasets: []string{`rpool`},
		},
		{
			name:  `filesystems`,
			kind:  DatasetFilesystem,
			prune: []string{`rpool/docker`},
			responses: map[string][]fakeResponse{
				listRpool: {{stdout: rpoolTree}},
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT rpool/data`:  {{stdout: rows(`rpool/ROOT`, `rpool/ROOT/pve-1`, `rpool/data`)}},
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool rpool/docker`: {{stdout: rows(`rpool`, `rpool/docker`)}},
			},
			calls: []string{
				listRpool,
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT rpool/data`,
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool rpool/docker`,
			},
			datasets: []string{`rpool`, `rpool/ROOT`, `rpool/ROOT/pve-1`, `rpool/data`, `rpool/docker`},
		},
		{
			name:  `snapshots`,
			kind:  DatasetSnapshot,
			prune: []string{`rpool/docker`},
			responses: map[string][]fakeResponse{
				listRpool: {{stdout: rpoolTree}},
				`zfs get -Hprt snapshot -o name,property,value used rpool/ROOT rpool/data`:  {{stdout: rows(`rpool/data@a`, `rpool/data/vm@a`)}},
				`zfs get -Hpt snapshot -d 1 -o name,property,value used rpool rpool/docker`: {{stdout: rows(`rpool@a`, `rpool/docker@a`)}},
			},
			calls: []string{
				listRpool,
				`zfs get -Hprt snapshot -o name,property,value used rpool/ROOT rpool/data`,
				`zfs get -Hpt snapshot -d 1 -o name,property,value used rpool rpool/docker`,
			},
			datasets: []string{`rpool/data/vm@a`, `rpool/data@a`, `rpool/docker@a`, `rpool@a`},
		},
		{
			name:  `volumes`,
			kind:  DatasetVolume,
			prune: []string{`rpool/docker`},
			responses: map[string][]fakeResponse{
				listRpool: {{stdout: rpoolTree}},
				`zfs get -Hprt volume -o name,property,value used rpool/ROOT rpool/data`:  {{stdout: rows(`rpool/data/vm-100-disk-0`)}},
				`zfs get -Hpt volume -d 0 -o name,property,value used rpool rpool/docker`: {{}},
			},
			calls: []string{
				listRpool,
				`zfs get -Hprt volume -o name,property,value used rpool/ROOT rpool/data`,
				`zfs get -Hpt volume -d 0 -o name,property,value used rpool rpool/docker`,
			},
			datasets: []string{`rpool/data/vm-100-disk-0`},
		},
		{
			name:  `prune at pool root skips the recursive query`,
			kind:  DatasetFilesystem,
			prune: []string{`rpool`},
			responses: map[string][]fakeResponse{
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool`: {{stdout: rows(`rpool`)}},
			},
			calls: []string{
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool`,
			},
			datasets: []string{`rpool`},
		},
		{
			name:  `retries once when a dataset vanishes`,
			kind:  DatasetFilesystem,
			prune: []string{`rpool/docker`},
			responses: map[string][]fakeResponse{
				listRpool: {
					{stdout: rpoolTree},
					{stdout: "rpool\nrpool/ROOT\nrpool/docker\n"},
				},
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT rpool/data`: {{
					stdout: rows(`rpool/ROOT`),
					stderr: "cannot open 'rpool/data': dataset does not exist\n",
					exit:   1,
				}},
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT`:             {{stdout: rows(`rpool/ROOT`)}},
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool rpool/docker`: {{stdout: rows(`rpool`, `rpool/docker`)}},
			},
			calls: []string{
				listRpool,
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT rpool/data`,
				listRpool,
				`zfs get -Hprt filesystem -o name,property,value used rpool/ROOT`,
				`zfs get -Hpt filesystem -d 0 -o name,property,value used rpool rpool/docker`,
			},
			datasets: []string{`rpool`, `rpool/ROOT`, `rpool/docker`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run := &fakeRunner{t: t, responses: tc.responses}
			props, err := newDatasetsImpl(run, `rpool`, tc.kind, tc.prune).Properties(`used`)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(run.calls, tc.calls) {
				t.Errorf("calls:\n  %s\nwant:\n  %s", strings.Join(run.calls, "\n  "), strings.Join(tc.calls, "\n  "))
			}

			names := make([]string, len(props))
			for i, p := range props {
				names[i] = p.DatasetName()
				if got := p.Properties()[`used`]; got != `1024` {
					t.Errorf("%s used = %q, want 1024", p.DatasetName(), got)
				}
			}
			slices.Sort(names)
			want := slices.Sorted(slices.Values(tc.datasets))
			if !slices.Equal(names, want) {
				t.Errorf("datasets = %q, want %q", names, want)
			}
		})
	}
}

func TestDatasetsPropertiesErrors(t *testing.T) {
	const getRecursive = `zfs get -Hprt filesystem -o name,property,value used rpool/ROOT rpool/data`
	vanished := fakeResponse{stderr: "cannot open 'rpool/data': dataset does not exist\n", exit: 1}

	testCases := []struct {
		name      string
		responses map[string][]fakeResponse
		lists     int
	}{
		{
			name: `other errors are not retried`,
			responses: map[string][]fakeResponse{
				listRpool:    {{stdout: rpoolTree}},
				getRecursive: {{stderr: "permission denied\n", exit: 1}},
			},
			lists: 1,
		},
		{
			name: `a second vanished dataset fails`,
			responses: map[string][]fakeResponse{
				listRpool:    {{stdout: rpoolTree}},
				getRecursive: {vanished},
			},
			lists: 2,
		},
		{
			name: `listing errors fail`,
			responses: map[string][]fakeResponse{
				listRpool: {{stderr: "permission denied\n", exit: 1}},
			},
			lists: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run := &fakeRunner{t: t, responses: tc.responses}
			props, err := newDatasetsImpl(run, `rpool`, DatasetFilesystem, []string{`rpool/docker`}).Properties(`used`)
			if err == nil {
				t.Fatalf("Properties() = %d datasets, want an error", len(props))
			}
			if got := run.count(listRpool); got != tc.lists {
				t.Errorf("listed rpool %d times, want %d", got, tc.lists)
			}
		})
	}
}
