package main

import (
	"hash"
	"io"
)

type HashingReader struct {
	Hasher hash.Hash
	Reader io.Reader
}

func (hr *HashingReader) Read(buf []byte) (int, error) {
	n, err := hr.Reader.Read(buf)
	hr.Hasher.Write(buf)
	return n, err
}

func (hr *HashingReader) Hash() []byte {
	return hr.Hasher.Sum(nil)
}

func (hr *HashingReader) Copy(w io.Writer) ([]byte, error) {
	_, err := io.Copy(w, hr)
	return hr.Hash(), err
}
