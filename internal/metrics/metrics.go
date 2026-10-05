package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ProcessSample struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSKB      uint64  `json:"rss_kb"`
	ReadBPS    float64 `json:"read_bytes_per_sec"`
	WriteBPS   float64 `json:"write_bytes_per_sec"`
}

type DiskSample struct {
	Name               string  `json:"name"`
	ReadBPS            float64 `json:"read_bytes_per_sec"`
	WriteBPS           float64 `json:"write_bytes_per_sec"`
	ReadIOPS           float64 `json:"read_iops"`
	WriteIOPS          float64 `json:"write_iops"`
	AwaitMS            float64 `json:"await_ms"`
	UtilizationPercent float64 `json:"utilization_percent"`
	AvgQueueDepth      float64 `json:"avg_queue_depth"`
}

type RAIDArraySample struct {
	Name                string  `json:"name"`
	State               string  `json:"state"`
	Level               string  `json:"level,omitempty"`
	RaidDevices         int     `json:"raid_devices"`
	ActiveDevices       int     `json:"active_devices"`
	Health              string  `json:"health,omitempty"`
	SyncAction          string  `json:"sync_action,omitempty"`
	SyncProgressPercent float64 `json:"sync_progress_percent"`
}

type Sample struct {
	SchemaVersion      int               `json:"schema_version"`
	ObservedAt         time.Time         `json:"observed_at"`
	Load1              float64           `json:"load1"`
	Load5              float64           `json:"load5"`
	Load15             float64           `json:"load15"`
	CPUCores           int               `json:"cpu_cores"`
	CPUPercent         float64           `json:"cpu_percent"`
	IOWaitPercent      float64           `json:"io_wait_percent"`
	MemTotalKB         uint64            `json:"mem_total_kb"`
	MemAvailableKB     uint64            `json:"mem_available_kb"`
	SwapTotalKB        uint64            `json:"swap_total_kb"`
	SwapUsedKB         uint64            `json:"swap_used_kb"`
	SwapInPagesPerSec  float64           `json:"swap_in_pages_per_sec"`
	SwapOutPagesPerSec float64           `json:"swap_out_pages_per_sec"`
	MajorFaultsPerSec  float64           `json:"major_faults_per_sec"`
	DiskReadBPS        float64           `json:"disk_read_bytes_per_sec"`
	DiskWriteBPS       float64           `json:"disk_write_bytes_per_sec"`
	PhysicalDisks      []DiskSample      `json:"physical_disks,omitempty"`
	RAIDArrays         []RAIDArraySample `json:"raid_arrays,omitempty"`
	ProcessesRunning   int               `json:"processes_running"`
	TopProcesses       []ProcessSample   `json:"top_processes,omitempty"`
}

type Finding struct {
	Code       string    `json:"code"`
	Severity   string    `json:"severity"`
	ObservedAt time.Time `json:"observed_at"`
	Summary    string    `json:"summary"`
	Evidence   string    `json:"evidence"`
}

type cpuCounters struct {
	total  uint64
	idle   uint64
	iowait uint64
	cores  int
}

type diskDeviceCounters struct {
	reads            uint64
	readSectors      uint64
	readMillis       uint64
	writes           uint64
	writeSectors     uint64
	writeMillis      uint64
	ioMillis         uint64
	weightedIOMillis uint64
}

type diskCounters struct {
	readSectors  uint64
	writeSectors uint64
	devices      map[string]diskDeviceCounters
}

type vmCounters struct {
	swapIn     uint64
	swapOut    uint64
	majorFault uint64
}

type processCounters struct {
	cpuTicks uint64
	read     uint64
	write    uint64
}

type Sampler struct {
	ProcRoot string
	prevAt   time.Time
	prevCPU  cpuCounters
	prevDisk diskCounters
	prevVM   vmCounters
	prevProc map[int]processCounters
}

func NewSampler(procRoot string) *Sampler {
	return &Sampler{ProcRoot: procRoot, prevProc: make(map[int]processCounters)}
}

func (s *Sampler) Sample(now time.Time) (Sample, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	var out Sample
	out.SchemaVersion = 1
	out.ObservedAt = now

	var errs []error
	if err := readLoad(filepath.Join(s.ProcRoot, "loadavg"), &out); err != nil {
		errs = append(errs, err)
	}
	if err := readMemory(filepath.Join(s.ProcRoot, "meminfo"), &out); err != nil {
		errs = append(errs, err)
	}

	cpu, err := readCPU(filepath.Join(s.ProcRoot, "stat"))
	if err != nil {
		errs = append(errs, err)
	} else {
		out.CPUCores = cpu.cores
		if s.prevCPU.total != 0 && cpu.total > s.prevCPU.total {
			dt := cpu.total - s.prevCPU.total
			didle := cpu.idle - s.prevCPU.idle
			diowait := cpu.iowait - s.prevCPU.iowait
			busy := dt
			if didle+diowait <= dt {
				busy = dt - didle - diowait
			}
			out.CPUPercent = pct(busy, dt)
			out.IOWaitPercent = pct(diowait, dt)
		}
	}

	disk, err := readDisk(filepath.Join(s.ProcRoot, "diskstats"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	vm, err := readVM(filepath.Join(s.ProcRoot, "vmstat"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	raid, err := readRAID(filepath.Join(s.ProcRoot, "mdstat"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	out.RAIDArrays = raid

	elapsed := 0.0
	if !s.prevAt.IsZero() {
		elapsed = now.Sub(s.prevAt).Seconds()
	}
	if elapsed > 0 {
		if disk.readSectors >= s.prevDisk.readSectors {
			out.DiskReadBPS = float64(disk.readSectors-s.prevDisk.readSectors) * 512 / elapsed
		}
		if disk.writeSectors >= s.prevDisk.writeSectors {
			out.DiskWriteBPS = float64(disk.writeSectors-s.prevDisk.writeSectors) * 512 / elapsed
		}
		out.PhysicalDisks = diskRates(s.prevDisk, disk, elapsed)
		if vm.swapIn >= s.prevVM.swapIn {
			out.SwapInPagesPerSec = float64(vm.swapIn-s.prevVM.swapIn) / elapsed
		}
		if vm.swapOut >= s.prevVM.swapOut {
			out.SwapOutPagesPerSec = float64(vm.swapOut-s.prevVM.swapOut) / elapsed
		}
		if vm.majorFault >= s.prevVM.majorFault {
			out.MajorFaultsPerSec = float64(vm.majorFault-s.prevVM.majorFault) / elapsed
		}
	}

	procs, nextProc := readProcesses(s.ProcRoot, s.prevProc, cpu, s.prevCPU, elapsed)
	out.TopProcesses = procs

	s.prevAt, s.prevCPU, s.prevDisk, s.prevVM, s.prevProc = now, cpu, disk, vm, nextProc
	return out, errors.Join(errs...)
}

func readLoad(path string, out *Sample) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 4 {
		return fmt.Errorf("loadavg: short record")
	}
	out.Load1, _ = strconv.ParseFloat(fields[0], 64)
	out.Load5, _ = strconv.ParseFloat(fields[1], 64)
	out.Load15, _ = strconv.ParseFloat(fields[2], 64)
	run := strings.Split(fields[3], "/")
	if len(run) > 0 {
		out.ProcessesRunning, _ = strconv.Atoi(run[0])
	}
	return nil
}

func readMemory(path string, out *Sample) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			vals[key] = v
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	out.MemTotalKB = vals["MemTotal"]
	out.MemAvailableKB = vals["MemAvailable"]
	if out.MemAvailableKB == 0 {
		out.MemAvailableKB = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	out.SwapTotalKB = vals["SwapTotal"]
	if vals["SwapFree"] <= out.SwapTotalKB {
		out.SwapUsedKB = out.SwapTotalKB - vals["SwapFree"]
	}
	return nil
}

func readCPU(path string) (cpuCounters, error) {
	f, err := os.Open(path)
	if err != nil {
		return cpuCounters{}, err
	}
	defer f.Close()
	var out cpuCounters
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:]
			var vals []uint64
			for _, raw := range fields {
				v, _ := strconv.ParseUint(raw, 10, 64)
				vals = append(vals, v)
			}
			for _, v := range vals {
				out.total += v
			}
			if len(vals) > 3 {
				out.idle = vals[3]
			}
			if len(vals) > 4 {
				out.iowait = vals[4]
			}
		} else if len(line) > 3 && strings.HasPrefix(line, "cpu") && line[3] >= '0' && line[3] <= '9' {
			out.cores++
		}
	}
	return out, sc.Err()
}

func readDisk(path string) (diskCounters, error) {
	f, err := os.Open(path)
	if err != nil {
		return diskCounters{}, err
	}
	defer f.Close()
	out := diskCounters{devices: make(map[string]diskDeviceCounters)}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 14 || !physicalDisk(fields[2]) {
			continue
		}
		parse := func(index int) uint64 {
			v, _ := strconv.ParseUint(fields[index], 10, 64)
			return v
		}
		device := diskDeviceCounters{
			reads:            parse(3),
			readSectors:      parse(5),
			readMillis:       parse(6),
			writes:           parse(7),
			writeSectors:     parse(9),
			writeMillis:      parse(10),
			ioMillis:         parse(12),
			weightedIOMillis: parse(13),
		}
		out.devices[fields[2]] = device
		out.readSectors += device.readSectors
		out.writeSectors += device.writeSectors
	}
	return out, sc.Err()
}

func diskRates(prev, current diskCounters, elapsed float64) []DiskSample {
	if elapsed <= 0 || len(current.devices) == 0 {
		return nil
	}
	names := make([]string, 0, len(current.devices))
	for name := range current.devices {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]DiskSample, 0, len(names))
	for _, name := range names {
		cur := current.devices[name]
		old, ok := prev.devices[name]
		if !ok ||
			cur.reads < old.reads ||
			cur.readSectors < old.readSectors ||
			cur.readMillis < old.readMillis ||
			cur.writes < old.writes ||
			cur.writeSectors < old.writeSectors ||
			cur.writeMillis < old.writeMillis ||
			cur.ioMillis < old.ioMillis ||
			cur.weightedIOMillis < old.weightedIOMillis {
			continue
		}
		reads := cur.reads - old.reads
		writes := cur.writes - old.writes
		readMillis := cur.readMillis - old.readMillis
		writeMillis := cur.writeMillis - old.writeMillis
		ioMillis := cur.ioMillis - old.ioMillis
		weightedIOMillis := cur.weightedIOMillis - old.weightedIOMillis
		ops := reads + writes
		d := DiskSample{
			Name:          name,
			ReadBPS:       float64(cur.readSectors-old.readSectors) * 512 / elapsed,
			WriteBPS:      float64(cur.writeSectors-old.writeSectors) * 512 / elapsed,
			ReadIOPS:      float64(reads) / elapsed,
			WriteIOPS:     float64(writes) / elapsed,
			AvgQueueDepth: float64(weightedIOMillis) / (elapsed * 1000),
		}
		if ops > 0 {
			d.AwaitMS = float64(readMillis+writeMillis) / float64(ops)
		}
		d.UtilizationPercent = float64(ioMillis) / (elapsed * 1000) * 100
		if d.UtilizationPercent > 100 {
			d.UtilizationPercent = 100
		}
		out = append(out, d)
	}
	return out
}

func hottestDisk(disks []DiskSample) (DiskSample, bool) {
	if len(disks) == 0 {
		return DiskSample{}, false
	}
	hottest := disks[0]
	for _, disk := range disks[1:] {
		if disk.UtilizationPercent > hottest.UtilizationPercent ||
			(disk.UtilizationPercent == hottest.UtilizationPercent && disk.AwaitMS > hottest.AwaitMS) {
			hottest = disk
		}
	}
	return hottest, true
}

func physicalDisk(name string) bool {
	if strings.HasPrefix(name, "sd") && len(name) >= 3 {
		for _, r := range name[2:] {
			if r < 'a' || r > 'z' {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(name, "hd") && len(name) >= 3 {
		for _, r := range name[2:] {
			if r < 'a' || r > 'z' {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(name, "vd") && len(name) >= 3 {
		for _, r := range name[2:] {
			if r < 'a' || r > 'z' {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(name, "nvme") && strings.Contains(name, "n") && !strings.Contains(name, "p") {
		return true
	}
	return false
}

var (
	mdHealthRE = regexp.MustCompile(`\[(\d+)/(\d+)\]\s+\[([U_]+)\]`)
	mdSyncRE   = regexp.MustCompile(`\b(resync|recovery|reshape|check)\s*=\s*([0-9]+(?:\.[0-9]+)?)%`)
)

func readRAID(path string) ([]RAIDArraySample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []RAIDArraySample
	current := -1
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Personalities") || strings.HasPrefix(trimmed, "unused devices") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.Contains(line, " : ") {
			parts := strings.SplitN(line, " : ", 2)
			fields := strings.Fields(parts[1])
			if len(fields) == 0 {
				current = -1
				continue
			}
			array := RAIDArraySample{Name: strings.TrimSpace(parts[0]), State: fields[0]}
			if len(fields) > 1 && raidLevel(fields[1]) {
				array.Level = fields[1]
			}
			out = append(out, array)
			current = len(out) - 1
			continue
		}
		if current < 0 {
			continue
		}
		if match := mdHealthRE.FindStringSubmatch(trimmed); len(match) == 4 {
			out[current].RaidDevices, _ = strconv.Atoi(match[1])
			out[current].ActiveDevices, _ = strconv.Atoi(match[2])
			out[current].Health = match[3]
		}
		if match := mdSyncRE.FindStringSubmatch(trimmed); len(match) == 3 {
			out[current].SyncAction = match[1]
			out[current].SyncProgressPercent, _ = strconv.ParseFloat(match[2], 64)
		}
	}
	return out, sc.Err()
}

func raidLevel(raw string) bool {
	if strings.HasPrefix(raw, "raid") {
		return true
	}
	switch raw {
	case "linear", "multipath", "faulty":
		return true
	default:
		return false
	}
}

func readVM(path string) (vmCounters, error) {
	f, err := os.Open(path)
	if err != nil {
		return vmCounters{}, err
	}
	defer f.Close()
	var out vmCounters
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "pswpin":
			out.swapIn = v
		case "pswpout":
			out.swapOut = v
		case "pgmajfault":
			out.majorFault = v
		}
	}
	return out, sc.Err()
}

func readProcesses(procRoot string, prev map[int]processCounters, cpu, prevCPU cpuCounters, elapsed float64) ([]ProcessSample, map[int]processCounters) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, map[int]processCounters{}
	}
	next := make(map[int]processCounters)
	var out []ProcessSample
	totalDelta := uint64(0)
	if cpu.total >= prevCPU.total {
		totalDelta = cpu.total - prevCPU.total
	}
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 0 || !ent.IsDir() {
			continue
		}
		name, rss := readProcessStatus(filepath.Join(procRoot, ent.Name(), "status"))
		ticks := readProcessTicks(filepath.Join(procRoot, ent.Name(), "stat"))
		rb, wb := readProcessIO(filepath.Join(procRoot, ent.Name(), "io"))
		next[pid] = processCounters{cpuTicks: ticks, read: rb, write: wb}
		ps := ProcessSample{PID: pid, Name: name, RSSKB: rss}
		if old, ok := prev[pid]; ok && totalDelta > 0 && ticks >= old.cpuTicks {
			cores := cpu.cores
			if cores < 1 {
				cores = 1
			}
			ps.CPUPercent = float64(ticks-old.cpuTicks) / float64(totalDelta) * float64(cores) * 100
			if elapsed > 0 {
				if rb >= old.read {
					ps.ReadBPS = float64(rb-old.read) / elapsed
				}
				if wb >= old.write {
					ps.WriteBPS = float64(wb-old.write) / elapsed
				}
			}
		}
		if ps.CPUPercent > 0 || ps.RSSKB > 0 || ps.ReadBPS > 0 || ps.WriteBPS > 0 {
			out = append(out, ps)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ai := out[i].CPUPercent + float64(out[i].RSSKB)/1024 + (out[i].ReadBPS+out[i].WriteBPS)/(1024*1024)
		aj := out[j].CPUPercent + float64(out[j].RSSKB)/1024 + (out[j].ReadBPS+out[j].WriteBPS)/(1024*1024)
		return ai > aj
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out, next
}

func readProcessStatus(path string) (string, uint64) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	var name string
	var rss uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "Name":
			name = fields[1]
		case "VmRSS":
			rss, _ = strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return name, rss
}

func readProcessTicks(path string) uint64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	line := string(b)
	end := strings.LastIndex(line, ")")
	if end < 0 || end+2 >= len(line) {
		return 0
	}
	fields := strings.Fields(line[end+2:])
	if len(fields) <= 12 {
		return 0
	}
	user, _ := strconv.ParseUint(fields[11], 10, 64)
	system, _ := strconv.ParseUint(fields[12], 10, 64)
	return user + system
}

func readProcessIO(path string) (uint64, uint64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var read, write uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "read_bytes":
			read = v
		case "write_bytes":
			write = v
		}
	}
	return read, write
}

func pct(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func Diagnose(s Sample) []Finding {
	var out []Finding
	memPct := 100.0
	if s.MemTotalKB > 0 {
		memPct = float64(s.MemAvailableKB) * 100 / float64(s.MemTotalKB)
	}
	if memPct < 5 {
		out = append(out, Finding{Code: "memory-critical", Severity: "critical", ObservedAt: s.ObservedAt, Summary: "Very little memory is available.", Evidence: fmt.Sprintf("MemAvailable %.1f%% (%d/%d KiB)", memPct, s.MemAvailableKB, s.MemTotalKB)})
	} else if memPct < 10 {
		out = append(out, Finding{Code: "memory-pressure", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "Memory headroom is low.", Evidence: fmt.Sprintf("MemAvailable %.1f%% (%d/%d KiB)", memPct, s.MemAvailableKB, s.MemTotalKB)})
	}
	if s.SwapInPagesPerSec+s.SwapOutPagesPerSec >= 8 {
		out = append(out, Finding{Code: "swap-churn", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "The host is actively paging.", Evidence: fmt.Sprintf("swap in %.1f pages/s, swap out %.1f pages/s", s.SwapInPagesPerSec, s.SwapOutPagesPerSec)})
	}
	if s.IOWaitPercent >= 20 {
		evidence := fmt.Sprintf("I/O wait %.1f%%", s.IOWaitPercent)
		if disk, ok := hottestDisk(s.PhysicalDisks); ok {
			evidence += fmt.Sprintf("; %s util %.1f%%, await %.1f ms, queue %.2f", disk.Name, disk.UtilizationPercent, disk.AwaitMS, disk.AvgQueueDepth)
		}
		out = append(out, Finding{Code: "storage-wait", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "CPU time is being lost waiting on storage.", Evidence: evidence})
	}
	if s.CPUPercent >= 90 && s.CPUCores > 0 && s.Load1 >= float64(s.CPUCores)*0.9 {
		out = append(out, Finding{Code: "cpu-saturation", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "CPU capacity is saturated.", Evidence: fmt.Sprintf("CPU %.1f%%, load1 %.2f on %d cores", s.CPUPercent, s.Load1, s.CPUCores)})
	}
	if s.CPUCores > 0 && s.Load1 >= float64(s.CPUCores)*1.5 && s.CPUPercent < 80 {
		out = append(out, Finding{Code: "blocked-load", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "Load is high without equivalent CPU use; blocked or I/O-bound work is likely.", Evidence: fmt.Sprintf("CPU %.1f%%, load1 %.2f on %d cores", s.CPUPercent, s.Load1, s.CPUCores)})
	}
	if s.MajorFaultsPerSec >= 25 {
		out = append(out, Finding{Code: "major-faults", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "Processes are faulting pages from storage at a high rate.", Evidence: fmt.Sprintf("%.1f major faults/s", s.MajorFaultsPerSec)})
	}
	for _, array := range s.RAIDArrays {
		degraded := strings.Contains(array.Health, "_")
		if array.RaidDevices > 0 && array.ActiveDevices < array.RaidDevices {
			degraded = true
		}
		if !degraded {
			continue
		}
		evidence := fmt.Sprintf("%s %s %s %d/%d %s", array.Name, array.State, array.Level, array.ActiveDevices, array.RaidDevices, array.Health)
		if array.SyncAction != "" {
			evidence += fmt.Sprintf("; %s %.1f%%", array.SyncAction, array.SyncProgressPercent)
		}
		out = append(out, Finding{Code: "raid-degraded", Severity: "warning", ObservedAt: s.ObservedAt, Summary: "A Linux MD RAID array is degraded.", Evidence: strings.TrimSpace(evidence)})
	}
	return out
}
