# Simple Tunnel

A simple TCP tunneling tool written in Go that allows exposing local services through a remote server using reverse tunneling.

## Purpose

This project implements a basic reverse TCP tunnel. The server runs on a publicly accessible machine, and clients connect to it to register tunnels. When users connect to the server's exposed port, the traffic is forwarded through the tunnel to the client's local service.

This is useful for:
- Exposing local development servers to the internet
- Bypassing firewall restrictions
- Creating secure tunnels for local services

## Requirements

- Go 1.16 or later
- Network connectivity between client and server

## How to Build

### Server
```bash
cd server
go build -ldflags="-s -w" -o tunnel.server main.go
```

### Client
```bash
cd client
go build -ldflags="-s -w" -o tunnel.client main.go
```

### Or use the build script
```bash
./build.sh
```
This creates `build/tunnel.server` and `build/tunnel.client` binaries, plus separate tar.gz archives for each.

## How to Use

### Running the Server

```bash
./tunnel.server
```

The server will:
- Listen on `:9000` for client control connections
- Listen on `:9001` for client data connections
- Listen on user-specified ports for tunnel traffic

### Running the Client

```bash
./tunnel.client <server_host> <control_port> <data_port> <local_host>:<local_port> <remote_port>
```

Example:
```bash
./tunnel.client 127.0.0.1 9000 9001 localhost:3306 33060
```

This will:
- Connect to server `127.0.0.1` on control port `9000`
- Use data port `9001` for tunnel traffic
- Register a tunnel from server port `33060` to `localhost:3306` on the client
- Users can now connect to `server_ip:33060` to access the local MySQL database

### Architecture

- **Control Connection**: Client maintains a persistent connection to `server:control_port` for registration and commands
- **Data Connection**: On each user connection, server sends `CONNECT` signal, client dials `server:data_port` and bridges traffic
- **User Connection**: Users connect to `server:remote_port`, traffic is forwarded through the tunnel

## Security Note

This is a basic implementation without authentication or encryption. Use with caution and consider adding security measures for production use.
