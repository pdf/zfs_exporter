package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PoolStatus enum contains status text
type PoolStatus string

const (
	// PoolOnline enum entry
	PoolOnline PoolStatus = `ONLINE`
	// PoolDegraded enum entry
	PoolDegraded PoolStatus = `DEGRADED`
	// PoolFaulted enum entry
	PoolFaulted PoolStatus = `FAULTED`
	// PoolOffline enum entry
	PoolOffline PoolStatus = `OFFLINE`
	// PoolUnavail enum entry
	PoolUnavail PoolStatus = `UNAVAIL`
	// PoolRemoved enum entry
	PoolRemoved PoolStatus = `REMOVED`
	// PoolSuspended enum entry
	PoolSuspended PoolStatus = `SUSPENDED`
)

type poolImpl struct {
	name string
}

func (p poolImpl) Name() string {
	return p.name
}

func (p poolImpl) Properties(props ...string) (PoolProperties, error) {
	handler := newPoolPropertiesImpl()
	if err := execute(p.name, handler, `zpool`, `get`, `-Hpo`, `name,property,value`, strings.Join(props, `,`)); err != nil {
		return handler, err
	}
	return handler, nil
}

func (p poolImpl) Scan() (PoolScan, error) {
	cmd := exec.Command(`zpool`, `status`, `-p`, p.name)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PoolScan{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return PoolScan{}, err
	}

	if err = cmd.Start(); err != nil {
		return PoolScan{}, fmt.Errorf("failed to start command '%s': %w", cmd.String(), err)
	}

	out, err := io.ReadAll(stdout)
	if err != nil {
		return PoolScan{}, err
	}
	stde, _ := io.ReadAll(stderr)

	if err = cmd.Wait(); err != nil {
		return PoolScan{}, fmt.Errorf("failed to execute command '%s'; output: '%s' (%w)", cmd.String(), strings.TrimSpace(string(stde)), err)
	}

	return parsePoolScan(out)
}

type poolPropertiesImpl struct {
	properties map[string]string
}

func (p *poolPropertiesImpl) Properties() map[string]string {
	return p.properties
}

// processLine implements the handler interface
func (p *poolPropertiesImpl) processLine(pool string, line []string) error {
	if len(line) != 3 || line[0] != pool {
		return ErrInvalidOutput
	}
	p.properties[line[1]] = line[2]

	return nil
}

// PoolNames returns a list of available pool names
func poolNames() ([]string, error) {
	pools := make([]string, 0)
	cmd := exec.Command(`zpool`, `list`, `-Ho`, `name`)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(out)

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command '%s': %w", cmd.String(), err)
	}

	for scanner.Scan() {
		pools = append(pools, scanner.Text())
	}

	stde, _ := io.ReadAll(stderr)
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute command '%s'; output: '%s' (%w)", cmd.String(), strings.TrimSpace(string(stde)), err)
	}

	return pools, nil
}

func newPoolImpl(name string) poolImpl {
	return poolImpl{
		name: name,
	}
}

func newPoolPropertiesImpl() *poolPropertiesImpl {
	return &poolPropertiesImpl{
		properties: make(map[string]string),
	}
}

var (
	scanCompletedRe    = regexp.MustCompile(`^(scrub repaired|resilvered) (\d+)B? in .+ with (\d+) errors on (.+)$`)
	scanProgressPartRe = regexp.MustCompile(`^(\S+)\s*/\s*(\S+)\s+(scanned|issued)\s+at\s+(\S+?)/s$`)
)

func parsePoolScan(out []byte) (PoolScan, error) {
	scan := PoolScan{Function: ScanFunctionNone, State: ScanStateNone}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, `scan:`) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, `scan:`))
		parsed, err := parseScanLine(rest)
		if err != nil {
			return PoolScan{}, err
		}
		scan = parsed
		if scan.State != ScanStateInProgress {
			return scan, nil
		}
		for scanner.Scan() {
			cont := scanner.Text()
			if !strings.HasPrefix(cont, "\t") {
				break
			}
			parseScanProgressLine(strings.TrimSpace(cont), &scan)
		}
		return scan, nil
	}
	if err := scanner.Err(); err != nil {
		return PoolScan{}, err
	}
	return scan, nil
}

func parseScanLine(rest string) (PoolScan, error) {
	scan := PoolScan{Function: ScanFunctionNone, State: ScanStateNone}
	switch {
	case rest == `none requested`:
		return scan, nil
	case strings.HasPrefix(rest, `scrub in progress`):
		scan.Function = ScanFunctionScrub
		scan.State = ScanStateInProgress
	case strings.HasPrefix(rest, `scrub paused since`):
		scan.Function = ScanFunctionScrub
		scan.State = ScanStatePaused
	case strings.HasPrefix(rest, `scrub canceled on`):
		scan.Function = ScanFunctionScrub
		scan.State = ScanStateCanceled
	case strings.HasPrefix(rest, `resilver in progress`):
		scan.Function = ScanFunctionResilver
		scan.State = ScanStateInProgress
	default:
		m := scanCompletedRe.FindStringSubmatch(rest)
		if m == nil {
			return PoolScan{}, fmt.Errorf("%w: unrecognized scan line: %q", ErrInvalidOutput, rest)
		}
		if m[1] == `scrub repaired` {
			scan.Function = ScanFunctionScrub
		} else {
			scan.Function = ScanFunctionResilver
		}
		scan.State = ScanStateFinished
		repaired, err := strconv.ParseUint(m[2], 10, 64)
		if err != nil {
			return PoolScan{}, fmt.Errorf("%w: invalid repaired bytes %q: %w", ErrInvalidOutput, m[2], err)
		}
		scan.Repaired = repaired
		errs, err := strconv.ParseUint(m[3], 10, 64)
		if err != nil {
			return PoolScan{}, fmt.Errorf("%w: invalid error count %q: %w", ErrInvalidOutput, m[3], err)
		}
		scan.Errors = errs
		t, err := time.ParseInLocation(time.ANSIC, strings.TrimSpace(m[4]), time.Local)
		if err != nil {
			return PoolScan{}, fmt.Errorf("%w: invalid completion timestamp %q: %w", ErrInvalidOutput, m[4], err)
		}
		scan.CompletedAt = t
	}
	return scan, nil
}

func parseScanProgressLine(line string, scan *PoolScan) {
	for _, part := range strings.Split(line, `, `) {
		part = strings.TrimSpace(part)
		if m := scanProgressPartRe.FindStringSubmatch(part); m != nil {
			done, derr := parseScanBytes(m[1])
			total, terr := parseScanBytes(m[2])
			rate, rerr := parseScanBytes(m[4])
			if derr != nil || terr != nil || rerr != nil {
				continue
			}
			if m[3] == `scanned` {
				scan.Scanned = done
				scan.Total = total
				scan.Rate = rate
			} else {
				scan.Issued = done
				if scan.Total == 0 {
					scan.Total = total
				}
			}
			continue
		}
		if strings.HasSuffix(part, ` repaired`) {
			if v, err := parseScanBytes(strings.TrimSuffix(part, ` repaired`)); err == nil {
				scan.Repaired = v
			}
		}
	}
}

func parseScanBytes(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == `` {
		return 0, fmt.Errorf("%w: empty byte value", ErrInvalidOutput)
	}
	s = strings.TrimSuffix(s, `B`)
	if s == `` {
		return 0, nil
	}
	var multiplier uint64 = 1
	switch s[len(s)-1] {
	case 'K', 'k':
		multiplier = 1 << 10
	case 'M', 'm':
		multiplier = 1 << 20
	case 'G', 'g':
		multiplier = 1 << 30
	case 'T', 't':
		multiplier = 1 << 40
	case 'P', 'p':
		multiplier = 1 << 50
	}
	if multiplier == 1 {
		return strconv.ParseUint(s, 10, 64)
	}
	f, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, err
	}
	return uint64(f * float64(multiplier)), nil
}
