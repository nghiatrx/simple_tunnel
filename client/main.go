package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	controlPort = "9000"
	dataPort    = "9001"
)

func handleConnect(serverHost, remotePort, localHost string) {
	dataConn, err := net.Dial("tcp", serverHost+":"+dataPort)
	if err != nil {
		log.Println("Cannot connect to server data port:", err)
		return
	}
	defer dataConn.Close()

	fmt.Fprintf(dataConn, "DATA %s\n", remotePort)

	localConn, err := net.Dial("tcp", localHost)
	if err != nil {
		log.Println("Cannot connect to local service:", localHost, err)
		return
	}
	defer localConn.Close()

	done := make(chan struct{})
	go func() { io.Copy(dataConn, localConn); done <- struct{}{} }()
	go func() { io.Copy(localConn, dataConn); done <- struct{}{} }()
	<-done
}

func runControl(serverHost, localHost, remotePort string) {
	controlAddr := serverHost + ":" + controlPort
	serverConn, err := net.Dial("tcp", controlAddr)
	if err != nil {
		log.Printf("Cannot connect to %s, retrying in 3s...\n", controlAddr)
		time.Sleep(3 * time.Second)
		return
	}
	defer serverConn.Close()

	fmt.Fprintf(serverConn, "REGISTER %s %s\n", remotePort, localHost)
	log.Printf("Registered tunnel: server:%s:%s -> %s\n", serverHost, remotePort, localHost)

	scanner := bufio.NewScanner(serverConn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "CONNECT" {
			go handleConnect(serverHost, remotePort, localHost)
		}
	}

	log.Println("Disconnected from server, reconnecting...")
}

func main() {
	if len(os.Args) != 6 {
		fmt.Println("Usage: tunnel.client <server_host> <control_port> <data_port> <local_host>:<local_port> <remote_port>")
		fmt.Println("Example: tunnel.client 127.0.0.1 9000 9001 localhost:3306 33060")
		os.Exit(1)
	}

	serverHost := os.Args[1]
	controlPort = os.Args[2]
	dataPort = os.Args[3]
	localHost := os.Args[4]
	remotePort := os.Args[5]

	// Validate port numbers
	if _, err := strconv.Atoi(controlPort); err != nil {
		log.Fatalf("Invalid control port: %s", controlPort)
	}
	if _, err := strconv.Atoi(dataPort); err != nil {
		log.Fatalf("Invalid data port: %s", dataPort)
	}

	fmt.Printf("Tunneling %s:%s -> %s via %s\n", serverHost, remotePort, localHost, serverHost)

	for {
		runControl(serverHost, localHost, remotePort)
	}
}
