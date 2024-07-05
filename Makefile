GOFLAGS_LINUX=-trimpath -ldflags "-s -w"
GOFLAGS_WINDOWS=-trimpath -ldflags "-s -w" #-H=windowsgui"
GOOS ?= linux
GOARCH ?= amd64

DEFAULT_ENV=CGO_ENABLED=0

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
	env ${DEFAULT_ENV} GOOS=windows GOARCH=amd64 go build ${GOFLAGS_WINDOWS} -o ./build/tshd_windows_amd64.exe cmd/tshd.go
	env ${DEFAULT_ENV} GOOS=windows GOARCH=amd64 go build ${GOFLAGS_WINDOWS} -o ./build/tsh_windows_amd64.exe cmd/tsh.go

linux:
	env ${DEFAULT_ENV} GOOS=linux GOARCH=amd64 go build ${GOFLAGS_LINUX} -o ./build/rsh cmd/tsh.go

unix:
	env ${DEFAULT_ENV} GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS_LINUX} -o ./build/tshd_${GOOS}_${GOARCH} cmd/tshd.go
	env ${DEFAULT_ENV} GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS_LINUX} -o ./build/tsh_${GOOS}_${GOARCH} cmd/tsh.go

clean:
	rm ./build/*

.PHONY: all clean windows linux unix
