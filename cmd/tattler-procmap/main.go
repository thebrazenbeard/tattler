package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/thebrazenbeard/tattler/internal/procmap"
)

func main() {
	var inodes []uint64
	if err := json.NewDecoder(os.Stdin).Decode(&inodes); err != nil {
		fmt.Fprintln(os.Stderr, "decode inodes:", err)
		os.Exit(2)
	}
	wanted := make(map[uint64]struct{}, len(inodes))
	for _, inode := range inodes {
		if inode != 0 {
			wanted[inode] = struct{}{}
		}
	}
	result := procmap.Resolve("/proc", wanted)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(3)
	}
}
