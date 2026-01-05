package errs

import "errors"

var (
	ErrAlreadyRunning      = errors.New("client is already running")
	ErrNotRunning          = errors.New("client is not running")
	ErrInvalidModelPath    = errors.New("invalid model path")
	ErrInvalidPort         = errors.New("invalid port number")
	ErrStartTimeout        = errors.New("start timeout exceeded")
	ErrModelNotFound       = errors.New("model file not found")
	ErrBinaryNotFound      = errors.New("llamafile binary not found")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrProcessDied         = errors.New("llamafile process died")
	ErrUINotEnabled        = errors.New("UI is not enabled")
	ErrRequestFailed       = errors.New("request failed")
)
