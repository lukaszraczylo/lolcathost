//go:build darwin

package daemon

import (
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// getPeerCredentials extracts peer credentials from a Unix socket connection on macOS.
// Note: macOS Xucred doesn't include PID, so we use LOCAL_PEERPID separately.
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
		xucred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			return
		}

		// Get PID separately using LOCAL_PEERPID
		var pid int32
		pidLen := uint32(unsafe.Sizeof(pid))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			unix.SOL_LOCAL,
			0x002, // LOCAL_PEERPID
			uintptr(unsafe.Pointer(&pid)),
			uintptr(unsafe.Pointer(&pidLen)),
			0,
		)
		if errno != 0 {
			pid = 0
		}

		creds = &PeerCredentials{
			UID: xucred.Uid,
			GID: xucred.Groups[0],
			PID: pid,
		}
	})

	return creds
}
