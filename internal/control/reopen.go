package control

import (
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

// ReopenOwnSocket only removes a dead same-owner socket in the opt-in Kubernetes layout.
func ReopenOwnSocket(path string) error {
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || before.Mode()&os.ModeSocket == 0 {
		return errors.New("unsafe_control_path")
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) || owner.Gid != uint32(os.Getegid()) {
		return errors.New("foreign_control_owner")
	}
	connection, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		connection.Close()
		return errors.New("control_already_live")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return errors.New("control_state_uncertain")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return errors.New("control_inode_changed")
	}
	return os.Remove(path)
}
