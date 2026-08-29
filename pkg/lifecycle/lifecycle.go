// Package lifecycle coordinates restarting the running process.
//
// Module changes take effect at startup, so an admin who enables one needs the
// process to come back. Rather than shelling out to systemctl -- which the
// service user deliberately has no privileges for -- the process re-executes
// itself. That works whether it is supervised or run directly, and picks up a
// replaced binary as a side effect.
package lifecycle

import (
	"os"
	"syscall"
)

// restart is buffered so a request never blocks its HTTP handler, and extra
// requests during a shutdown are dropped rather than queued.
var restart = make(chan struct{}, 1)

// RequestRestart asks the process to restart. Safe to call more than once.
func RequestRestart() {
	select {
	case restart <- struct{}{}:
	default:
	}
}

// Restart returns the channel signalled by RequestRestart.
func Restart() <-chan struct{} {
	return restart
}

// Exec replaces the current process with a fresh copy of the same binary,
// keeping the arguments and environment. It only returns on failure -- on
// success this process image is gone.
//
// Listening sockets are closed automatically: Go opens them with CLOEXEC, so
// the new image can bind the same port.
func Exec() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}
