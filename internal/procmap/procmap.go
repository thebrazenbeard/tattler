package procmap

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func Resolve(procRoot string, wanted map[uint64]struct{}) map[uint64]model.ProcessInfo {
	out := make(map[uint64]model.ProcessInfo)
	if len(wanted) == 0 {
		return out
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 0 || !ent.IsDir() {
			continue
		}
		fdDir := filepath.Join(procRoot, ent.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		var hits []uint64
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			raw := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			inode, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				continue
			}
			if _, ok := wanted[inode]; ok {
				hits = append(hits, inode)
			}
		}
		if len(hits) == 0 {
			continue
		}
		info := model.ProcessInfo{PID: pid}
		if b, err := os.ReadFile(filepath.Join(procRoot, ent.Name(), "comm")); err == nil {
			info.Name = strings.TrimSpace(string(b))
		}
		if exe, err := os.Readlink(filepath.Join(procRoot, ent.Name(), "exe")); err == nil {
			info.Exe = exe
		}
		for _, inode := range hits {
			out[inode] = info
		}
		if len(out) == len(wanted) {
			break
		}
	}
	return out
}
