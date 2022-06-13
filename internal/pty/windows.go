//go:build windows
// +build windows

package pty

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"tsh-go/internal/pty/resources"

	"github.com/denisbrodbeck/machineid"
	"github.com/iamacarpet/go-winpty"
)

var (
	tempdir string
)

type WinPtyWrapper struct {
	wp *winpty.WinPTY
}

func (pw WinPtyWrapper) StdIn() io.Writer {
	return pw.wp.StdIn
}

func (pw WinPtyWrapper) StdOut() io.Reader {
	return pw.wp.StdOut
}

func (pw WinPtyWrapper) Close() {
	pw.wp.Close()
}

func init() {
	id, err := machineid.ID()
	if err != nil {
		hostname, err := os.Hostname()
		if err != nil {
			id = ""
		} else {
			id = hostname
		}
	}

	h := sha1.New()
	ostempdir := os.TempDir()

	for tries := 0; tries < 100; tries++ {
		h.Write([]byte(id))
		bs := h.Sum(nil)
		fakeguid := fmt.Sprintf(
			"{%X-%X-%X-%X-%X}",
			bs[:4], bs[4:6], bs[6:8], bs[8:10], bs[10:16],
		)
		dstdir := path.Join(ostempdir, fakeguid)
		if tryDropWinpty(dstdir) {
			tempdir = dstdir
			break
		}
	}
}

func tryDropWinpty(dstdir string) bool {
	checksum1, checksum2 := getWinptyChecksum()
	winptyAgentPath := path.Join(dstdir, "winpty-agent.exe")
	winptyDllPath := path.Join(dstdir, "winpty.dll")
	if fi, err := os.Stat(dstdir); os.IsNotExist(err) {
		err := os.Mkdir(dstdir, 0700)
		if err == nil {
			err := ioutil.WriteFile(winptyAgentPath, resources.WinptyAgent, 0700)
			if err != nil {
				return false
			}
			err = ioutil.WriteFile(winptyDllPath, resources.WinptyDll, 0600)
			if err != nil {
				return false
			}
			return true
		}
	} else if fi.Mode().IsDir() {
		if isFileNotExist(winptyAgentPath) ||
			bytes.Compare(getFileChecksum(winptyAgentPath), checksum1) != 0 {
			return false
		}
		if isFileNotExist(winptyDllPath) ||
			bytes.Compare(getFileChecksum(winptyDllPath), checksum2) != 0 {
			return false
		}
		return true
	}
	return false
}

func getWinptyChecksum() ([]byte, []byte) {
	h := sha1.New()
	h.Write(resources.WinptyAgent)
	a := h.Sum(nil)
	h.Reset()
	h.Write(resources.WinptyDll)
	d := h.Sum(nil)
	return a, d
}

func getFileChecksum(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err == nil {
		return h.Sum(nil)
	}
	return nil
}

func isFileNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func OpenPty(command, term string, ws_col, ws_row uint32) (PtyWrapper, error) {
	if command == "exec bash --login" {
		command = `C:\windows\system32\cmd.exe`
	}
	options := winpty.Options{
		DLLPrefix:   tempdir,
		Command:     command,
		InitialCols: ws_col,
		InitialRows: ws_row,
		Env:         os.Environ(),
	}
	wp, err := winpty.OpenWithOptions(options)
	if err != nil {
		return nil, err
	}
	return WinPtyWrapper{wp: wp}, nil
}
