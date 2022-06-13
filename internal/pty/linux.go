//go:build !windows
// +build !windows

package pty

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type LinuxPtyWrapper struct {
	ptmx *os.File
}

func (pw LinuxPtyWrapper) StdIn() io.Writer {
	return pw.ptmx
}

func (pw LinuxPtyWrapper) StdOut() io.Reader {
	return pw.ptmx
}

func (pw LinuxPtyWrapper) Close() {
	pw.ptmx.Close()
}

func OpenPty(command, term string, ws_col, ws_row uint32) (PtyWrapper, error) {
	c := exec.Command("/bin/sh", "-c", command)
	c.Env = os.Environ()
	c.Env = append(c.Env, "TERM="+term)
	c.Env = append(c.Env, "HISFILE=")
	ptmx, err := pty.StartWithSize(c, &pty.Winsize{
		Rows: uint16(ws_row),
		Cols: uint16(ws_col),
	})
	if err != nil {
		return nil, err
	}
	return LinuxPtyWrapper{ptmx: ptmx}, nil
}
