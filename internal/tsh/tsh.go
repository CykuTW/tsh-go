package tsh

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"sync"
	"time"
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
		fmt.Fprintf(flagset.Output(), "        uuids shell cmds timeout\n")
		fmt.Fprintf(flagset.Output(), "        uuids script a.sh timeout\n")
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
	case args[0] == "script" && len(args) >= 2:
		var timeout int
		if len(args) == 2 {
			timeout = 10
		} else {
			timeout, _ = strconv.Atoi(args[2])
			if timeout <= 0 {
				timeout = 10
			}
		}
		handleScripts(uuid, args[1], timeout)
		return
	case args[0] == "shell" && len(args) >= 2:
		var timeout int
		if len(args) == 2 {
			timeout = 10
		} else {
			timeout, _ = strconv.Atoi(args[2])
			if timeout <= 0 {
				timeout = 10
			}
		}
		handleShell(uuid, args[1], timeout)
		return
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

var STATUS_SPLIT = "\n----------------------------\n"
var STATUS_DISCONNECT = "DisConnect"
var STATUS_FAILED = "Failed"
var STATUS_SUCCESS = "Success"
var STATUS_TIMEOUT = "Timeout"
var STATUS_EMPTY_STDOUT = "EmptyResult"
var SEND_BUFFER = 4096

func do_script(uuid string, script string, content *string, ch chan string, timeout int) {
	done := make(chan string, 1)
	var result string
	go func() {
		// 上传脚本
		layer_put, err := pel.Dial(uuid, pel.PEL_SECRET, false)
		if err != nil {
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_DISCONNECT, STATUS_SPLIT)
			done <- result
			return
		}
		defer layer_put.Close()

		layer_put.Write([]byte{pel.PutFile})

		basename := filepath.Base(script)
		destfile := "/tmp/" + basename
		_, err = layer_put.Write([]byte(destfile))
		if err != nil {
			result += fmt.Sprintf("error: %v\n", err)
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
			done <- result
			return
		}
		rd := make([]byte, 1024)
		_, err = layer_put.Read(rd)
		if err != nil {
			result += fmt.Sprintf("error: %v\n", err)
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
			done <- result
			return
		}

		length := len(*content)
		send_time := (length + 4095) / 4096
		file_buffer := []byte(*content)
		for i := 0; i < send_time; i++ {
			var n int
			var err error
			if i == send_time - 1 {
				n, err = layer_put.Write(file_buffer[i*4096:])
			} else {
				n, err = layer_put.Write(file_buffer[i*4096:i*4096+4096])
			}
			
			if err != nil {
				result += fmt.Sprintf("error: %v, send: %d\n", err, n)
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
				done <- result
				return
			}
		}

		layer_put.Close()
		empty_time := 0
		// 执行脚本
		for i := 0; i < 5; i++ {
			var cmds string
			if strings.Contains(basename, ".py") {
				cmds = fmt.Sprintf(" python %s", destfile)
			} else {
				cmds = fmt.Sprintf(" sh %s", destfile)
			}
			
			layer, err := pel.Dial(uuid, pel.PEL_SECRET, false)
			if err != nil {
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_DISCONNECT, STATUS_SPLIT)
				done <- result
				return
			}
			defer layer.Close()
	
			layer.Write([]byte{pel.RunShell})
			
			_, err = layer.Write([]byte("vt100"))
			if err != nil {
				result += fmt.Sprintf("error: %v\n", err)
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
				done <- result
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
				result += fmt.Sprintf("error: %v\n", err)
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
				done <- result
				return
			}
	
			_, err = layer.Write([]byte(cmds))
			if err != nil {
				result += fmt.Sprintf("error: %v\n", err)
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
				done <- result
				return
			}
	
			buffer := make([]byte, pel.Bufsize)
			var cmd_recv_bytes int
			for {
				n, err := layer.Read(buffer)
				result += string(buffer[0:n])
				if err != nil {
					break
				}
				cmd_recv_bytes += n
			}
	
			if cmd_recv_bytes > 0 {
				result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_SUCCESS, STATUS_SPLIT)
				break
			} else {
				empty_time += 1
				result += fmt.Sprintf("empty result, retry %d...\n", empty_time)
				continue
			}
		}

		if empty_time == 5 {
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_EMPTY_STDOUT, STATUS_SPLIT)
		}
		
		done <- result
	}()

	select {
	case result := <-done:
		ch <- result
		return
	case <-time.After(time.Second * time.Duration(timeout)):
		var tmp = result
		tmp += fmt.Sprintf("\nuuid:%s 任务执行结果:%s%s", uuid, STATUS_TIMEOUT, STATUS_SPLIT)
		ch <- tmp
		return
	}
}

func handleScripts(uuids string, script string, timeout int) {
	uuid_list := strings.Split(uuids, ",")
	length := len(uuid_list)
	if length > 100 {
		length = 100
	}

	f, err := os.Open(script)
	if err != nil {
		fmt.Printf("原始文件读取失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	bytes, err := ioutil.ReadFile(script)
	if err != nil {
		fmt.Printf("原始文件读取失败: %v\n", err)
		os.Exit(1)
	}
	content := string(bytes)

	var responseChannel = make(chan string, length)

	for _, uuid := range uuid_list {
		go do_script(uuid, script, &content, responseChannel, timeout)
	}

	for i := 0; i < length; i++ {
		data := <- responseChannel
		fmt.Println(data)
	}

	time.Sleep(1 * time.Second)
}

func shell(uuid string, cmds string, ch chan string, timeout int) {
	done := make(chan string, 1)
	var result string
	go func() {
		layer, err := pel.Dial(uuid, pel.PEL_SECRET, false)
		if err != nil {
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_DISCONNECT, STATUS_SPLIT)
			done <- result
			return
		}
		defer layer.Close()

		layer.Write([]byte{pel.RunShell})
		
		_, err = layer.Write([]byte("vt100"))
		if err != nil {
			result += fmt.Sprintf("error: %v\n", err)
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
			done <- result
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
			result += fmt.Sprintf("error: %v\n", err)
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
			done <- result
			return
		}

		_, err = layer.Write([]byte(cmds))
		if err != nil {
			result += fmt.Sprintf("error: %v\n", err)
			result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_FAILED, STATUS_SPLIT)
			done <- result
			return
		}

		buffer := make([]byte, pel.Bufsize)
		for {
			n, err := layer.Read(buffer)
			result += string(buffer[0:n])
			if err != nil {
				break
			}
		}

		result += fmt.Sprintf("uuid:%s 任务执行结果:%s%s", uuid, STATUS_SUCCESS, STATUS_SPLIT)
		done <- result
	}()

	select {
	case result := <-done:
		ch <- result
		return
	case <-time.After(time.Second * time.Duration(timeout)):
		var tmp = result
		tmp += fmt.Sprintf("\nuuid:%s 任务执行结果:%s%s", uuid, STATUS_TIMEOUT, STATUS_SPLIT)
		ch <- tmp
		return
	}
}

func handleShell(uuids string, cmds string, timeout int) {
	uuid_list := strings.Split(uuids, ",")
	length := len(uuid_list)
	if length > 100 {
		length = 100
	}

	var responseChannel = make(chan string, length)

	for _, uuid := range uuid_list {
		go shell(uuid, cmds, responseChannel, timeout)
	}

	for i := 0; i < length; i++ {
		data := <- responseChannel
		fmt.Println(data)
	}

	time.Sleep(1 * time.Second)
}

func handleGetFile(layer *pel.PktEncLayer, srcfile, dstdir string) {
	buffer := make([]byte, pel.Bufsize)

	basename := strings.ReplaceAll(srcfile, "\\", "/")
	basename = filepath.Base(filepath.FromSlash(basename))

	f, err := os.OpenFile(filepath.Join(dstdir, basename), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("创建文件失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	_, err = layer.Write([]byte(srcfile))
	if err != nil {
		fmt.Printf("下载文件失败: %v\n", err)
		os.Exit(1)
	}
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWidth(20),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionSpinnerType(22),
	)

	rd := make([]byte, 1024)
	_, err = layer.Read(rd)

	bytes, err := io.CopyBuffer(io.MultiWriter(f, bar), layer, buffer)
	if strings.Contains(string(rd), "error:") {
		fmt.Printf("\n Recv %s\n", rd)
	} else if err == nil {
		fmt.Printf("\n Recv Success: %d bytes\n", bytes)
		os.Exit(0)
	} else {
		fmt.Printf("\n err: %v", err)
	}

	layer.Close()
}

func handlePutFile(layer *pel.PktEncLayer, srcfile, dstdir string) {
	buffer := make([]byte, pel.Bufsize)
	f, err := os.Open(srcfile)
	if err != nil {
		fmt.Printf("原始文件读取失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fmt.Printf("原始文件读取失败: %v\n", err)
		os.Exit(1)
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

	var wg sync.WaitGroup
	wg.Add(1)

	rd := make([]byte, 1024)
	_, err = layer.Read(rd)

	go func(done func()) {
		defer done()

		bytes, err := io.CopyBuffer(io.MultiWriter(layer, bar), f, buffer)
		if strings.Contains(string(rd), "error:") {
			fmt.Printf("\n Send %s\n", rd)
		} else if err == nil {
			fmt.Printf("\n Send Success: %d bytes\n", bytes)
			os.Exit(0)
		} else {
			fmt.Printf("\n err: %v", err)
		}
		layer.Close()
	}(wg.Done)

	io.CopyBuffer(os.Stdout, layer, rd)
	layer.Close()
	wg.Wait()
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
