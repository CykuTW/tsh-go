GOFLAGS_LINUX=-trimpath -ldflags "-s -w"
GOFLAGS_WINDOWS=-trimpath -ldflags "-s -w" #-H=windowsgui"
GOOS ?= linux
GOARCH ?= amd64

all:
	@echo
	@echo "Please specify one of these targets:"
	@echo "	make linux"
	@echo "	make windows"
	@echo
	@echo "It can be compiled to other unix-like platforms supported by go compiler:"
	@echo "	GOOS=freebsd GOARCH=386 make unix"
	@echo
	@echo "Get more with:"
	@echo "	go tool dist list"
	@echo

windows:
	env GOOS=windows GOARCH=amd64 go build ${GOFLAGS_WINDOWS} -o ./build/tshd_windows_amd64.exe cmd/tshd.go
	env GOOS=windows GOARCH=amd64 go build ${GOFLAGS_WINDOWS} -o ./build/tsh_windows_amd64.exe cmd/tsh.go

linux:
	env GOOS=linux GOARCH=amd64 go build ${GOFLAGS_LINUX} -o ./build/tshd_linux_amd64 cmd/tshd.go
	env GOOS=linux GOARCH=amd64 go build ${GOFLAGS_LINUX} -o ./build/tsh_linux_amd64 cmd/tsh.go

unix:
	env GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS_LINUX} -o ./build/tshd_${GOOS}_${GOARCH} cmd/tshd.go
	env GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS_LINUX} -o ./build/tsh_${GOOS}_${GOARCH} cmd/tsh.go

clean:
	rm ./build/*

.PHONY: all clean windows linux unix
