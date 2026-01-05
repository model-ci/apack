//go:build windows
// +build windows

package distribution

import (
	"io/fs"
)

func (fm *FileMetadata) Fill(info fs.FileInfo) error {
	var typeflag byte
	switch {
	case info.IsDir():
		typeflag = '5'
	case info.Mode().IsRegular():
		typeflag = '0'
	default:
		typeflag = '?'
	}

	fm.Name = info.Name()
	fm.Mode = uint32(info.Mode().Perm())
	fm.Uid = 0
	fm.Gid = 0
	fm.Size = info.Size()
	fm.ModTime = info.ModTime()
	fm.Typeflag = typeflag

	return nil
}
