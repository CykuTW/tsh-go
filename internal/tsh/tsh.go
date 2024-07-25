package tsh

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pel "tsh-go/internal/rsh"

	"golang.org/x/crypto/ssh/terminal"
)

func Run() {
	flagset := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage: ./%s uuid\n", flagset.Name())
		flagset.PrintDefaults()
	}
	flagset.Parse(os.Args[1:])

	args := flagset.Args()
	var uuid, command string
	var mode uint8

	if len(args) == 0 {
		os.Exit(0)
	}

	uuid = args[0]
	args = args[1:]
	use_ps1 := true

	command = "exec bash --login"
	switch {
	case len(args) == 0:
		mode = pel.RunShell
	default:
		mode = pel.RunShell
		command = args[0]
		use_ps1 = false
	}

	layer, err := pel.Dial(uuid, pel.PEL_SECRET, false)
	if err != nil {
		fmt.Printf("Authentication failed: %v\n", err)
		os.Exit(0)
	}
	defer layer.Close()
	layer.Write([]byte{mode})

	handleRunShell(layer, command, use_ps1)
}

func handleRunShell(layer *pel.PktEncLayer, command string, use_ps1 bool) {
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}

	defer func() {
		_ = terminal.Restore(int(os.Stdin.Fd()), oldState)
		_ = recover()
	}()

	term := os.Getenv("TERM")
	if term == "" {
		term = "vt100"
	}
	_, err = layer.Write([]byte(term))
	if err != nil {
		return
	}

	ws_col, ws_row, _ := terminal.GetSize(int(os.Stdout.Fd()))
	ws := make([]byte, 4)
	ws[0] = byte((ws_row >> 8) & 0xFF)
	ws[1] = byte((ws_row) & 0xFF)
	ws[2] = byte((ws_col >> 8) & 0xFF)
	ws[3] = byte((ws_col) & 0xFF)
	_, err = layer.Write(ws)
	if err != nil {
		return
	}

	_, err = layer.Write([]byte(command))
	if err != nil {
		return
	}

	if use_ps1 {
		_, err = layer.Write([]byte(" export PS1=\"[\\u@`cat /etc/salt/minion_id` \\W]\\\\$ \"\n"))
		rd := make([]byte, 1024)
		_, err = layer.Read(rd)
	}

	buffer := make([]byte, pel.Bufsize)
	buffer2 := make([]byte, pel.Bufsize)
	go func() {
		_, _ = io.CopyBuffer(layer, os.Stdin, buffer2)
	}()
	_, _ = io.CopyBuffer(os.Stdout, layer, buffer)
	layer.Close()
}
