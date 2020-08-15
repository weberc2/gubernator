package main

import (
	"io"
	"log"
)

func properClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Printf("WARN failed to close: %v", err)
	}
}
