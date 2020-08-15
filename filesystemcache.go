package main

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func FileSystemCacheFromTempDir(root string) (*FileSystemCache, error) {
	tmpDir, err := ioutil.TempDir("", "")
	return &FileSystemCache{
		root:   root,
		tmpDir: tmpDir,
	}, err
}

type FileSystemCache struct {
	root   string
	tmpDir string
}

func (fsc *FileSystemCache) NewDirEntry(
	cacheDirCallback CacheDirCallback,
	nameCallback NameCallback,
) error {
	return fsc.withTmpArtifact(
		func(tmpDirPath string) error {
			if err := os.MkdirAll(tmpDirPath, 0744); err != nil {
				return err
			}
			return cacheDirCallback(
				func(relpath string, callback CacheFileCallback) error {
					filePath := filepath.Join(tmpDirPath, relpath)
					if err := os.MkdirAll(
						filepath.Dir(filePath),
						0744,
					); err != nil {
						return err
					}
					file, err := os.Create(filePath)
					if err != nil {
						return err
					}
					defer properClose(file)
					return callback(file)
				},
			)
		},
		nameCallback,
	)
}

func (fsc *FileSystemCache) NewFileEntry(
	cacheFileCallback CacheFileCallback,
	nameCallback NameCallback,
) error {
	return fsc.withTmpArtifact(
		func(tmpPath string) error {
			file, err := os.Create(tmpPath)
			if err != nil {
				return err
			}
			defer properClose(file)

			return cacheFileCallback(file)
		},
		nameCallback,
	)
}

func (fsc *FileSystemCache) MoveFile(src, dst string) error {
	// Initially try os.Rename. This will fail (at least on Linux) if `src` and
	// `dst` are on different file systems. There are probably other failure
	// cases as well. In case of any failure, log the error and try to fallback
	// to a copy-based method.
	if err := os.Rename(src, filepath.Join(fsc.root, dst)); err != nil {
		return errors.Wrapf(
			fsc.NewFileEntry(
				func(w io.Writer) error {
					file, err := os.Open(src)
					if err != nil {
						return errors.Wrapf(
							err,
							"Opening source file: %s",
							src,
						)
					}
					fileClosed := false
					defer func() {
						// Just in case there's a panic or error while copying
						// the file, make sure the file gets closed properly.
						if !fileClosed {
							properClose(file)
						}
					}()

					if _, err := io.Copy(w, file); err != nil {
						return errors.Wrapf(
							err,
							"Copying source file: %s",
							src,
						)
					}

					if err := file.Close(); err != nil {
						return errors.Wrapf(
							err,
							"Closing source file: %s",
							src,
						)
					}
					fileClosed = true

					return errors.Wrapf(
						os.RemoveAll(src),
						"Removing source file: %s",
						src,
					)
				},
				func() string { return dst },
			),
			"Handling error '%v': Copying file '%s' to cache path '%s'",
			err,
			src,
			dst,
		)
	}
	return nil
}

func (fsc *FileSystemCache) Exists(cachePath string) (bool, error) {
	if _, err := os.Stat(filepath.Join(fsc.root, cachePath)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (fsc *FileSystemCache) Root() string { return fsc.root }

func (fsc *FileSystemCache) withTmpArtifact(
	artifactCallback func(string) error,
	nameCallback NameCallback,
) error {
	tmpPath := filepath.Join(fsc.tmpDir, randString())
	if err := artifactCallback(tmpPath); err != nil {
		if err := os.RemoveAll(tmpPath); err != nil {
			log.Printf(
				"WARN failed to remove temporary artifact '%s': %v",
				tmpPath,
				err,
			)
		}
		return err
	}

	// commit artifact to cache
	cachePath := filepath.Join(fsc.root, nameCallback())
	if err := os.Rename(tmpPath, cachePath); err != nil {
		if os.IsExist(err) {
			if err := os.RemoveAll(cachePath); err != nil {
				return err
			}
			if err := os.Rename(tmpPath, cachePath); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func randString() string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(rand.Int63()))
	return hex.EncodeToString(buf[:])
}
