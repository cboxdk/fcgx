---
title: "Error Handling"
description: "Handle errors gracefully with fcgx sentinel error types and errors.Is"
weight: 32
---

# Error Handling

fcgx provides structured sentinel errors for robust error handling using Go's `errors.Is()`.

## Sentinel Errors

| Error | Description |
|-------|-------------|
| `ErrClientClosed` | Client connection has been closed |
| `ErrTimeout` | Operation timed out |
| `ErrContextCancelled` | Context was cancelled |
| `ErrUnexpectedEOF` | Unexpected end of response |
| `ErrInvalidResponse` | Malformed response from server |
| `ErrPHPFPM` | PHP-FPM specific error |
| `ErrConnect` | Failed to establish connection |
| `ErrWrite` | Failed to write to connection |
| `ErrRead` | Failed to read from connection |

## Using errors.Is

```go
import "errors"

resp, err := client.Get(ctx, params)
if err != nil {
    switch {
    case errors.Is(err, fcgx.ErrTimeout):
        log.Println("Request timed out, retrying...")
        return retry(ctx, params)

    case errors.Is(err, fcgx.ErrContextCancelled):
        log.Println("Request was cancelled")
        return nil

    case errors.Is(err, fcgx.ErrClientClosed):
        log.Println("Client closed, reconnecting...")
        return reconnect()

    case errors.Is(err, fcgx.ErrConnect):
        log.Println("Connection failed, PHP-FPM may be down")
        return err

    default:
        log.Printf("Unexpected error: %v\n", err)
        return err
    }
}
```

## Error Context

Errors include context information for debugging:

```go
resp, err := client.Get(ctx, params)
if err != nil {
    // Error message includes context:
    // "fcgx: timeout: timeout while reading header: i/o timeout"
    fmt.Println(err)
}
```

## Timeout Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

resp, err := client.Get(ctx, params)
if err != nil {
    if errors.Is(err, fcgx.ErrTimeout) {
        // Handle timeout - maybe retry with longer timeout
        log.Println("Request timed out")
    }
}
```

## Connection Errors

```go
client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
if err != nil {
    if errors.Is(err, fcgx.ErrConnect) {
        log.Println("Cannot connect to PHP-FPM")
        log.Println("Check if PHP-FPM is running and socket exists")
    }
    return err
}
```

## Retry Pattern

```go
func requestWithRetry(ctx context.Context, client *fcgx.Client, params map[string]string, maxRetries int) (*http.Response, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        resp, err := client.Get(ctx, params)
        if err == nil {
            return resp, nil
        }

        lastErr = err

        // Only retry on timeout errors
        if !errors.Is(err, fcgx.ErrTimeout) {
            return nil, err
        }

        log.Printf("Attempt %d failed: %v, retrying...", i+1, err)
        time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

## Health Check Example

```go
func checkFPMHealth(socketPath string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    client, err := fcgx.DialContext(ctx, "unix", socketPath)
    if err != nil {
        if errors.Is(err, fcgx.ErrConnect) {
            return fmt.Errorf("php-fpm not responding: %w", err)
        }
        return err
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_NAME":     "/ping",
        "SCRIPT_FILENAME": "/ping",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        if errors.Is(err, fcgx.ErrTimeout) {
            return fmt.Errorf("php-fpm responding slowly: %w", err)
        }
        return err
    }
    defer resp.Body.Close()

    body, _ := fcgx.ReadBody(resp)
    if string(body) != "pong" {
        return fmt.Errorf("unexpected ping response: %s", body)
    }

    return nil
}
```

## Next Steps

- [API Reference](../api-reference) - Complete API documentation
