package procmap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

const helperTimeout = 750 * time.Millisecond

func ResolveBestEffort(procRoot, helperPath string, wanted map[uint64]struct{}) map[uint64]model.ProcessInfo {
	out := Resolve(procRoot, wanted)
	if len(out) == len(wanted) || procRoot != "/proc" || helperPath == "" {
		return out
	}
	missing := make(map[uint64]struct{}, len(wanted)-len(out))
	for inode := range wanted {
		if _, ok := out[inode]; !ok {
			missing[inode] = struct{}{}
		}
	}
	if len(missing) == 0 {
		return out
	}
	resolved, err := ResolveWithHelper(helperPath, missing)
	if err != nil {
		return out
	}
	for inode, info := range resolved {
		if _, ok := missing[inode]; ok {
			out[inode] = info
		}
	}
	return out
}

func ResolveWithHelper(helperPath string, wanted map[uint64]struct{}) (map[uint64]model.ProcessInfo, error) {
	if len(wanted) == 0 {
		return map[uint64]model.ProcessInfo{}, nil
	}
	st, err := os.Stat(helperPath)
	if err != nil {
		return nil, err
	}
	if st.IsDir() || st.Mode()&0111 == 0 {
		return nil, errors.New("procmap helper is not executable")
	}
	inodes := make([]uint64, 0, len(wanted))
	for inode := range wanted {
		inodes = append(inodes, inode)
	}
	payload, err := json.Marshal(inodes)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, helperPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	result := make(map[uint64]model.ProcessInfo)
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, err
	}
	return result, nil
}
