package runtime

import "net"

type PortPair struct {
	Port1 int
	Port2 int
}

func (p *PortPair) GenPortPair() error {
	var err error
	p.Port1, err = getRandomFreePort()
	if err != nil {
		return err
	}

	p.Port2, err = getRandomFreePort()
	if err != nil {
		return err
	}

	return nil
}

func getRandomFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
