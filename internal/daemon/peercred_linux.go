//go:build linux

package daemon

import (
	"net"

	"golang.org/x/sys/unix"
)

// getPeerCredentials extracts peer credentials from a Unix socket connection on Linux.
func (s *Server) getPeerCredentials(conn net.Conn) *PeerCredentials {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil
	}

	var creds *PeerCredentials
	rawConn.Control(func(fd uintptr) {
		ucred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			return
		}
		creds = &PeerCredentials{
			UID: ucred.Uid,
			GID: ucred.Gid,
			PID: ucred.Pid,
		}
	})

	return creds
}
