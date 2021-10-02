package main

import (
	"bytes"
	"encoding/binary"
)

func getBool(b *[]byte) bool {
	r := (*b)[0] > 0
	*b = (*b)[1:]
	return r
}

func getByte(b *[]byte) byte {
	r := (*b)[0]
	*b = (*b)[1:]
	return r
}

func getShort(b *[]byte) int {
	r := binary.LittleEndian.Uint16(*b)
	*b = (*b)[2:]
	return int(r)
}

func getLong(b *[]byte) int {
	r := binary.LittleEndian.Uint32(*b)
	*b = (*b)[4:]
	return int(r)
}

func getFloat(b *[]byte) {
	*b = (*b)[4:]
}

func getBytes(b *[]byte, n int) []byte {
	r := (*b)[:n]
	*b = (*b)[n:]
	return r
}

func getString(b *[]byte) string {
	n := bytes.IndexByte(*b, 0)
	r := string((*b)[:n])
	*b = (*b)[n+1:]
	return r
}
