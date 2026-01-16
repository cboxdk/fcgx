---
title: "Quickstart"
description: "Get started with fcgx in 5 minutes - make your first FastCGI request"
weight: 3
---

# Quickstart

This guide will have you making FastCGI requests in under 5 minutes.

## Basic Example

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/cboxdk/fcgx"
)

func main() {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Connect to PHP-FPM
    client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Set up FastCGI parameters
    params := map[string]string{
        "SCRIPT_FILENAME": "/var/www/html/index.php",
        "SCRIPT_NAME":     "/index.php",
        "REQUEST_METHOD":  "GET",
        "SERVER_PROTOCOL": "HTTP/1.1",
        "REMOTE_ADDR":     "127.0.0.1",
    }

    // Make the request
    resp, err := client.Get(ctx, params)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    // Read the response body
    body, err := fcgx.ReadBody(resp)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(body))
}
```

## Query PHP-FPM Status

A common use case is querying PHP-FPM status:

```go
params := map[string]string{
    "SCRIPT_NAME":     "/status",
    "SCRIPT_FILENAME": "/status",
    "REQUEST_METHOD":  "GET",
    "QUERY_STRING":    "json",
}

resp, err := client.Get(ctx, params)
if err != nil {
    panic(err)
}
defer resp.Body.Close()

var status map[string]interface{}
if err := fcgx.ReadJSON(resp, &status); err != nil {
    panic(err)
}

fmt.Printf("Pool: %s\n", status["pool"])
fmt.Printf("Active processes: %v\n", status["active processes"])
```

## POST Request

Send data to PHP scripts:

```go
import (
    "strings"
)

data := "name=John&email=john@example.com"
params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/form.php",
    "SCRIPT_NAME":     "/form.php",
    "CONTENT_TYPE":    "application/x-www-form-urlencoded",
}

resp, err := client.Post(ctx, params, strings.NewReader(data), len(data))
if err != nil {
    panic(err)
}
defer resp.Body.Close()
```

## Key Concepts

| Concept | Description |
|---------|-------------|
| Context | Controls timeouts and cancellation |
| Params | FastCGI environment variables |
| Response | Standard `*http.Response` |
| Body | Use `ReadBody()` or `ReadJSON()` |

## Next Steps

- [PHP-FPM Configuration](php-fpm-configuration) - Configure status endpoints and admin sockets
- [Making Requests](basic-usage/making-requests) - Learn request patterns
- [Reading Responses](basic-usage/reading-responses) - Handle response data
- [Error Handling](advanced-usage/error-handling) - Handle errors gracefully
