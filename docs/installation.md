---
title: "Installation"
description: "Install fcgx in your Go project using go get and verify your PHP-FPM connection"
weight: 2
---

# Installation

## Requirements

- Go 1.21 or later
- Access to a FastCGI server (e.g., PHP-FPM)

## Install with Go Modules

```bash
go get github.com/cboxdk/fcgx
```

## Import in Your Code

```go
import "github.com/cboxdk/fcgx"
```

## Verify Installation

Create a simple test file to verify the installation:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/cboxdk/fcgx"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    client, err := fcgx.DialContext(ctx, "tcp", "127.0.0.1:9000")
    if err != nil {
        fmt.Printf("Connection test: %v\n", err)
        return
    }
    defer client.Close()

    fmt.Println("Successfully connected to FastCGI server!")
}
```

Run with:

```bash
go run main.go
```

## Connection Types

fcgx supports both Unix sockets and TCP connections:

```go
// Unix socket (recommended for local PHP-FPM)
client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")

// TCP connection
client, err := fcgx.DialContext(ctx, "tcp", "127.0.0.1:9000")
```

## Next Steps

- [PHP-FPM Configuration](php-fpm-configuration) - Configure status endpoints and admin sockets
- [Quickstart](quickstart) - Make your first FastCGI request
- [Configuration](advanced-usage/configuration) - Customize fcgx client settings
