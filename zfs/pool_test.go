package zfs

import (
	"testing"
	"time"
)

var (
	mib = float64(uint64(1) << 20)
	gib = float64(uint64(1) << 30)
	tib = float64(uint64(1) << 40)
	pib = float64(uint64(1) << 50)
)

func TestParsePoolScan(t *testing.T) {
	testCases := []struct {
		name     string
		output   string
		expected PoolScan
	}{
		{
			name: `none requested`,
			output: `  pool: tank
 state: ONLINE
  scan: none requested
config:
`,
			expected: PoolScan{Function: ScanFunctionNone, State: ScanStateNone},
		},
		{
			name: `scrub in progress`,
			output: `  pool: tank
 state: ONLINE
  scan: scrub in progress since Sun May 12 00:24:01 2024
config:
`,
			expected: PoolScan{Function: ScanFunctionScrub, State: ScanStateInProgress},
		},
		{
			name: `scrub in progress with progress lines`,
			output: "  pool: vault\n" +
				" state: ONLINE\n" +
				"  scan: scrub in progress since Mon Jun  1 02:00:02 2026\n" +
				"\t10.4T / 30.2T scanned at 864M/s, 7.60T / 30.2T issued at 635M/s\n" +
				"\t0B repaired, 25.14% done, 10:23:16 to go\n" +
				"config:\n",
			expected: PoolScan{
				Function: ScanFunctionScrub,
				State:    ScanStateInProgress,
				Total:    uint64(30.2 * tib),
				Scanned:  uint64(10.4 * tib),
				Issued:   uint64(7.60 * tib),
				Rate:     uint64(864 * mib),
				Repaired: 0,
			},
		},
		{
			name: `scrub in progress with parsable byte values`,
			output: "  pool: tank\n" +
				"  scan: scrub in progress since Mon Jun  1 02:00:02 2026\n" +
				"\t11428000000000 / 33200000000000 scanned at 864000000/s, 7600000000000 / 33200000000000 issued at 635000000/s\n" +
				"\t4096 repaired, 25.14% done, 10:23:16 to go\n",
			expected: PoolScan{
				Function: ScanFunctionScrub,
				State:    ScanStateInProgress,
				Total:    33200000000000,
				Scanned:  11428000000000,
				Issued:   7600000000000,
				Rate:     864000000,
				Repaired: 4096,
			},
		},
		{
			name: `scrub paused`,
			output: `  pool: tank
 state: ONLINE
  scan: scrub paused since Sun May 12 00:24:01 2024
config:
`,
			expected: PoolScan{Function: ScanFunctionScrub, State: ScanStatePaused},
		},
		{
			name: `scrub canceled`,
			output: `  pool: tank
 state: ONLINE
  scan: scrub canceled on Sun May 12 00:24:01 2024
config:
`,
			expected: PoolScan{Function: ScanFunctionScrub, State: ScanStateCanceled},
		},
		{
			name: `scrub finished`,
			output: `  pool: tank
 state: ONLINE
  scan: scrub repaired 4096 in 02:18:36 with 3 errors on Sun May 12 02:42:37 2024
config:
`,
			expected: PoolScan{
				Function:    ScanFunctionScrub,
				State:       ScanStateFinished,
				Errors:      3,
				Repaired:    4096,
				CompletedAt: time.Date(2024, 5, 12, 2, 42, 37, 0, time.Local),
			},
		},
		{
			name: `scrub finished with B suffix`,
			output: `  pool: tank
 state: ONLINE
  scan: scrub repaired 0B in 02:18:36 with 0 errors on Sun May 12 02:42:37 2024
`,
			expected: PoolScan{
				Function:    ScanFunctionScrub,
				State:       ScanStateFinished,
				Errors:      0,
				Repaired:    0,
				CompletedAt: time.Date(2024, 5, 12, 2, 42, 37, 0, time.Local),
			},
		},
		{
			name: `resilver in progress`,
			output: `  pool: tank
 state: DEGRADED
  scan: resilver in progress since Sun May 12 00:24:01 2024
`,
			expected: PoolScan{Function: ScanFunctionResilver, State: ScanStateInProgress},
		},
		{
			name: `resilver finished`,
			output: `  pool: tank
 state: ONLINE
  scan: resilvered 1024 in 01:00:00 with 0 errors on Sun May 12 02:42:37 2024
`,
			expected: PoolScan{
				Function:    ScanFunctionResilver,
				State:       ScanStateFinished,
				Errors:      0,
				Repaired:    1024,
				CompletedAt: time.Date(2024, 5, 12, 2, 42, 37, 0, time.Local),
			},
		},
		{
			name:     `missing scan line`,
			output:   "  pool: tank\n state: ONLINE\nconfig:\n",
			expected: PoolScan{Function: ScanFunctionNone, State: ScanStateNone},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePoolScan([]byte(tc.output))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Function != tc.expected.Function {
				t.Errorf("Function: got %q, want %q", got.Function, tc.expected.Function)
			}
			if got.State != tc.expected.State {
				t.Errorf("State: got %q, want %q", got.State, tc.expected.State)
			}
			if got.Errors != tc.expected.Errors {
				t.Errorf("Errors: got %d, want %d", got.Errors, tc.expected.Errors)
			}
			if got.Repaired != tc.expected.Repaired {
				t.Errorf("Repaired: got %d, want %d", got.Repaired, tc.expected.Repaired)
			}
			if !got.CompletedAt.Equal(tc.expected.CompletedAt) {
				t.Errorf("CompletedAt: got %s, want %s", got.CompletedAt, tc.expected.CompletedAt)
			}
			if got.Total != tc.expected.Total {
				t.Errorf("Total: got %d, want %d", got.Total, tc.expected.Total)
			}
			if got.Scanned != tc.expected.Scanned {
				t.Errorf("Scanned: got %d, want %d", got.Scanned, tc.expected.Scanned)
			}
			if got.Issued != tc.expected.Issued {
				t.Errorf("Issued: got %d, want %d", got.Issued, tc.expected.Issued)
			}
			if got.Rate != tc.expected.Rate {
				t.Errorf("Rate: got %d, want %d", got.Rate, tc.expected.Rate)
			}
		})
	}
}

func TestParseScanBytes(t *testing.T) {
	testCases := []struct {
		in   string
		want uint64
	}{
		{`0`, 0},
		{`0B`, 0},
		{`1024`, 1024},
		{`10K`, 10 << 10},
		{`10.4M`, uint64(10.4 * mib)},
		{`7.60G`, uint64(7.60 * gib)},
		{`30.2T`, uint64(30.2 * tib)},
		{`1.5P`, uint64(1.5 * pib)},
		{`11428000000000`, 11428000000000},
	}
	for _, tc := range testCases {
		got, err := parseScanBytes(tc.in)
		if err != nil {
			t.Errorf("parseScanBytes(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseScanBytes(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParsePoolScanInvalid(t *testing.T) {
	output := "  pool: tank\n  scan: scrub repaired bogus stuff\n"
	if _, err := parsePoolScan([]byte(output)); err == nil {
		t.Fatal("expected error parsing invalid scan line")
	}
}
