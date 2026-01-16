---
title: "PHP-FPM Monitoring"
description: "Monitor PHP-FPM pools and OPcache directly via FastCGI without exposing data through web servers"
weight: 33
---

# PHP-FPM Monitoring

This guide explains how to monitor PHP-FPM pools and OPcache directly using the FastCGI protocol, without exposing sensitive data through web server proxies.

## Why Direct FastCGI Access?

### Security Concerns with HTTP Proxy

The traditional approach to PHP-FPM monitoring involves configuring Nginx or Apache to proxy status endpoints:

```nginx
# ⚠️ Security risk - exposes internal data via HTTP
location /fpm-status {
    fastcgi_pass unix:/var/run/php-fpm.sock;
    fastcgi_param SCRIPT_NAME /status;
    include fastcgi_params;
    allow 127.0.0.1;
    deny all;
}
```

**Problems with this approach:**

| Issue | Risk |
|-------|------|
| Exposed endpoint | Even with IP restrictions, the endpoint exists on the web server |
| Configuration drift | Status path might accidentally be exposed in production |
| Attack surface | Additional HTTP endpoint increases attack surface |
| ACL complexity | Managing allow/deny lists across environments |

### Direct FastCGI Benefits

Using fcgx to connect directly to PHP-FPM:

- **No HTTP exposure**: Status data never touches the web server
- **Process isolation**: Monitoring runs as a separate service
- **Simpler security**: Only the monitoring service needs socket access
- **No web server config**: No Nginx/Apache configuration needed

## PHP-FPM Status Endpoint

See [PHP-FPM Configuration](../php-fpm-configuration) for complete setup including:
- Basic status endpoint configuration
- Admin socket (`pm.status_listen`) for high-load monitoring
- Multi-pool configuration

### Querying Status

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/cboxdk/fcgx"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Query status with JSON output and full process list
    params := map[string]string{
        "SCRIPT_NAME":     "/status",
        "SCRIPT_FILENAME": "/status",
        "QUERY_STRING":    "json&full",
        "REQUEST_METHOD":  "GET",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var status FPMStatus
    if err := fcgx.ReadJSON(resp, &status); err != nil {
        panic(err)
    }

    fmt.Printf("Pool: %s\n", status.Pool)
    fmt.Printf("Active: %d / Total: %d\n", status.ActiveProcesses, status.TotalProcesses)
    fmt.Printf("Accepted connections: %d\n", status.AcceptedConn)
}

type FPMStatus struct {
    Pool               string `json:"pool"`
    ProcessManager     string `json:"process manager"`
    StartTime          int64  `json:"start time"`
    StartSince         int64  `json:"start since"`
    AcceptedConn       int64  `json:"accepted conn"`
    ListenQueue        int64  `json:"listen queue"`
    MaxListenQueue     int64  `json:"max listen queue"`
    IdleProcesses      int64  `json:"idle processes"`
    ActiveProcesses    int64  `json:"active processes"`
    TotalProcesses     int64  `json:"total processes"`
    MaxActiveProcesses int64  `json:"max active processes"`
    MaxChildrenReached int64  `json:"max children reached"`
    SlowRequests       int64  `json:"slow requests"`
}
```

### Status Query Parameters

| Parameter | Description |
|-----------|-------------|
| `json` | Return JSON formatted output |
| `full` | Include per-process details |
| `html` | Return HTML formatted output |
| `xml` | Return XML formatted output |

Combine parameters: `?json&full`

## OPcache Monitoring

### The OPcache Scoping Problem

**Critical concept**: OPcache is scoped per PHP-FPM pool. Each pool maintains its own independent OPcache.

This means:

```bash
# ❌ This does NOT show your PHP-FPM pool's OPcache
php -r "print_r(opcache_get_status());"

# The CLI has its own OPcache (usually disabled)
# It cannot see what's cached in your www pool
```

**Why?**

1. Each PHP-FPM pool runs as separate master + worker processes
2. OPcache uses shared memory segments per pool
3. CLI PHP is a completely separate process with its own memory
4. There is no IPC mechanism to query another process's OPcache

### Solution: Execute PHP via FastCGI

To query OPcache for a specific pool, you must execute PHP code within that pool:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/cboxdk/fcgx"
)

func main() {
    // Create a temporary PHP script to query OPcache
    scriptPath := "/tmp/opcache-status.php"
    scriptContent := `<?php
error_reporting(0);
ini_set('display_errors', 0);
header("Status: 200 OK");
header("Content-Type: application/json");
echo json_encode(opcache_get_status());
exit;`

    if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
        panic(err)
    }
    defer os.Remove(scriptPath)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_FILENAME": scriptPath,
        "SCRIPT_NAME":     "/opcache-status.php",
        "REQUEST_METHOD":  "GET",
        "SERVER_SOFTWARE": "fcgx-monitor",
        "REMOTE_ADDR":     "127.0.0.1",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    body, err := fcgx.ReadBody(resp)
    if err != nil {
        panic(err)
    }

    var status OpcacheStatus
    if err := json.Unmarshal(body, &status); err != nil {
        panic(err)
    }

    fmt.Printf("OPcache enabled: %v\n", status.Enabled)
    fmt.Printf("Memory used: %d bytes\n", status.MemoryUsage.UsedMemory)
    fmt.Printf("Hit rate: %.2f%%\n", status.Statistics.HitRate)
    fmt.Printf("Cached scripts: %d\n", status.Statistics.NumCachedScripts)
}

type OpcacheStatus struct {
    Enabled     bool        `json:"opcache_enabled"`
    MemoryUsage MemoryUsage `json:"memory_usage"`
    Statistics  Statistics  `json:"opcache_statistics"`
}

type MemoryUsage struct {
    UsedMemory       uint64  `json:"used_memory"`
    FreeMemory       uint64  `json:"free_memory"`
    WastedMemory     uint64  `json:"wasted_memory"`
    CurrentWastedPct float64 `json:"current_wasted_percentage"`
}

type Statistics struct {
    NumCachedScripts uint64  `json:"num_cached_scripts"`
    Hits             uint64  `json:"hits"`
    Misses           uint64  `json:"misses"`
    HitRate          float64 `json:"opcache_hit_rate"`
    OomRestarts      uint64  `json:"oom_restarts"`
    HashRestarts     uint64  `json:"hash_restarts"`
    ManualRestarts   uint64  `json:"manual_restarts"`
}
```

### Important OPcache Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `opcache_hit_rate` | Cache hit percentage | < 95% |
| `current_wasted_percentage` | Fragmented memory | > 5% |
| `oom_restarts` | Out of memory restarts | > 0 |
| `hash_restarts` | Hash table overflow restarts | > 0 |
| `num_cached_scripts` | Number of cached files | Monitor trend |

### Security Considerations

When deploying the OPcache monitoring PHP script:

1. **File permissions**: Ensure the script is readable by PHP-FPM user
2. **Script location**: Use `/tmp` or a dedicated monitoring directory
3. **Cleanup**: Remove temporary scripts when done
4. **open_basedir**: Ensure the script path is allowed

```go
// Check if script path is accessible
if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
    // Create the script
}
```

## Multi-Pool Monitoring

When monitoring multiple PHP-FPM pools:

```go
type PoolConfig struct {
    Name   string
    Socket string
}

func MonitorPools(ctx context.Context, pools []PoolConfig) map[string]*FPMStatus {
    results := make(map[string]*FPMStatus)

    for _, pool := range pools {
        status, err := getPoolStatus(ctx, pool.Socket)
        if err != nil {
            log.Printf("Failed to get status for %s: %v", pool.Name, err)
            continue
        }
        results[pool.Name] = status
    }

    return results
}

func getPoolStatus(ctx context.Context, socket string) (*FPMStatus, error) {
    client, err := fcgx.DialContext(ctx, "unix", socket)
    if err != nil {
        return nil, err
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_NAME":     "/status",
        "SCRIPT_FILENAME": "/status",
        "QUERY_STRING":    "json&full",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var status FPMStatus
    if err := fcgx.ReadJSON(resp, &status); err != nil {
        return nil, err
    }

    return &status, nil
}
```

## Health Check Endpoints

### Ping Endpoint

PHP-FPM provides a lightweight ping endpoint:

```go
func PingPool(ctx context.Context, socket string) error {
    client, err := fcgx.DialContext(ctx, "unix", socket)
    if err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_NAME":     "/ping",
        "SCRIPT_FILENAME": "/ping",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        return fmt.Errorf("ping failed: %w", err)
    }
    defer resp.Body.Close()

    body, _ := fcgx.ReadBody(resp)
    if string(body) != "pong" {
        return fmt.Errorf("unexpected response: %s", body)
    }

    return nil
}
```

### Kubernetes Probes

```go
// Liveness probe - is PHP-FPM responding?
func LivenessHandler(socket string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        if err := PingPool(ctx, socket); err != nil {
            w.WriteHeader(http.StatusServiceUnavailable)
            fmt.Fprintf(w, "unhealthy: %v", err)
            return
        }

        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "healthy")
    }
}

// Readiness probe - can PHP-FPM handle requests?
func ReadinessHandler(socket string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        status, err := getPoolStatus(ctx, socket)
        if err != nil {
            w.WriteHeader(http.StatusServiceUnavailable)
            fmt.Fprintf(w, "not ready: %v", err)
            return
        }

        // Check if pool has capacity
        if status.ListenQueue > 0 {
            w.WriteHeader(http.StatusServiceUnavailable)
            fmt.Fprintf(w, "queue backlog: %d", status.ListenQueue)
            return
        }

        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "ready")
    }
}
```

## Summary

| Approach | Use Case | Security |
|----------|----------|----------|
| HTTP proxy | Legacy setups, simple deployments | ⚠️ Endpoint exposed |
| Direct FastCGI | Production monitoring, exporters | ✅ No HTTP exposure |
| CLI PHP | Development debugging only | ❌ Cannot see FPM OPcache |

For production PHP-FPM monitoring, direct FastCGI access via fcgx provides the most secure and reliable approach.

## See Also

- [PHP-FPM Configuration](../php-fpm-configuration) - Status endpoints and admin sockets
- [Making Requests](../basic-usage/making-requests) - Core request patterns
- [Error Handling](error-handling) - Handle connection errors
- [API Reference](../api-reference) - Complete API documentation
