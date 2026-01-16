---
title: "Configuration"
description: "Customize fcgx behavior with timeouts, buffer sizes, and connection settings"
weight: 31
---

# Configuration

fcgx provides a `Config` struct to customize client behavior.

## Default Configuration

```go
config := fcgx.DefaultConfig()
// MaxWriteSize:   65500 bytes
// ConnectTimeout: 5 seconds
// RequestTimeout: 30 seconds
```

## Config Options

| Option | Default | Description |
|--------|---------|-------------|
| `MaxWriteSize` | 65500 | Maximum chunk size for STDIN data |
| `ConnectTimeout` | 5s | Timeout for establishing connections |
| `RequestTimeout` | 30s | Default timeout when context has no deadline |

## Custom Configuration

```go
config := &fcgx.Config{
    MaxWriteSize:   32768,        // 32KB chunks
    ConnectTimeout: 2 * time.Second,
    RequestTimeout: 10 * time.Second,
}

client, err := fcgx.DialWithConfig("unix", "/var/run/php-fpm.sock", config)
if err != nil {
    return err
}
defer client.Close()
```

## With Context

```go
config := &fcgx.Config{
    ConnectTimeout: 2 * time.Second,
    RequestTimeout: 10 * time.Second,
}

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

client, err := fcgx.DialContextWithConfig(ctx, "unix", "/var/run/php-fpm.sock", config)
```

## Timeout Behavior

### Connect Timeout

Controls how long to wait when establishing a connection:

```go
config := &fcgx.Config{
    ConnectTimeout: 500 * time.Millisecond, // Fail fast
}
```

### Request Timeout

Used when the context has no deadline:

```go
config := &fcgx.Config{
    RequestTimeout: 60 * time.Second, // Long-running scripts
}

// This request uses RequestTimeout (60s)
resp, err := client.Get(context.Background(), params)

// This request uses context deadline (5s)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
resp, err := client.Get(ctx, params)
```

## MaxWriteSize

Controls chunking for large request bodies:

```go
// For memory-constrained environments
config := &fcgx.Config{
    MaxWriteSize: 16384, // 16KB chunks
}

// For high-throughput scenarios
config := &fcgx.Config{
    MaxWriteSize: 65500, // Maximum (default)
}
```

## Production Example

```go
func NewFPMClient(socketPath string) (*fcgx.Client, error) {
    config := &fcgx.Config{
        MaxWriteSize:   65500,
        ConnectTimeout: 2 * time.Second,
        RequestTimeout: 30 * time.Second,
    }

    ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
    defer cancel()

    return fcgx.DialContextWithConfig(ctx, "unix", socketPath, config)
}
```

## Next Steps

- [Error Handling](error-handling) - Handle errors gracefully
- [API Reference](../api-reference) - Complete API documentation
