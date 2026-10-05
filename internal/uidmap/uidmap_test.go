package uidmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "passwd")
	data := "root:x:0:0:root:/root:/bin/sh\nPlexMediaServer:x:297536:297536::/var/packages/PlexMediaServer/home:/sbin/nologin\nmalformed\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	got := Load(path)
	if got[0] != "root" {
		t.Fatalf("uid 0=%q", got[0])
	}
	if got[297536] != "PlexMediaServer" {
		t.Fatalf("plex=%q", got[297536])
	}
}
