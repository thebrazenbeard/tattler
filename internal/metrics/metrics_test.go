package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSamplerAndDiagnose(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, 100, 200, 10, 20, 100, 1000, 2000, 20, 10)

	s := NewSampler(root)
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	first, err := s.Sample(t0)
	if err != nil {
		t.Fatal(err)
	}
	if first.CPUCores != 2 || first.MemAvailableKB != 8000 {
		t.Fatalf("unexpected first sample: %+v", first)
	}

	writeFixture(t, root, 120, 260, 50, 40, 300, 5000, 8000, 30, 20)
	second, err := s.Sample(t0.Add(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if second.CPUPercent < 49.9 || second.CPUPercent > 50.1 {
		t.Fatalf("cpu_percent=%f", second.CPUPercent)
	}
	if second.IOWaitPercent < 19.9 || second.IOWaitPercent > 20.1 {
		t.Fatalf("io_wait_percent=%f", second.IOWaitPercent)
	}
	if second.DiskReadBPS != 2048 || second.DiskWriteBPS != 6144 {
		t.Fatalf("disk rates read=%f write=%f", second.DiskReadBPS, second.DiskWriteBPS)
	}
	if second.SwapInPagesPerSec != 8 || second.SwapOutPagesPerSec != 4 {
		t.Fatalf("swap rates in=%f out=%f", second.SwapInPagesPerSec, second.SwapOutPagesPerSec)
	}
	if second.MajorFaultsPerSec != 40 {
		t.Fatalf("major faults=%f", second.MajorFaultsPerSec)
	}
	if len(second.TopProcesses) != 1 || second.TopProcesses[0].PID != 123 {
		t.Fatalf("top processes=%+v", second.TopProcesses)
	}
	if second.TopProcesses[0].CPUPercent < 39.9 || second.TopProcesses[0].CPUPercent > 40.1 {
		t.Fatalf("process cpu=%f", second.TopProcesses[0].CPUPercent)
	}

	findings := Diagnose(second)
	want := map[string]bool{
		"memory-pressure": false,
		"swap-churn":      false,
		"storage-wait":    false,
		"major-faults":    false,
	}
	for _, f := range findings {
		if _, ok := want[f.Code]; ok {
			want[f.Code] = true
		}
	}
	for code, seen := range want {
		if !seen {
			t.Fatalf("missing finding %s in %+v", code, findings)
		}
	}
}

func TestDiagnoseCPUAndBlockedLoadFindings(t *testing.T) {
	cases := []struct {
		name   string
		sample Sample
		want   string
	}{
		{
			name:   "cpu saturation",
			sample: Sample{CPUCores: 2, CPUPercent: 95, Load1: 2.0},
			want:   "cpu-saturation",
		},
		{
			name:   "blocked load",
			sample: Sample{CPUCores: 2, CPUPercent: 50, Load1: 4.0},
			want:   "blocked-load",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := Diagnose(tc.sample)
			for _, finding := range findings {
				if finding.Code == tc.want {
					return
				}
			}
			t.Fatalf("missing finding %q in %+v", tc.want, findings)
		})
	}
}

func TestRAIDArrayEvidenceAndDegradedFinding(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, 100, 200, 10, 20, 100, 1000, 2000, 20, 10)
	mustWrite(t, filepath.Join(root, "mdstat"), `Personalities : [raid1]
md2 : active raid1 sda5[0] sdb5[1](F)
      3906885440 blocks super 1.2 [2/1] [U_]
      [=======>.............]  recovery = 42.3% (1650000000/3906885440) finish=120.0min speed=312000K/sec

unused devices: <none>
`)

	s := NewSampler(root)
	sample, err := s.Sample(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(sample.RAIDArrays) != 1 {
		t.Fatalf("raid arrays=%+v", sample.RAIDArrays)
	}
	array := sample.RAIDArrays[0]
	if array.Name != "md2" || array.State != "active" || array.Level != "raid1" {
		t.Fatalf("raid identity=%+v", array)
	}
	if array.RaidDevices != 2 || array.ActiveDevices != 1 || array.Health != "U_" {
		t.Fatalf("raid health=%+v", array)
	}
	if array.SyncAction != "recovery" || array.SyncProgressPercent < 42.29 || array.SyncProgressPercent > 42.31 {
		t.Fatalf("raid sync=%+v", array)
	}

	findings := Diagnose(sample)
	for _, finding := range findings {
		if finding.Code == "raid-degraded" {
			return
		}
	}
	t.Fatalf("missing raid-degraded finding in %+v", findings)
}

func TestHealthyRAIDArrayDoesNotWarn(t *testing.T) {
	findings := Diagnose(Sample{RAIDArrays: []RAIDArraySample{{
		Name:          "md0",
		State:         "active",
		Level:         "raid1",
		RaidDevices:   2,
		ActiveDevices: 2,
		Health:        "UU",
	}}})
	for _, finding := range findings {
		if finding.Code == "raid-degraded" {
			t.Fatalf("unexpected degraded finding: %+v", finding)
		}
	}
}

func TestPhysicalDiskLatencyUtilizationAndQueue(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, 100, 200, 10, 20, 100, 1000, 2000, 20, 10)
	mustWrite(t, filepath.Join(root, "diskstats"), "8 0 sda 10 0 100 1000 20 0 200 2000 0 3000 4000\n")

	s := NewSampler(root)
	t0 := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	if _, err := s.Sample(t0); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, root, 120, 260, 50, 40, 300, 5000, 8000, 30, 20)
	mustWrite(t, filepath.Join(root, "diskstats"), "8 0 sda 12 0 120 1600 26 0 260 2600 0 6500 10000\n")
	second, err := s.Sample(t0.Add(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.PhysicalDisks) != 1 {
		t.Fatalf("physical disks=%+v", second.PhysicalDisks)
	}
	d := second.PhysicalDisks[0]
	if d.Name != "sda" || d.ReadIOPS != 0.4 || d.WriteIOPS != 1.2 {
		t.Fatalf("disk identity/iops=%+v", d)
	}
	if d.AwaitMS < 149.9 || d.AwaitMS > 150.1 {
		t.Fatalf("await_ms=%f", d.AwaitMS)
	}
	if d.UtilizationPercent < 69.9 || d.UtilizationPercent > 70.1 {
		t.Fatalf("utilization_percent=%f", d.UtilizationPercent)
	}
	if d.AvgQueueDepth < 1.19 || d.AvgQueueDepth > 1.21 {
		t.Fatalf("avg_queue_depth=%f", d.AvgQueueDepth)
	}
}

func writeFixture(t *testing.T, root string, readSectors, writeSectors, swapIn, swapOut, majorFault, readBytes, writeBytes, procUser, procSystem uint64) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "loadavg"), "4.00 2.00 1.00 3/100 12345\n")
	mustWrite(t, filepath.Join(root, "meminfo"), "MemTotal:       100000 kB\nMemAvailable:     8000 kB\nSwapTotal:        50000 kB\nSwapFree:         25000 kB\n")
	cpuUser := uint64(100)
	idle := uint64(850)
	iowait := uint64(0)
	if readSectors > 100 {
		cpuUser = 150
		idle = 880
		iowait = 20
	}
	stat := "cpu  " + u(cpuUser) + " 0 50 " + u(idle) + " " + u(iowait) + " 0 0 0 0 0\n" +
		"cpu0 50 0 25 425 0 0 0 0 0 0\n" +
		"cpu1 50 0 25 425 0 0 0 0 0 0\n"
	mustWrite(t, filepath.Join(root, "stat"), stat)
	disk := "8 0 sda 10 0 " + u(readSectors) + " 0 20 0 " + u(writeSectors) + " 0 0 0 0\n"
	mustWrite(t, filepath.Join(root, "diskstats"), disk)
	vm := "pswpin " + u(swapIn) + "\npswpout " + u(swapOut) + "\npgmajfault " + u(majorFault) + "\n"
	mustWrite(t, filepath.Join(root, "vmstat"), vm)

	pid := filepath.Join(root, "123")
	if err := os.MkdirAll(pid, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(pid, "status"), "Name:\tplex\nVmRSS:\t12000 kB\n")
	procStat := "123 (plex) S 1 2 3 4 5 6 7 8 9 10 " + u(procUser) + " " + u(procSystem) + " 0 0 0 0 0\n"
	mustWrite(t, filepath.Join(pid, "stat"), procStat)
	mustWrite(t, filepath.Join(pid, "io"), "read_bytes: "+u(readBytes)+"\nwrite_bytes: "+u(writeBytes)+"\n")
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func u(v uint64) string {
	return fmtUint(v)
}

func fmtUint(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
