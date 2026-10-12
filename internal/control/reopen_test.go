package control

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestReopenCannotDeleteLiveOrForeignFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "socket")
	l, err := net.Listen("unix", p)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	if ReopenOwnSocket(p) == nil {
		t.Fatal("live socket removed")
	}
	l.Close()
	if ReopenOwnSocket(p) != nil {
		t.Fatal("dead own socket not reopened")
	}
	os.WriteFile(p, []byte("foreign"), 0600)
	if ReopenOwnSocket(p) == nil {
		t.Fatal("regular file removed")
	}
	os.Remove(p)
	os.Symlink("foreign", p)
	if ReopenOwnSocket(p) == nil {
		t.Fatal("alias removed")
	}
}
