package main

import (
	"io"
	"os"
)

type CacheFileCallback func(io.Writer) (os.FileMode, error)

type CacheDir func(relpath string, callback CacheFileCallback) error

type CacheDirCallback func(CacheDir) error

type NameCallback func() string

// NOTE: Cache methods take a NameCallback (a function that returns the name)
// instead of a Name string because the name itself might be a function of a
// side-effect of the CacheDirCallback or the CacheFileCallback, for example,
// if the name includes the hash of a file (or files in the directory)--in this
// example, if we wanted to support a Name string instead of a NameCallback,
// the caller would have to read the file once to get the hash, build the name
// string, and then read the file again in the CacheFileCallback. By providing
// a NameCallback, we can avoid reading the file twice (we could also use a
// *string or make the other callback return a (string, error) instead of just
// an error--at some point we should explore these other options and see what
// is most ergonomic).

type Cache interface {
	NewDirEntry(CacheDirCallback, NameCallback) error
	NewFileEntry(CacheFileCallback, NameCallback) error
}
