# make-it-public
Service for publishing services that are hidden behind NAT

**make-it-public** is a service designed to expose services that are hidden behind NAT (Network Address Translation). It allows users to securely and efficiently publish services that are otherwise inaccessible from the public internet. This project achieves this by implementing a reverse proxy and client-server architecture, enabling seamless communication between clients and servers.

---

## Table of Contents

1. [Overview](#overview)
2. [How It Works](#how-it-works)
3. [Modules](#modules)
   - [cmd](#cmd)
   - [pkg/core](#pkgcore)
   - [pkg/edge](#pkgedge)
   - [pkg/repo](#pkgrepo)
   - [pkg/revclient](#pkgrevclient)
   - [pkg/revproxy](#pkgrevproxy)
4. [Configuration](#configuration)
5. [Usage](#usage)
6. [Development](#development)
7. [License](#license)

---

## Overview

The **make-it-public** project is a reverse proxy solution that allows users to expose services running on private networks to the public internet. It consists of two main components:

1. **Server**: Acts as a reverse proxy, handling incoming connections and routing them to the appropriate client.
2. **Client**: Establishes a connection to the server and forwards requests to the local service running behind NAT.

This architecture enables users to securely expose their local services without requiring complex network configurations or public IP addresses.

---

## How It Works

1. **Client-Server Communication**:
   - The client connects to the server using a secure token for authentication.
   - The server listens for incoming requests and forwards them to the appropriate client based on the subdomain or user ID.

2. **Reverse Proxy**:
   - The server acts as a reverse proxy, routing HTTP and TCP connections to the appropriate client.

3. **Authentication**:
   - The server uses a token-based authentication mechanism to verify clients.

4. **Connection Management**:
   - The server manages connections using a pool of active connections for each user.
   - Connections are established and maintained using a round-robin mechanism for load balancing.

---

## Modules

### cmd

The `cmd` directory contains the entry points for the client and server applications.

#### `cmd/mitserve/main.go`
- Entry point for the server application.
- Initializes the server and starts the reverse proxy and HTTP server.

#### `cmd/mit/main.go`
- Entry point for the client application.
- Initializes the client and connects to the server.

---

### pkg/core

The `pkg/core` directory contains the core logic for the project.

#### `pkg/core/connsvc`
- Implements the connection service (`Service`) that handles reverse connections and HTTP connections.
- Manages authentication and connection resolution.

#### `pkg/core/token`
- Provides utilities for encoding and decoding authentication tokens.
- Tokens are used to securely authenticate clients with the server.

---

### pkg/edge

The `pkg/edge` directory contains the HTTP server implementation.

#### `pkg/edge/httpserver.go`
- Implements an HTTP server that handles incoming HTTP requests.
- Extracts the user ID from the subdomain and forwards the request to the appropriate client.

#### `pkg/edge/httpserver_test.go`
- Contains unit tests for the HTTP server.

---

### pkg/repo

The `pkg/repo` directory contains repositories for managing authentication and connections.

#### `pkg/repo/auth`
- Implements the authentication repository (`Repo`) that verifies user credentials.

#### `pkg/repo/connmng`
- Implements the connection manager (`ConnManager`) that manages user connections.
- Supports adding, removing, and retrieving connections in a thread-safe manner.

---

### pkg/revclient

The `pkg/revclient` directory contains the client implementation.

#### `pkg/revclient/clientserver.go`
- Implements the client-side logic for connecting to the server and forwarding requests to the local service.

---

### pkg/revproxy

The `pkg/revproxy` directory contains the reverse proxy server implementation.

#### `pkg/revproxy/revserver.go`
- Implements the reverse proxy server (`RevServer`) that listens for incoming connections and forwards them to the appropriate client.

---

## Configuration

The project uses a YAML configuration file to define server settings. The default configuration file is located at `runtime/config.yaml`.

### Example Configuration

```yaml
http:
  listen: ":8080"
reverse_proxy:
  listen: ":8081"
auth:
  users:
    user1: test
    user2: test
```
