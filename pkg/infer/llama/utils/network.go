package utils

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

func FindAvailablePort(startPort, endPort int) (int, error) {
	for port := startPort; port <= endPort; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, endPort)
}

func IsPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

func WaitForPort(host string, port int, timeout time.Duration) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for port %s:%d", host, port)
}

func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no local IP address found")
}

func GetFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func IsValidPort(port int) bool {
	return port > 0 && port <= 65535
}

func ParseHostPort(hostport string) (host string, port int, err error) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", 0, err
	}

	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", portStr)
	}

	if !IsValidPort(port) {
		return "", 0, fmt.Errorf("port out of range: %d", port)
	}

	return host, port, nil
}

type NetworkInfo struct {
	LocalIP    string          `json:"local_ip"`
	Interfaces []InterfaceInfo `json:"interfaces"`
}

type InterfaceInfo struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	IsUp      bool     `json:"is_up"`
}

func GetNetworkInfo() (*NetworkInfo, error) {
	localIP, _ := GetLocalIP()

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var interfaceInfos []InterfaceInfo
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var addresses []string
		for _, addr := range addrs {
			addresses = append(addresses, addr.String())
		}

		interfaceInfos = append(interfaceInfos, InterfaceInfo{
			Name:      iface.Name,
			Addresses: addresses,
			IsUp:      iface.Flags&net.FlagUp != 0,
		})
	}

	return &NetworkInfo{
		LocalIP:    localIP,
		Interfaces: interfaceInfos,
	}, nil
}

type PortScanner struct {
	host    string
	timeout time.Duration
}

func NewPortScanner(host string, timeout time.Duration) *PortScanner {
	return &PortScanner{
		host:    host,
		timeout: timeout,
	}
}

func (ps *PortScanner) ScanPort(port int) bool {
	address := fmt.Sprintf("%s:%d", ps.host, port)
	conn, err := net.DialTimeout("tcp", address, ps.timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (ps *PortScanner) ScanRange(startPort, endPort int) []int {
	var openPorts []int

	for port := startPort; port <= endPort; port++ {
		if ps.ScanPort(port) {
			openPorts = append(openPorts, port)
		}
	}

	return openPorts
}

type ConnectionPool struct {
	connections chan net.Conn
	factory     func() (net.Conn, error)
	maxSize     int
}

func NewConnectionPool(maxSize int, factory func() (net.Conn, error)) *ConnectionPool {
	return &ConnectionPool{
		connections: make(chan net.Conn, maxSize),
		factory:     factory,
		maxSize:     maxSize,
	}
}

func (cp *ConnectionPool) Get() (net.Conn, error) {
	select {
	case conn := <-cp.connections:
		return conn, nil
	default:
		return cp.factory()
	}
}

func (cp *ConnectionPool) Put(conn net.Conn) {
	select {
	case cp.connections <- conn:
	default:
		conn.Close()
	}
}

func (cp *ConnectionPool) Close() {
	close(cp.connections)
	for conn := range cp.connections {
		conn.Close()
	}
}
