//go:build windows
// +build windows

package distribution

import "io/fs"

func (fm *FileMetadata) Fill(info fs.FileInfo) error {
	return fm.FillWithRename(info, "")
}

func (fm *FileMetadata) FillWithRename(info fs.FileInfo, rename string) error {
	var typeflag byte
	switch {
	case info.IsDir():
		typeflag = '5'
	case info.Mode().IsRegular():
		typeflag = '0'
	default:
		typeflag = '?'
	}

	fm.FileMetadata.Name = info.Name()
	if rename != "" {
		fm.FileMetadata.Name = rename
	}

	fm.FileMetadata.Mode = uint32(info.Mode().Perm())
	fm.Uid = 0
	fm.Gid = 0
	fm.FileMetadata.Size = info.Size()
	fm.FileMetadata.ModTime = info.ModTime()
	fm.Typeflag = typeflag

	return nil
}
