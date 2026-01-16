---
title: "Making Requests"
description: "Learn how to make GET, POST, and custom FastCGI requests"
weight: 11
---

# Making Requests

fcgx provides several methods for making FastCGI requests to PHP-FPM.

## Connecting to PHP-FPM

### Unix Socket (Recommended)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
if err != nil {
    return err
}
defer client.Close()
```

### TCP Connection

```go
client, err := fcgx.DialContext(ctx, "tcp", "127.0.0.1:9000")
```

## GET Requests

Use `Get()` for simple requests without a body:

```go
params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/index.php",
    "SCRIPT_NAME":     "/index.php",
    "REQUEST_METHOD":  "GET",
    "SERVER_PROTOCOL": "HTTP/1.1",
    "REMOTE_ADDR":     "127.0.0.1",
    "QUERY_STRING":    "page=1&limit=10",
}

resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()
```

## POST Requests

Use `Post()` for requests with a body:

```go
import "strings"

data := "username=admin&password=secret"

params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/login.php",
    "SCRIPT_NAME":     "/login.php",
    "CONTENT_TYPE":    "application/x-www-form-urlencoded",
}

resp, err := client.Post(ctx, params, strings.NewReader(data), len(data))
if err != nil {
    return err
}
defer resp.Body.Close()
```

### JSON POST

```go
import (
    "bytes"
    "encoding/json"
)

payload := map[string]interface{}{
    "name":  "John",
    "email": "john@example.com",
}

jsonData, _ := json.Marshal(payload)

params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/api.php",
    "SCRIPT_NAME":     "/api.php",
    "CONTENT_TYPE":    "application/json",
}

resp, err := client.Post(ctx, params, bytes.NewReader(jsonData), len(jsonData))
```

## Custom Requests

Use `DoRequest()` for full control:

```go
params := map[string]string{
    "SCRIPT_FILENAME": "/var/www/html/api.php",
    "SCRIPT_NAME":     "/api.php",
    "REQUEST_METHOD":  "PUT",
    "CONTENT_TYPE":    "application/json",
    "CONTENT_LENGTH":  strconv.Itoa(len(body)),
}

resp, err := client.DoRequest(ctx, params, bytes.NewReader(body))
```

## Required Parameters

These parameters are typically required for PHP-FPM:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `SCRIPT_FILENAME` | Full path to PHP file | `/var/www/html/index.php` |
| `SCRIPT_NAME` | URL path to script | `/index.php` |
| `REQUEST_METHOD` | HTTP method | `GET`, `POST`, `PUT` |

## Optional Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `QUERY_STRING` | URL query parameters | `page=1&sort=name` |
| `CONTENT_TYPE` | Request content type | `application/json` |
| `CONTENT_LENGTH` | Body length | `128` |
| `SERVER_PROTOCOL` | HTTP protocol version | `HTTP/1.1` |
| `REMOTE_ADDR` | Client IP address | `127.0.0.1` |
| `HTTP_HOST` | Host header | `example.com` |

## PHP-FPM Status

Query PHP-FPM status pages:

```go
params := map[string]string{
    "SCRIPT_NAME":     "/status",
    "SCRIPT_FILENAME": "/status",
    "REQUEST_METHOD":  "GET",
    "QUERY_STRING":    "json",
}

resp, err := client.Get(ctx, params)
```

## Next Steps

- [Reading Responses](reading-responses) - Parse response data
- [Error Handling](../advanced-usage/error-handling) - Handle errors gracefully
