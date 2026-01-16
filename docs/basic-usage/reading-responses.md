---
title: "Reading Responses"
description: "Parse FastCGI response bodies and JSON data efficiently"
weight: 12
---

# Reading Responses

fcgx returns standard Go `*http.Response` objects, with helper functions for common operations.

## Response Structure

```go
resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

// Access response properties
fmt.Println("Status:", resp.Status)
fmt.Println("Status Code:", resp.StatusCode)
fmt.Println("Content-Type:", resp.Header.Get("Content-Type"))
```

## Reading Raw Body

Use `ReadBody()` to get the response body as bytes:

```go
resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

body, err := fcgx.ReadBody(resp)
if err != nil {
    return err
}

fmt.Println(string(body))
```

`ReadBody()` automatically:
- Reads the entire response body
- Strips HTTP headers if present in the body
- Closes the response body

## Reading JSON

Use `ReadJSON()` to unmarshal JSON responses:

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

var user User
if err := fcgx.ReadJSON(resp, &user); err != nil {
    return err
}

fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
```

### JSON Maps

For dynamic JSON structures:

```go
var data map[string]interface{}
if err := fcgx.ReadJSON(resp, &data); err != nil {
    return err
}

fmt.Printf("Pool: %v\n", data["pool"])
```

## PHP-FPM Status Example

```go
type FPMStatus struct {
    Pool               string `json:"pool"`
    ProcessManager     string `json:"process manager"`
    StartTime          int64  `json:"start time"`
    StartSince         int    `json:"start since"`
    AcceptedConn       int    `json:"accepted conn"`
    ListenQueue        int    `json:"listen queue"`
    MaxListenQueue     int    `json:"max listen queue"`
    ListenQueueLen     int    `json:"listen queue len"`
    IdleProcesses      int    `json:"idle processes"`
    ActiveProcesses    int    `json:"active processes"`
    TotalProcesses     int    `json:"total processes"`
    MaxActiveProcesses int    `json:"max active processes"`
    MaxChildrenReached int    `json:"max children reached"`
    SlowRequests       int    `json:"slow requests"`
}

params := map[string]string{
    "SCRIPT_NAME":     "/status",
    "SCRIPT_FILENAME": "/status",
    "QUERY_STRING":    "json",
}

resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

var status FPMStatus
if err := fcgx.ReadJSON(resp, &status); err != nil {
    return err
}

fmt.Printf("Pool: %s\n", status.Pool)
fmt.Printf("Active: %d / Total: %d\n", status.ActiveProcesses, status.TotalProcesses)
```

## Manual Body Reading

For streaming or custom processing:

```go
import "io"

resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

// Read in chunks
buf := make([]byte, 1024)
for {
    n, err := resp.Body.Read(buf)
    if n > 0 {
        // Process chunk
        fmt.Print(string(buf[:n]))
    }
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
}
```

## Checking Status Codes

```go
resp, err := client.Get(ctx, params)
if err != nil {
    return err
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    body, _ := fcgx.ReadBody(resp)
    return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
}
```

## Next Steps

- [Error Handling](../advanced-usage/error-handling) - Handle errors gracefully
- [Configuration](../advanced-usage/configuration) - Customize timeouts
