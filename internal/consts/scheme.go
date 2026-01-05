package consts

const (
	UnixSocketScheme = "unix://"
	TCPScheme        = "tcp://"
	HTTPScheme       = "http://"
	HTTPSScheme      = "https://"
)

func ApackSock() string {
	return UnixSocketScheme + "/tmp/run/apack.sock"
}
