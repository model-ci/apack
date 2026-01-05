//go:build !windows
// +build !windows

package utils

import "syscall"

func GetFileID(filename string) (uint64, error) {
	var stat syscall.Stat_t
	err := syscall.Stat(filename, &stat)
	if err != nil {
		return 0, err
	}
	return stat.Ino, nil
}
