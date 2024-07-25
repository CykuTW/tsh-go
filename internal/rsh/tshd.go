package rsh

import (
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

type PtyWrapper interface {
	StdIn() io.Writer
	StdOut() io.Reader
	Close()
}

func Run() {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	addr := "127.0.0.1:60022"
	ln, err := Listen(addr, PEL_SECRET, true)
	if err != nil {
		os.Exit(0)
	}
	for {
		layer, err := ln.Accept()
		if err == nil {
			go handleGeneric(layer)
		}
	}
}

// entry handler,
// automatically close connection after handling
// it's safe to run with goroutine
func handleGeneric(layer *PktEncLayer) {
	defer layer.Close()
	defer func() {
		recover()
	}()
	buffer := make([]byte, 1)
	n, err := layer.Read(buffer)
	if err != nil || n != 1 {
		return
	}
	switch buffer[0] {
	case GetFile:
		handleGetFile(layer)
	case PutFile:
		handlePutFile(layer)
	case RunShell:
		handleRunShell(layer)
	}
}

func handleGetFile(layer *PktEncLayer) {
	buffer := make([]byte, Bufsize)
	n, err := layer.Read(buffer)
	if err != nil {
		return
	}
	filename := string(buffer[:n])
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	io.CopyBuffer(layer, f, buffer)
}

func handlePutFile(layer *PktEncLayer) {
	buffer := make([]byte, Bufsize)
	n, err := layer.Read(buffer)
	if err != nil {
		return
	}
	filename := filepath.FromSlash(string(buffer[:n]))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	io.CopyBuffer(f, layer, buffer)
	layer.Close()
}

func handleRunShell(layer *PktEncLayer) {
	buffer := make([]byte, Bufsize)
	buffer2 := make([]byte, Bufsize)

	n, err := layer.Read(buffer)
	if err != nil {
		return
	}
	term := string(buffer[:n])

	n, err = layer.Read(buffer[:4])
	if err != nil || n != 4 {
		return
	}
	ws_row := int(buffer[0])<<8 + int(buffer[1])
	ws_col := int(buffer[2])<<8 + int(buffer[3])

	n, err = layer.Read(buffer)
	if err != nil {
		return
	}
	command := string(buffer[:n])

	tp, err := OpenPty(command, term, uint32(ws_col), uint32(ws_row))
	if err != nil {
		return
	}
	defer tp.Close()
	go func() {
		io.CopyBuffer(tp.StdIn(), layer, buffer)
		tp.Close()
	}()
	io.CopyBuffer(layer, tp.StdOut(), buffer2)
}
