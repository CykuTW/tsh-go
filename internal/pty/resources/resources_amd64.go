//go:build windows && amd64
// +build windows,amd64

package resources

import _ "embed"

//go:embed amd64/winpty-agent.exe
var WinptyAgent []byte

//go:embed amd64/winpty.dll
var WinptyDll []byte
