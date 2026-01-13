//go:build !windows
// +build !windows

package distribution

import (
	"fmt"
	"io/fs"
	"syscall"
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

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("not syscall.Stat_t")
	}

	fm.FileMetadata.Name = info.Name()
	fm.FileMetadata.Mode = uint32(info.Mode().Perm())
	fm.Uid = stat.Uid
	fm.Gid = stat.Gid
	fm.FileMetadata.Size = info.Size()
	fm.FileMetadata.ModTime = info.ModTime()
	fm.Typeflag = typeflag

	return nil
}
