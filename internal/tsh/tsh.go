package tsh

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pel "tsh-go/internal/rsh"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh/terminal"
)

func Run() {
	flagset := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage: ./%s uuid\n", flagset.Name())
		fmt.Fprintf(flagset.Output(), "        uuid get <source-file> <dest-dir>\n")
		fmt.Fprintf(flagset.Output(), "        uuid put <source-file> <dest-dir>\n")
		flagset.PrintDefaults()
	}
	flagset.Parse(os.Args[1:])

	args := flagset.Args()
	var uuid, command, srcfile, dstdir string
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
	case args[0] == "get" && len(args) == 3:
		mode = pel.GetFile
		srcfile = args[1]
		dstdir = args[2]
	case args[0] == "put" && len(args) == 3:
		mode = pel.PutFile
		srcfile = args[1]
		dstdir = args[2]
	default:
		mode = pel.RunShell
		command = args[0]
		use_ps1 = false
	}

	layer, err := pel.Dial(uuid, pel.PEL_SECRET, false)
	if err != nil {
		fmt.Printf("边缘设备连接失败: %v\n", err)
		os.Exit(0)
	}
	defer layer.Close()
	layer.Write([]byte{mode})

	switch mode {
	case pel.RunShell:
		handleRunShell(layer, command, use_ps1)
	case pel.GetFile:
		handleGetFile(layer, srcfile, dstdir)
	case pel.PutFile:
		handlePutFile(layer, srcfile, dstdir)
	}
}

func handleGetFile(layer *pel.PktEncLayer, srcfile, dstdir string) {
	buffer := make([]byte, pel.Bufsize)

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
	buffer := make([]byte, pel.Bufsize)
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
