package procnet

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thebrazenbeard/tattler/internal/model"
)

var tcpStates = map[string]string{
	"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV", "04": "FIN_WAIT1",
	"05": "FIN_WAIT2", "06": "TIME_WAIT", "07": "CLOSE", "08": "CLOSE_WAIT",
	"09": "LAST_ACK", "0A": "LISTEN", "0B": "CLOSING", "0C": "NEW_SYN_RECV",
}

type Snapshot struct {
	Connections []model.Connection
	Listeners   []model.Connection
}

func Read(procRoot string) (Snapshot, error) {
	specs := []struct {
		file, proto string
		v6          bool
	}{
		{"tcp", "tcp", false}, {"tcp6", "tcp6", true}, {"udp", "udp", false}, {"udp6", "udp6", true},
	}
	var out Snapshot
	var errs []error
	for _, s := range specs {
		rows, err := readTable(filepath.Join(procRoot, "net", s.file), s.proto, s.v6)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		for _, c := range rows {
			if c.Kind == model.ObservationTCPListener {
				out.Listeners = append(out.Listeners, c)
				continue
			}
			if c.Kind == model.ObservationUDPEndpoint ||
				(c.Remote.Port() != 0 && !c.Remote.Addr().IsUnspecified()) {
				out.Connections = append(out.Connections, c)
			}
		}
	}
	return out, errors.Join(errs...)
}

func readTable(path, proto string, v6 bool) ([]model.Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var result []model.Connection
	s := bufio.NewScanner(f)
	first := true
	for s.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(s.Text())
		if len(fields) < 10 {
			continue
		}
		local, err1 := parseAddrPort(fields[1], v6)
		remote, err2 := parseAddrPort(fields[2], v6)
		if err1 != nil || err2 != nil {
			continue
		}
		uid64, _ := strconv.ParseUint(fields[7], 10, 32)
		inode, _ := strconv.ParseUint(fields[9], 10, 64)
		state := fields[3]
		kind := model.ObservationUDPEndpoint
		if strings.HasPrefix(proto, "udp") && remote.Port() != 0 && remote.Addr().IsValid() && !remote.Addr().IsUnspecified() {
			kind = model.ObservationUDPFlow
		}
		if strings.HasPrefix(proto, "tcp") {
			if v, ok := tcpStates[state]; ok {
				state = v
			}
			kind = model.ObservationTCPSession
			if state == "LISTEN" {
				kind = model.ObservationTCPListener
			}
		}
		result = append(result, model.Connection{
			Kind: kind, Protocol: proto, Local: local, Remote: remote, State: state,
			Inode: inode, UID: uint32(uid64),
		})
	}
	return result, s.Err()
}

func parseAddrPort(raw string, v6 bool) (netip.AddrPort, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return netip.AddrPort{}, fmt.Errorf("bad endpoint %q", raw)
	}
	b, err := hex.DecodeString(parts[0])
	if err != nil {
		return netip.AddrPort{}, err
	}
	if (!v6 && len(b) != 4) || (v6 && len(b) != 16) {
		return netip.AddrPort{}, fmt.Errorf("bad address width")
	}
	if v6 {
		for i := 0; i < 16; i += 4 {
			reverse(b[i : i+4])
		}
	} else {
		reverse(b)
	}
	addr, ok := netip.AddrFromSlice(b)
	if !ok {
		return netip.AddrPort{}, fmt.Errorf("invalid address")
	}
	p, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(p)), nil
}

func reverse(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
