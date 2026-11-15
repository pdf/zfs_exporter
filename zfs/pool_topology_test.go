package zfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePoolTopology(t *testing.T) {
	root := t.TempDir()
	for dev, sectors := range map[string]string{"sdb": "1953125000\n", "sdc": "1953125000\n", "nvme0n1p1": "3906250000\n"} {
		p := filepath.Join(root, "sys", "class", "block", dev)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "size"), []byte(sectors), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const fixture = "tank\t2000000000000\t700000000000\t1300000000000\n" +
		"\tmirror-0\t1000000000000\t400000000000\t600000000000\n" +
		"\t\t/dev/sdb\t-\t-\t-\n" +
		"\t\t/dev/sdc\t-\t-\t-\n" +
		"\t/dev/nvme0n1p1\t1000000000000\t300000000000\t700000000000\n"
	got, err := parsePoolTopology(strings.NewReader(fixture), "tank", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Vdevs) != 2 || len(got.Disks) != 3 {
		t.Fatalf("unexpected topology: %+v", got)
	}
	if got.Vdevs[0].AllocatedBytes != 400000000000 {
		t.Fatalf("bad allocated size: %+v", got.Vdevs[0])
	}
	if got.Disks[0].Vdev != "mirror-0" || got.Disks[0].SizeBytes != 1000000000000 {
		t.Fatalf("bad disk: %+v", got.Disks[0])
	}
	if got.Disks[2].Vdev != "/dev/nvme0n1p1" || got.Disks[2].SizeBytes != 2000000000000 {
		t.Fatalf("bad single-disk VDEV: %+v", got.Disks[2])
	}
}

func TestParsePoolTopologyMissingDevice(t *testing.T) {
	fixture := "tank\t2000\t1000\t1000\n\t/dev/sdzz\t2000\t1000\t1000\n"
	_, err := parsePoolTopology(strings.NewReader(fixture), "tank", t.TempDir())
	if err == nil {
		t.Fatal("expected sysfs failure")
	}
}

func TestParsePoolTopologyInvalid(t *testing.T) {
	_, err := parsePoolTopology(strings.NewReader("tank\tbad\t3\t4\n\tmirror-0\twrong\t2\t3\n"), "tank", t.TempDir())
	if err == nil {
		t.Fatal("expected invalid vdev numeric value")
	}
}

func TestParsePoolTopologySpecialVdev(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "sys", "class", "block", "sdd")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "size"), []byte("1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := "tank\t512000\t120000\t392000\n" +
		"\tlogs\t-\t-\t-\n" +
		"\t\tmirror-1\t512000\t120000\t392000\n" +
		"\t\t\t/dev/sdd\t-\t-\t-\n"
	got, err := parsePoolTopology(strings.NewReader(fixture), "tank", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Vdevs) != 1 || len(got.Disks) != 1 || got.Disks[0].Vdev != "mirror-1" {
		t.Fatalf("bad nested topology: %+v", got)
	}
}

// Regression fixture from a real host: the pool row has four fields, while
// the direct disk vdev includes ten fields even with -o name,size,allocated,free.
func TestParsePoolTopologyRealSingleDiskTenColumns(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "sys", "class", "block", "sdb1")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	// Raw block-device capacity: 5497556041728 bytes / 512.
	if err := os.WriteFile(filepath.Join(p, "size"), []byte("10737414144\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const fixture = "zpool-hdd\t5480378269696\t25087770624\t5455290499072\n" +
		"\t/dev/sdb1\t5497556041728\t25087770624\t5455290499072\t-\t-\t1\t0\t-\tONLINE\n"
	got, err := parsePoolTopology(strings.NewReader(fixture), "zpool-hdd", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Vdevs) != 1 || len(got.Disks) != 1 {
		t.Fatalf("want 1 VDEV and 1 disk, got: %+v", got)
	}
	if v := got.Vdevs[0]; v.Name != "/dev/sdb1" || v.SizeBytes != 5497556041728 || v.AllocatedBytes != 25087770624 || v.FreeBytes != 5455290499072 {
		t.Fatalf("unexpected direct-disk VDEV: %+v", v)
	}
	if d := got.Disks[0]; d.Vdev != "/dev/sdb1" || d.Device != "/dev/sdb1" || d.SizeBytes != 5497556041728 {
		t.Fatalf("unexpected disk: %+v", d)
	}
}

func TestParsePoolTopologyTooFewColumns(t *testing.T) {
	_, err := parsePoolTopology(strings.NewReader("zpool-hdd\t100\t50\n"), "zpool-hdd", t.TempDir())
	if err == nil {
		t.Fatal("expected error when FREE column is missing")
	}
}
