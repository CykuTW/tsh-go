package pel

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"net"
	"time"

	"tsh-go/internal/constants"
)

// Packet Encryption Layer
type PktEncLayer struct {
	conn          net.Conn
	secret        string
	sendEncrypter cipher.BlockMode
	recvDecrypter cipher.BlockMode
	sendPktCtr    uint
	recvPktCtr    uint
	sendHmac      hash.Hash
	recvHmac      hash.Hash
	readBuffer    []byte
	writeBuffer   []byte
}

// Packet Encryption Layer Listener
type PktEncLayerListener struct {
	listener net.Listener
	secret   string
	isServer bool
}

func NewPktEncLayerListener(address, secret string, isServer bool) (*PktEncLayerListener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	ln := &PktEncLayerListener{
		listener: listener,
		secret:   secret,
		isServer: isServer,
	}
	return ln, nil
}

func NewPktEncLayer(conn net.Conn, secret string) (*PktEncLayer, error) {
	layer := &PktEncLayer{
		conn:        conn,
		secret:      secret,
		sendPktCtr:  0,
		recvPktCtr:  0,
		readBuffer:  make([]byte, constants.Bufsize+16+20),
		writeBuffer: make([]byte, constants.Bufsize+16+20),
	}
	return layer, nil
}

func NewPelError(err int) error {
	return errors.New(fmt.Sprintf("%d", err))
}

func Listen(address, secret string, isServer bool) (*PktEncLayerListener, error) {
	listener, err := NewPktEncLayerListener(address, secret, isServer)
	return listener, err
}

func (ln *PktEncLayerListener) Close() error {
	return ln.listener.Close()
}

func (ln *PktEncLayerListener) Addr() net.Addr {
	return ln.listener.Addr()
}

func (ln *PktEncLayerListener) Accept() (l *PktEncLayer, err error) {
	defer func() {
		if _err := recover(); _err != nil {
			l = nil
			err = NewPelError(constants.PelSystemError)
		}
	}()
	conn, err := ln.listener.Accept()
	if err != nil {
		return nil, err
	}
	layer, _ := NewPktEncLayer(conn, ln.secret)
	err = layer.Handshake(ln.isServer)
	if err != nil {
		layer.Close()
		return nil, err
	}
	return layer, nil
}

func Dial(address, secret string, isServer bool) (l *PktEncLayer, err error) {
	defer func() {
		if _err := recover(); _err != nil {
			l = nil
			err = NewPelError(constants.PelSystemError)
		}
	}()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, err
	}
	layer, _ := NewPktEncLayer(conn, secret)
	err = layer.Handshake(isServer)
	if err != nil {
		layer.Close()
		return nil, err
	}
	return layer, nil
}

func (layer *PktEncLayer) hashKey(iv []byte) []byte {
	h := sha1.New()
	h.Write([]byte(layer.secret))
	h.Write(iv)
	return h.Sum(nil)
}

// exchange IV with client and setup the encryption layer
// return err if the packet read/write operation
// takes more than HandshakeRWTimeout (default: 3) seconds
func (layer *PktEncLayer) Handshake(isServer bool) error {
	if isServer {
		buffer := make([]byte, 40)
		layer.conn.SetReadDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		if err := layer.readConnUntil(buffer, 40); err != nil {
			return err
		}
		layer.conn.SetReadDeadline(time.Time{})
		iv1 := buffer[20:]
		iv2 := buffer[:20]

		var key []byte
		var block cipher.Block

		key = layer.hashKey(iv1)
		block, _ = aes.NewCipher(key[:16])
		layer.sendEncrypter = cipher.NewCBCEncrypter(block, iv1[:16])
		layer.sendHmac = hmac.New(sha1.New, key)

		key = layer.hashKey(iv2)
		block, _ = aes.NewCipher(key[:16])
		layer.recvDecrypter = cipher.NewCBCDecrypter(block, iv2[:16])
		layer.recvHmac = hmac.New(sha1.New, key)

		layer.conn.SetReadDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		n, err := layer.Read(buffer[:16])
		layer.conn.SetReadDeadline(time.Time{})
		if n != 16 || err != nil ||
			bytes.Compare(buffer[:16], constants.Challenge) != 0 {
			return NewPelError(constants.PelWrongChallenge)
		}

		layer.conn.SetWriteDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		n, err = layer.Write(constants.Challenge)
		layer.conn.SetWriteDeadline(time.Time{})
		if n != 16 || err != nil {
			return NewPelError(constants.PelFailure)
		}
		return nil
	} else {
		iv := make([]byte, 40)
		rand.Read(iv)
		layer.conn.SetWriteDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		n, err := layer.conn.Write(iv)
		layer.conn.SetWriteDeadline(time.Time{})
		if n != 40 || err != nil {
			return NewPelError(constants.PelFailure)
		}

		var key []byte
		var block cipher.Block

		key = layer.hashKey(iv[:20])
		block, _ = aes.NewCipher(key[:16])
		layer.sendEncrypter = cipher.NewCBCEncrypter(block, iv[:16])
		layer.sendHmac = hmac.New(sha1.New, key)

		key = layer.hashKey(iv[20:])
		block, _ = aes.NewCipher(key[:16])
		layer.recvDecrypter = cipher.NewCBCDecrypter(block, iv[20:36])
		layer.recvHmac = hmac.New(sha1.New, key)

		layer.conn.SetWriteDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		n, err = layer.Write(constants.Challenge)
		layer.conn.SetWriteDeadline(time.Time{})
		if n != 16 || err != nil {
			return NewPelError(constants.PelFailure)
		}

		challenge := make([]byte, 16)
		layer.conn.SetReadDeadline(
			time.Now().Add(time.Duration(constants.HandshakeRWTimeout) * time.Second))
		n, err = layer.Read(challenge)
		layer.conn.SetReadDeadline(time.Time{})
		if n != 16 || err != nil {
			return NewPelError(constants.PelFailure)
		}
		if bytes.Compare(constants.Challenge, challenge) != 0 {
			return NewPelError(constants.PelWrongChallenge)
		}
		return nil
	}
}

func (layer *PktEncLayer) Close() {
	layer.conn.Close()
}

func (layer *PktEncLayer) Write(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		n, err := layer.write(p[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (layer *PktEncLayer) write(p []byte) (int, error) {
	length := len(p)
	if length <= 0 || length > constants.Bufsize {
		return 0, NewPelError(constants.PelBadMsgLength)
	}

	buffer := layer.writeBuffer
	buffer[0] = byte((length >> 8) & 0xFF)
	buffer[1] = byte(length & 0xFF)
	copy(buffer[2:], p)

	blkLength := 2 + length
	padding := 16 - (blkLength & 0x0F)
	if (blkLength & 0x0F) != 0 {
		blkLength += padding
	}

	layer.sendEncrypter.CryptBlocks(buffer[:blkLength], buffer[:blkLength])

	buffer[blkLength] = byte(layer.sendPktCtr << 24 & 0xFF)
	buffer[blkLength+1] = byte(layer.sendPktCtr << 16 & 0xFF)
	buffer[blkLength+2] = byte(layer.sendPktCtr << 8 & 0xFF)
	buffer[blkLength+3] = byte(layer.sendPktCtr & 0xFF)

	layer.sendHmac.Reset()
	layer.sendHmac.Write(buffer[:blkLength+4])
	digest := layer.sendHmac.Sum(nil)

	copy(buffer[blkLength:], digest[:20])
	total := 0
	for total < blkLength+20 {
		n, err := layer.conn.Write(buffer[total : blkLength+20])
		if err != nil {
			return 0, err
		}
		total += n
	}
	layer.sendPktCtr++
	return length, nil
}

func (layer *PktEncLayer) Read(p []byte) (int, error) {
	return layer.read(p)
}

func (layer *PktEncLayer) read(p []byte) (int, error) {
	firstblock := make([]byte, 16)
	buffer := layer.readBuffer

	if err := layer.readConnUntil(buffer, 16); err != nil {
		return 0, err
	}

	layer.recvDecrypter.CryptBlocks(firstblock, buffer[:16])
	length := int(firstblock[0])<<8 + int(firstblock[1])
	if length <= 0 || length > constants.Bufsize || length > len(p) {
		return 0, NewPelError(constants.PelBadMsgLength)
	}

	blkLength := 2 + length
	if (blkLength & 0x0F) != 0 {
		blkLength += 16 - (blkLength & 0x0F)
	}

	if err := layer.readConnUntil(buffer[16:], blkLength-16+20); err != nil {
		return 0, err
	}

	hmac := append([]byte{}, buffer[blkLength:blkLength+20]...)
	buffer[blkLength] = byte(layer.recvPktCtr << 24 & 0xFF)
	buffer[blkLength+1] = byte(layer.recvPktCtr << 16 & 0xFF)
	buffer[blkLength+2] = byte(layer.recvPktCtr << 8 & 0xFF)
	buffer[blkLength+3] = byte(layer.recvPktCtr & 0xFF)

	layer.recvHmac.Reset()
	layer.recvHmac.Write(buffer[:blkLength+4])
	digest := layer.recvHmac.Sum(nil)

	if bytes.Compare(hmac, digest) != 0 {
		return 0, NewPelError(constants.PelCorruptedData)
	}

	layer.recvDecrypter.CryptBlocks(buffer[16:blkLength], buffer[16:blkLength])
	copy(buffer, firstblock)
	n := copy(p, buffer[2:2+length])
	layer.recvPktCtr++
	return n, nil
}

func (layer *PktEncLayer) readConnUntil(p []byte, fill int) error {
	total := 0
	for total < fill {
		n, err := layer.conn.Read(p[total:fill])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}
