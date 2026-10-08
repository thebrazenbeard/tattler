//go:build windows

package wincollect

import (
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/thebrazenbeard/tattler/internal/metrics"
	"github.com/thebrazenbeard/tattler/internal/model"
)

const (
	afInet                  = 2
	afInet6                 = 23
	tcpTableOwnerPIDAll     = 5
	udpTableOwnerPID        = 1
	errorInsufficientBuffer = 122
	processQueryLimitedInfo = 0x1000
)

var (
	iphlpapi                = syscall.NewLazyDLL("iphlpapi.dll")
	procGetExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUDPTable = iphlpapi.NewProc("GetExtendedUdpTable")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	procQueryProcessImage   = kernel32.NewProc("QueryFullProcessImageNameW")
	procGlobalMemoryStatus  = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes      = kernel32.NewProc("GetSystemTimes")
)

type Collector struct {
	prevIdle   uint64
	prevKernel uint64
	prevUser   uint64
}

func New() *Collector { return &Collector{} }

func (c *Collector) Connections() ([]model.Connection, error) {
	tcp4, errTCP4 := tcp4Connections()
	tcp6, errTCP6 := tcp6Connections()
	udp4, errUDP4 := udp4Endpoints()
	udp6, errUDP6 := udp6Endpoints()
	out := make([]model.Connection, 0, len(tcp4)+len(tcp6)+len(udp4)+len(udp6))
	out = append(out, tcp4...)
	out = append(out, tcp6...)
	out = append(out, udp4...)
	out = append(out, udp6...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Protocol != out[j].Protocol {
			return out[i].Protocol < out[j].Protocol
		}
		if out[i].Local != out[j].Local {
			return out[i].Local.String() < out[j].Local.String()
		}
		return out[i].Remote.String() < out[j].Remote.String()
	})
	return out, errors.Join(errTCP4, errTCP6, errUDP4, errUDP6)
}

func (c *Collector) Sample(now time.Time) (metrics.Sample, error) {
	if now.IsZero() {
		now = time.Now()
	}
	s := metrics.Sample{
		SchemaVersion: 1,
		ObservedAt:    now.UTC(),
		Platform:      "windows",
		CPUCores:      runtime.NumCPU(),
		UnavailableMetrics: []string{
			"disk_throughput", "io_wait_percent", "load_average",
			"major_faults", "physical_disk_latency", "raid_state", "swap_activity",
		},
	}
	if err := fillMemory(&s); err != nil {
		return s, err
	}
	idle, kernel, user, err := systemTimes()
	if err != nil {
		return s, err
	}
	if c.prevKernel != 0 || c.prevUser != 0 {
		total := (kernel - c.prevKernel) + (user - c.prevUser)
		idleDelta := idle - c.prevIdle
		if total > 0 && total >= idleDelta {
			s.CPUPercent = float64(total-idleDelta) * 100 / float64(total)
		}
	}
	c.prevIdle, c.prevKernel, c.prevUser = idle, kernel, user
	return s, nil
}

type tcp4Row struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}
type tcp6Row struct {
	LocalAddr     [16]byte
	LocalScopeID  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeID uint32
	RemotePort    uint32
	State         uint32
	OwningPID     uint32
}
type udp4Row struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPID uint32
}
type udp6Row struct {
	LocalAddr    [16]byte
	LocalScopeID uint32
	LocalPort    uint32
	OwningPID    uint32
}

func tcp4Connections() ([]model.Connection, error) {
	buf, err := extendedTCPTable(afInet)
	if err != nil {
		return nil, err
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(tcp4Row{})
	offset := alignedOffset(unsafe.Sizeof(uint32(0)), unsafe.Alignof(tcp4Row{}))
	rows := make([]tcp4Row, 0, count)
	for i := uint32(0); i < count; i++ {
		rows = append(rows, *(*tcp4Row)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + offset + uintptr(i)*rowSize)))
	}
	out := make([]model.Connection, 0, count)
	for _, row := range rows {
		kind := model.ObservationTCPSession
		direction := classifyTCP4(row, rows)
		if row.State == 2 {
			kind = model.ObservationTCPListener
			direction = "listen"
		} else if row.RemotePort == 0 {
			continue
		}
		local := netip.AddrPortFrom(ipv4(row.LocalAddr), networkPort(row.LocalPort))
		remote := netip.AddrPortFrom(ipv4(row.RemoteAddr), networkPort(row.RemotePort))
		out = append(out, connection(kind, "tcp", local, remote, row.State, row.OwningPID, direction))
	}
	return out, nil
}

func tcp6Connections() ([]model.Connection, error) {
	buf, err := extendedTCPTable(afInet6)
	if err != nil {
		return nil, err
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(tcp6Row{})
	offset := alignedOffset(unsafe.Sizeof(uint32(0)), unsafe.Alignof(tcp6Row{}))
	rows := make([]tcp6Row, 0, count)
	for i := uint32(0); i < count; i++ {
		rows = append(rows, *(*tcp6Row)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + offset + uintptr(i)*rowSize)))
	}
	out := make([]model.Connection, 0, count)
	for _, row := range rows {
		kind := model.ObservationTCPSession
		direction := classifyTCP6(row, rows)
		if row.State == 2 {
			kind = model.ObservationTCPListener
			direction = "listen"
		} else if row.RemotePort == 0 {
			continue
		}
		local := netip.AddrPortFrom(ipv6WithScope(row.LocalAddr, row.LocalScopeID), networkPort(row.LocalPort))
		remote := netip.AddrPortFrom(ipv6WithScope(row.RemoteAddr, row.RemoteScopeID), networkPort(row.RemotePort))
		out = append(out, connection(kind, "tcp6", local, remote, row.State, row.OwningPID, direction))
	}
	return out, nil
}

func udp4Endpoints() ([]model.Connection, error) {
	buf, err := extendedUDPTable(afInet)
	if err != nil {
		return nil, err
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(udp4Row{})
	offset := alignedOffset(unsafe.Sizeof(uint32(0)), unsafe.Alignof(udp4Row{}))
	out := make([]model.Connection, 0, count)
	for i := uint32(0); i < count; i++ {
		row := *(*udp4Row)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + offset + uintptr(i)*rowSize))
		local := netip.AddrPortFrom(ipv4(row.LocalAddr), networkPort(row.LocalPort))
		remote := netip.AddrPortFrom(netip.IPv4Unspecified(), 0)
		out = append(out, connection(model.ObservationUDPEndpoint, "udp", local, remote, 0, row.OwningPID, ""))
	}
	return out, nil
}

func udp6Endpoints() ([]model.Connection, error) {
	buf, err := extendedUDPTable(afInet6)
	if err != nil {
		return nil, err
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(udp6Row{})
	offset := alignedOffset(unsafe.Sizeof(uint32(0)), unsafe.Alignof(udp6Row{}))
	out := make([]model.Connection, 0, count)
	for i := uint32(0); i < count; i++ {
		row := *(*udp6Row)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + offset + uintptr(i)*rowSize))
		local := netip.AddrPortFrom(ipv6WithScope(row.LocalAddr, row.LocalScopeID), networkPort(row.LocalPort))
		remote := netip.AddrPortFrom(netip.IPv6Unspecified(), 0)
		out = append(out, connection(model.ObservationUDPEndpoint, "udp6", local, remote, 0, row.OwningPID, ""))
	}
	return out, nil
}

func extendedUDPTable(af uintptr) ([]byte, error) {
	var size uint32
	r1, _, _ := procGetExtendedUDPTable.Call(
		0, uintptr(unsafe.Pointer(&size)), 1, af, udpTableOwnerPID, 0,
	)
	if r1 != errorInsufficientBuffer {
		if r1 == 0 && size == 0 {
			return []byte{0, 0, 0, 0}, nil
		}
		return nil, syscall.Errno(r1)
	}
	buf := make([]byte, size)
	r1, _, _ = procGetExtendedUDPTable.Call(
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, af, udpTableOwnerPID, 0,
	)
	if r1 != 0 {
		return nil, syscall.Errno(r1)
	}
	return buf, nil
}

func extendedTCPTable(af uintptr) ([]byte, error) {
	var size uint32
	r1, _, _ := procGetExtendedTCPTable.Call(
		0, uintptr(unsafe.Pointer(&size)), 1, af, tcpTableOwnerPIDAll, 0,
	)
	if r1 != errorInsufficientBuffer {
		if r1 == 0 && size == 0 {
			return []byte{0, 0, 0, 0}, nil
		}
		return nil, syscall.Errno(r1)
	}
	buf := make([]byte, size)
	r1, _, _ = procGetExtendedTCPTable.Call(
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, af, tcpTableOwnerPIDAll, 0,
	)
	if r1 != 0 {
		return nil, syscall.Errno(r1)
	}
	return buf, nil
}

// Keep IPv6 zone/scope IDs from the Windows native table. Omitting them
// merges endpoints on separate scoped interfaces into one tracking identity.
func ipv6WithScope(raw [16]byte, scope uint32) netip.Addr {
	addr := netip.AddrFrom16(raw)
	if scope != 0 {
		addr = addr.WithZone(strconv.FormatUint(uint64(scope), 10))
	}
	return addr
}

func connection(kind, proto string, local, remote netip.AddrPort, state, pid uint32, direction string) model.Connection {
	p := processInfo(pid)
	out := model.Connection{
		Kind:      kind,
		Protocol:  proto,
		Local:     local,
		Remote:    remote,
		Direction: direction,
		Process:   p,
	}
	if kind != model.ObservationUDPEndpoint {
		out.State = tcpState(state)
	}
	return out
}

func classifyTCP4(row tcp4Row, rows []tcp4Row) string {
	for _, listener := range rows {
		if listener.State != 2 || listener.LocalPort != row.LocalPort {
			continue
		}
		if listener.LocalAddr == 0 || listener.LocalAddr == row.LocalAddr {
			return "inbound"
		}
	}
	return "outbound"
}

func classifyTCP6(row tcp6Row, rows []tcp6Row) string {
	for _, listener := range rows {
		if listener.State != 2 || listener.LocalPort != row.LocalPort {
			continue
		}
		if allZero16(listener.LocalAddr) || listener.LocalAddr == row.LocalAddr {
			return "inbound"
		}
	}
	return "outbound"
}

func allZero16(addr [16]byte) bool {
	return addr == [16]byte{}
}

func processInfo(pid uint32) model.ProcessInfo {
	if pid == 0 {
		return model.ProcessInfo{}
	}
	out := model.ProcessInfo{PID: int(pid)}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInfo, 0, uintptr(pid))
	if h == 0 {
		return out
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	ok, _, _ := procQueryProcessImage.Call(
		h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 {
		return out
	}
	out.Exe = syscall.UTF16ToString(buf[:size])
	out.Name = filepath.Base(out.Exe)
	return out
}

func alignedOffset(size, alignment uintptr) uintptr {
	return (size + alignment - 1) &^ (alignment - 1)
}

func networkPort(v uint32) uint16 {
	p := uint16(v)
	return (p&0x00ff)<<8 | (p&0xff00)>>8
}

func ipv4(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func tcpState(v uint32) string {
	switch v {
	case 1:
		return "CLOSED"
	case 2:
		return "LISTEN"
	case 3:
		return "SYN_SENT"
	case 4:
		return "SYN_RECV"
	case 5:
		return "ESTABLISHED"
	case 6:
		return "FIN_WAIT1"
	case 7:
		return "FIN_WAIT2"
	case 8:
		return "CLOSE_WAIT"
	case 9:
		return "CLOSING"
	case 10:
		return "LAST_ACK"
	case 11:
		return "TIME_WAIT"
	case 12:
		return "DELETE_TCB"
	default:
		return fmt.Sprintf("STATE_%d", v)
	}
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func fillMemory(s *metrics.Sample) error {
	m := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ok, _, e := procGlobalMemoryStatus.Call(uintptr(unsafe.Pointer(&m)))
	if ok == 0 {
		if e != syscall.Errno(0) {
			return e
		}
		return errors.New("GlobalMemoryStatusEx failed")
	}
	s.MemTotalKB = m.TotalPhys / 1024
	s.MemAvailableKB = m.AvailPhys / 1024
	return nil
}

func systemTimes() (idle, kernel, user uint64, err error) {
	var idleFT, kernelFT, userFT syscall.Filetime
	ok, _, e := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if ok == 0 {
		if e != syscall.Errno(0) {
			return 0, 0, 0, e
		}
		return 0, 0, 0, errors.New("GetSystemTimes failed")
	}
	toUint64 := func(ft syscall.Filetime) uint64 {
		return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
	}
	return toUint64(idleFT), toUint64(kernelFT), toUint64(userFT), nil
}
