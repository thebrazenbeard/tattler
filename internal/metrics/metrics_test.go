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
