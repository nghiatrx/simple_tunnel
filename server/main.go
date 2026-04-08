package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

type Tunnel struct {
	localHost   string
	controlConn net.Conn
	pending     chan net.Conn
}

var (
	tunnels   = make(map[int]*Tunnel)
	tunnelsMu sync.RWMutex
)

func handleUserConnection(remotePort int, userConn net.Conn) {
	tunnelsMu.RLock()
	tunnel, ok := tunnels[remotePort]
	tunnelsMu.RUnlock()
	if !ok {
		log.Println("No client for port", remotePort)
		userConn.Close()
		return
	}

	// Signal client to open a data connection
	_, err := fmt.Fprintf(tunnel.controlConn, "CONNECT\n")
	if err != nil {
		log.Println("Cannot signal client:", err)
		userConn.Close()
		return
	}

	// Queue userConn, waiting for client to dial back on data port
	tunnel.pending <- userConn
}

func handleDataConnection(dataConn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			dataConn.Close()
		}
	}()

	scanner := bufio.NewScanner(dataConn)
	if !scanner.Scan() {
		dataConn.Close()
		return
	}
	line := scanner.Text()

	if !strings.HasPrefix(line, "DATA ") {
		log.Println("Invalid data handshake:", line)
		dataConn.Close()
		return
	}

	var remotePort int
	fmt.Sscanf(strings.TrimPrefix(line, "DATA "), "%d", &remotePort)

	tunnelsMu.RLock()
	tunnel, ok := tunnels[remotePort]
	tunnelsMu.RUnlock()
	if !ok {
		log.Println("No tunnel for data port", remotePort)
		dataConn.Close()
		return
	}

	userConn, ok := <-tunnel.pending
	if !ok {
		dataConn.Close()
		return
	}

	log.Printf("Bridging user <-> client for port %d\n", remotePort)
	done := make(chan struct{})
	go func() { io.Copy(dataConn, userConn); done <- struct{}{} }()
	go func() { io.Copy(userConn, dataConn); done <- struct{}{} }()
	<-done
	userConn.Close()
	dataConn.Close()
}

func handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	scanner := bufio.NewScanner(clientConn)
	if !scanner.Scan() {
		return
	}
	line := scanner.Text()

	if !strings.HasPrefix(line, "REGISTER ") {
		log.Println("Invalid client command:", line)
		return
	}

	parts := strings.Fields(line)
	if len(parts) < 3 {
		log.Println("Invalid REGISTER format:", line)
		return
	}

	var remotePort int
	fmt.Sscanf(parts[1], "%d", &remotePort)
	localHost := parts[2]

	tunnel := &Tunnel{
		localHost:   localHost,
		controlConn: clientConn,
		pending:     make(chan net.Conn, 32),
	}

	tunnelsMu.Lock()
	tunnels[remotePort] = tunnel
	tunnelsMu.Unlock()

	log.Printf("Registered: server:%d -> %s\n", remotePort, localHost)

	// Listen for user connections on remotePort
	go func(port int) {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			log.Println("Listen error for port", port, err)
			return
		}
		defer listener.Close()
		log.Println("Listening for users on port", port)
		for {
			userConn, err := listener.Accept()
			if err != nil {
				return
			}
			log.Printf("User connected to port %d\n", port)
			go handleUserConnection(port, userConn)
		}
	}(remotePort)

	// Keep control connection alive; clean up on disconnect
	for scanner.Scan() {
	}

	tunnelsMu.Lock()
	delete(tunnels, remotePort)
	tunnelsMu.Unlock()
	close(tunnel.pending)
	log.Printf("Client disconnected, removed tunnel for port %d\n", remotePort)
}

func main() {
	// Data port listener for client data connections
	dataListener, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Data listener on :9001")
	go func() {
		for {
			conn, err := dataListener.Accept()
			if err != nil {
				log.Println("Data accept error:", err)
				continue
			}
			go handleDataConnection(conn)
		}
	}()

	// Control listener for client registrations
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Server listening on :9000")

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go handleClient(clientConn)
	}
}
