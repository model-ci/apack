package layerdb

import (
	"io"
	"io/fs"
	"time"
)

type File interface {
	fs.FileInfo
	io.ReadCloser
}

type file struct {
	fs.FileInfo
	io.ReadCloser
}

func NewFile(fi fs.FileInfo, rc io.ReadCloser) File {
	return &file{
		FileInfo:   fi,
		ReadCloser: rc,
	}
}

func (f *file) Name() string {
	return f.FileInfo.Name()
}
func (f *file) Size() int64 {
	return f.FileInfo.Size()
}

func (f *file) Mode() fs.FileMode {
	return f.FileInfo.Mode()
}

func (f *file) ModTime() time.Time {
	return f.FileInfo.ModTime()
}

func (f *file) IsDir() bool {
	return f.FileInfo.IsDir()
}

func (f *file) Sys() any {
	return f.FileInfo.Sys()
}

func (f *file) Read(p []byte) (n int, err error) {
	return f.ReadCloser.Read(p)
}

func (f *file) Close() error {
	return f.ReadCloser.Close()
}
