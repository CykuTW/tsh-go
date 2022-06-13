package constants

const (
	Bufsize = 4096

	GetFile  = 1
	PutFile  = 2
	RunShell = 3

	PelSuccess = 1
	PelFailure = 0

	PelSystemError    = -1
	PelConnClosed     = -2
	PelWrongChallenge = -3
	PelBadMsgLength   = -4
	PelCorruptedData  = -5
	PelUndefinedError = -6

	HandshakeRWTimeout = 3 // seconds
)

var Challenge = []byte{
	0x58, 0x90, 0xAE, 0x86, 0xF1, 0xB9, 0x1C, 0xF6,
	0x29, 0x83, 0x95, 0x71, 0x1D, 0xDE, 0x58, 0x0D,
}
