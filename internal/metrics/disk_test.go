package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestSamplerReportsPerDeviceStorageEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, 100, 200, 10, 20, 100, 1000, 2000, 20, 10)
	mustWrite(t, root+"/diskstats", diskFixture(
		"sda", 10, 100, 50, 20, 200, 100, 1, 100, 200,
		"sdb", 20, 200, 100, 30, 300, 150, 2, 200, 400,
	))

	s := NewSampler(root)
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	if _, err := s.Sample(t0); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, root, 120, 260, 50, 40, 300, 5000, 8000, 30, 20)
	mustWrite(t, root+"/diskstats", diskFixture(
		"sda", 20, 120, 150, 30, 260, 300, 3, 2600, 5200,
		"sdb", 30, 300, 500, 40, 500, 950, 4, 5200, 15400,
	))
	got, err := s.Sample(t0.Add(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Disks) != 2 {
		t.Fatalf("disks=%+v", got.Disks)
	}
	sda, sdb := got.Disks[0], got.Disks[1]
	if sda.Name != "sda" || sda.Source != "proc-diskstats" {
		t.Fatalf("sda identity=%+v", sda)
	}
	if sda.ReadBPS != 2048 || sda.WriteBPS != 6144 || sda.BusyPercent != 50 {
		t.Fatalf("sda rates=%+v", sda)
	}
	if sda.AvgQueueDepth != 1 || sda.AvgAwaitMS != 15 || sda.IOInProgress != 3 {
		t.Fatalf("sda latency/queue=%+v", sda)
	}
	if sdb.Name != "sdb" || sdb.BusyPercent != 100 || sdb.AvgQueueDepth != 3 || sdb.AvgAwaitMS != 60 {
		t.Fatalf("sdb evidence=%+v", sdb)
	}
	if got.DiskReadBPS != 12288 || got.DiskWriteBPS != 26624 {
		t.Fatalf("aggregate disk rates read=%f write=%f", got.DiskReadBPS, got.DiskWriteBPS)
	}

	findings := Diagnose(got)
	for _, f := range findings {
		if f.Code == "storage-wait" {
			if !strings.Contains(f.Evidence, "sda") || !strings.Contains(f.Evidence, "sdb") ||
				!strings.Contains(f.Evidence, "proc-diskstats") {
				t.Fatalf("storage evidence=%q", f.Evidence)
			}
			return
		}
	}
	t.Fatalf("storage-wait not found in %+v", findings)
}

func diskFixture(a string, ar, ars, arm, aw, aws, awm, aip, aio, awio uint64, b string, br, brs, brm, bw, bws, bwm, bip, bio, bwio uint64) string {
	return "8 0 " + a + " " + u(ar) + " 0 " + u(ars) + " " + u(arm) + " " + u(aw) + " 0 " + u(aws) + " " + u(awm) + " " + u(aip) + " " + u(aio) + " " + u(awio) + "\n" +
		"8 16 " + b + " " + u(br) + " 0 " + u(brs) + " " + u(brm) + " " + u(bw) + " 0 " + u(bws) + " " + u(bwm) + " " + u(bip) + " " + u(bio) + " " + u(bwio) + "\n" +
		"8 1 sda1 999 0 999 999 999 0 999 999 9 999 999\n"
}

func TestDiskSamplesIgnoreRegressedCounters(t *testing.T) {
	previous := map[string]diskDeviceCounters{
		"sda": {readSectors: 1000, writeSectors: 2000, readIOs: 100, writeIOs: 200, readMS: 1000, writeMS: 2000, ioMS: 3000, weightedIOMS: 4000},
		"sdb": {readSectors: 1000, writeSectors: 2000, readIOs: 100, writeIOs: 200, readMS: 1000, writeMS: 2000, ioMS: 3000, weightedIOMS: 4000},
	}
	current := map[string]diskDeviceCounters{
		"sda": {readSectors: 10, writeSectors: 20, readIOs: 1, writeIOs: 2, readMS: 10, writeMS: 20, ioMS: 30, weightedIOMS: 40},
		"sdb": {readSectors: 1100, writeSectors: 2200, readIOs: 110, writeIOs: 220, readMS: 1100, writeMS: 2200, ioMS: 3500, weightedIOMS: 4500},
	}
	got := diskSamples(current, previous, 5)
	if len(got) != 2 {
		t.Fatalf("disks=%+v", got)
	}
	if got[0].Name != "sda" || got[0].ReadBPS != 0 || got[0].WriteBPS != 0 ||
		got[0].BusyPercent != 0 || got[0].AvgQueueDepth != 0 || got[0].AvgAwaitMS != 0 {
		t.Fatalf("regressed sda counters produced rates: %+v", got[0])
	}
	if got[1].Name != "sdb" || got[1].ReadBPS != 10240 || got[1].WriteBPS != 20480 {
		t.Fatalf("monotonic sdb counters lost valid rates: %+v", got[1])
	}
}
