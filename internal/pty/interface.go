package pty

import "io"

type PtyWrapper interface {
	StdIn() io.Writer
	StdOut() io.Reader
	Close()
}
