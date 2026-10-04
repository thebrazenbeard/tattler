package uidmap

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Load parses a passwd-format file into UID -> account name.
// Malformed lines are ignored; the first name for a UID wins.
func Load(path string) map[uint32]string {
	out := make(map[uint32]string)
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		raw, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		uid := uint32(raw)
		if _, exists := out[uid]; !exists {
			out[uid] = fields[0]
		}
	}
	return out
}
