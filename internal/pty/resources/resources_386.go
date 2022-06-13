//go:build windows && 386
// +build windows,386

package resources

import _ "embed"

//go:embed 386/winpty-agent.exe
var WinptyAgent []byte

//go:embed 386/winpty.dll
var WinptyDll []byte
