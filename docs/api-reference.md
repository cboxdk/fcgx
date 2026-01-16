---
title: "API Reference"
description: "Complete API documentation for all fcgx types, functions, and methods"
weight: 90
---

# API Reference

Complete reference documentation for the fcgx package.

## Types

### Client

```go
type Client struct {
    // contains filtered or unexported fields
}
```

Client represents a FastCGI client connection. All methods are thread-safe.

### Config

```go
type Config struct {
    // MaxWriteSize controls the maximum size of data chunks sent to the FastCGI server.
    // Default: 65500 bytes
    MaxWriteSize int

    // ConnectTimeout sets the timeout for establishing initial connections.
    // Default: 5 seconds
    ConnectTimeout time.Duration

    // RequestTimeout sets a default timeout for requests when context has no deadline.
    // Default: 30 seconds
    RequestTimeout time.Duration
}
```

## Connection Functions

### Dial

```go
func Dial(network, address string) (*Client, error)
```

Establishes a connection using default configuration.

**Parameters:**
- `network`: Network type (`"tcp"` or `"unix"`)
- `address`: Server address or socket path

**Example:**
```go
client, err := fcgx.Dial("unix", "/var/run/php-fpm.sock")
```

### DialContext

```go
func DialContext(ctx context.Context, network, address string) (*Client, error)
```

Establishes a connection with context support.

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

client, err := fcgx.DialContext(ctx, "tcp", "127.0.0.1:9000")
```

### DialWithConfig

```go
func DialWithConfig(network, address string, config *Config) (*Client, error)
```

Establishes a connection with custom configuration.

### DialContextWithConfig

```go
func DialContextWithConfig(ctx context.Context, network, address string, config *Config) (*Client, error)
```

Establishes a connection with context and custom configuration.

### DefaultConfig

```go
func DefaultConfig() *Config
```

Returns a Config with sensible defaults.

## Client Methods

### Get

```go
func (c *Client) Get(ctx context.Context, params map[string]string) (*http.Response, error)
```

Performs a GET request. Sets `REQUEST_METHOD` to `"GET"` and `CONTENT_LENGTH` to `"0"`.

**Example:**
```go
params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/index.php",
    "SCRIPT_NAME":     "/index.php",
}

resp, err := client.Get(ctx, params)
```

### Post

```go
func (c *Client) Post(ctx context.Context, params map[string]string, body io.Reader, contentLength int) (*http.Response, error)
```

Performs a POST request with body data.

**Parameters:**
- `ctx`: Context for timeout/cancellation
- `params`: FastCGI parameters
- `body`: Request body reader
- `contentLength`: Length of body data

**Example:**
```go
data := "name=John"
resp, err := client.Post(ctx, params, strings.NewReader(data), len(data))
```

### DoRequest

```go
func (c *Client) DoRequest(ctx context.Context, params map[string]string, body io.Reader) (*http.Response, error)
```

Performs a custom FastCGI request with full control over parameters.

### Close

```go
func (c *Client) Close() error
```

Closes the FastCGI connection.

## Response Helpers

### ReadBody

```go
func ReadBody(resp *http.Response) ([]byte, error)
```

Reads the response body, strips HTTP headers if present, and closes the body.

**Example:**
```go
body, err := fcgx.ReadBody(resp)
if err != nil {
    return err
}
fmt.Println(string(body))
```

### ReadJSON

```go
func ReadJSON(resp *http.Response, out any) error
```

Reads and unmarshals the response body as JSON.

**Example:**
```go
var data map[string]interface{}
if err := fcgx.ReadJSON(resp, &data); err != nil {
    return err
}
```

## Errors

### Sentinel Errors

```go
var (
    ErrClientClosed     = errors.New("fcgx: client closed")
    ErrTimeout          = errors.New("fcgx: timeout")
    ErrContextCancelled = errors.New("fcgx: context cancelled")
    ErrUnexpectedEOF    = errors.New("fcgx: unexpected EOF")
    ErrInvalidResponse  = errors.New("fcgx: invalid response")
    ErrPHPFPM           = errors.New("fcgx: php-fpm error")
    ErrConnect          = errors.New("fcgx: connect error")
    ErrWrite            = errors.New("fcgx: write error")
    ErrRead             = errors.New("fcgx: read error")
)
```

Use `errors.Is()` to check error types:

```go
if errors.Is(err, fcgx.ErrTimeout) {
    // Handle timeout
}
```

## Constants

```go
const (
    FCGI_HEADER_LEN = 8 // FastCGI record header length in bytes
)
```

## Complete Example

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/cboxdk/fcgx"
)

func main() {
    config := &fcgx.Config{
        ConnectTimeout: 2 * time.Second,
        RequestTimeout: 10 * time.Second,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    client, err := fcgx.DialContextWithConfig(ctx, "unix", "/var/run/php-fpm.sock", config)
    if err != nil {
        if errors.Is(err, fcgx.ErrConnect) {
            fmt.Println("Cannot connect to PHP-FPM")
        }
        panic(err)
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_NAME":     "/status",
        "SCRIPT_FILENAME": "/status",
        "QUERY_STRING":    "json",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        if errors.Is(err, fcgx.ErrTimeout) {
            fmt.Println("Request timed out")
        }
        panic(err)
    }
    defer resp.Body.Close()

    var status map[string]interface{}
    if err := fcgx.ReadJSON(resp, &status); err != nil {
        panic(err)
    }

    fmt.Printf("Pool: %v\n", status["pool"])
    fmt.Printf("Active processes: %v\n", status["active processes"])
}
```
