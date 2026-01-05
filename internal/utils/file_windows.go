//go:build windows
// +build windows

package utils

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// GetFileID returns a unique identifier for a file on Windows
// Windows uses a combination of volume serial number and file index
func GetFileID(filename string) (uint64, error) {
	// Convert filename to UTF16 for Windows API
	filenamePtr, err := windows.UTF16PtrFromString(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to convert filename to UTF16: %w", err)
	}

	// Get file handle
	handle, err := windows.CreateFile(
		filenamePtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer windows.CloseHandle(handle)

	// Get file information
	var fileInfo windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(handle, &fileInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to get file information: %w", err)
	}

	// Combine volume serial number and file index to create a unique ID
	// FileIndexHigh and FileIndexLow form the file index
	fileIndex := (uint64(fileInfo.FileIndexHigh) << 32) | uint64(fileInfo.FileIndexLow)

	return fileIndex, nil
}
