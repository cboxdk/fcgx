---
title: "PHP-FPM Configuration"
description: "Configure PHP-FPM for monitoring with status endpoints, admin sockets, and OPcache access"
weight: 4
---

# PHP-FPM Configuration

Before using fcgx to monitor PHP-FPM, you need to enable the status endpoints in your PHP-FPM configuration.

## Basic Status Configuration

Edit your pool configuration (`/etc/php/8.x/fpm/pool.d/www.conf`):

```ini
; Enable status endpoint
pm.status_path = /status

; Enable ping endpoint for health checks
ping.path = /ping
ping.response = pong
```

Restart PHP-FPM after changes:

```bash
sudo systemctl restart php8.x-fpm
```

## Admin Socket for High-Load Sites

**Critical for production**: Under high load, all PHP-FPM workers may be busy processing requests. This makes the status endpoint unresponsive - exactly when you need it most.

### The Problem

```
Main pool (busy)          Status request
┌─────────────────┐       ┌─────────────┐
│ Worker 1: busy  │       │             │
│ Worker 2: busy  │  ───► │  BLOCKED    │
│ Worker 3: busy  │       │  (waiting)  │
│ Worker 4: busy  │       │             │
└─────────────────┘       └─────────────┘
```

When all workers are handling long-running requests, status queries queue behind them.

### The Solution: pm.status_listen

PHP-FPM 7.4+ provides `pm.status_listen` - a dedicated socket for status requests:

```ini
; Main application socket
listen = /var/run/php-fpm.sock

; Dedicated admin socket for monitoring (separate invisible pool)
pm.status_listen = /var/run/php-fpm-status.sock

; Status endpoints
pm.status_path = /status
ping.path = /ping
ping.response = pong
```

```
Main pool (busy)          Admin socket (always available)
┌─────────────────┐       ┌─────────────────┐
│ Worker 1: busy  │       │                 │
│ Worker 2: busy  │       │  Status: OK     │  ◄── Monitoring
│ Worker 3: busy  │       │  Response: 2ms  │
│ Worker 4: busy  │       │                 │
└─────────────────┘       └─────────────────┘
```

The admin socket creates an invisible pool that handles status requests independently.

### Using Admin Socket with fcgx

```go
// Use the dedicated admin socket for monitoring
client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm-status.sock")
if err != nil {
    return err
}
defer client.Close()

params := map[string]string{
    "SCRIPT_NAME":     "/status",
    "SCRIPT_FILENAME": "/status",
    "QUERY_STRING":    "json&full",
}

// This will respond even when main pool is at 100% capacity
resp, err := client.Get(ctx, params)
```

### Socket Permissions

Ensure your monitoring service can access the admin socket:

```ini
; In pool config
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

; Admin socket inherits same permissions
pm.status_listen = /var/run/php-fpm-status.sock
```

Or add your monitoring user to the `www-data` group:

```bash
sudo usermod -aG www-data monitoring-user
```

## Complete Pool Configuration

A production-ready pool configuration for monitoring:

```ini
[www]
; Main socket for application traffic
listen = /var/run/php-fpm.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

; Process manager settings
pm = dynamic
pm.max_children = 50
pm.start_servers = 5
pm.min_spare_servers = 5
pm.max_spare_servers = 35

; === MONITORING CONFIGURATION ===

; Dedicated admin socket (always responsive)
pm.status_listen = /var/run/php-fpm-status.sock

; Status and health endpoints
pm.status_path = /status
ping.path = /ping
ping.response = pong

; Slow request logging (useful for debugging)
slowlog = /var/log/php-fpm/www-slow.log
request_slowlog_timeout = 5s

; Request termination (prevents runaway processes)
request_terminate_timeout = 300s
```

## Multiple Pools

For sites with multiple PHP-FPM pools, configure each with its own admin socket:

```ini
[www]
listen = /var/run/php-fpm-www.sock
pm.status_listen = /var/run/php-fpm-www-status.sock
pm.status_path = /status

[api]
listen = /var/run/php-fpm-api.sock
pm.status_listen = /var/run/php-fpm-api-status.sock
pm.status_path = /status

[admin]
listen = /var/run/php-fpm-admin.sock
pm.status_listen = /var/run/php-fpm-admin-status.sock
pm.status_path = /status
```

Monitor all pools:

```go
pools := []struct {
    Name   string
    Socket string
}{
    {"www", "/var/run/php-fpm-www-status.sock"},
    {"api", "/var/run/php-fpm-api-status.sock"},
    {"admin", "/var/run/php-fpm-admin-status.sock"},
}

for _, pool := range pools {
    status, err := getPoolStatus(ctx, pool.Socket)
    if err != nil {
        log.Printf("[%s] Error: %v", pool.Name, err)
        continue
    }
    log.Printf("[%s] Active: %d, Idle: %d", pool.Name, status.ActiveProcesses, status.IdleProcesses)
}
```

## TCP Sockets

For remote monitoring or containerized environments:

```ini
; TCP socket instead of Unix socket
listen = 127.0.0.1:9000

; Admin socket on different port
pm.status_listen = 127.0.0.1:9001
```

```go
// Connect via TCP
client, err := fcgx.DialContext(ctx, "tcp", "127.0.0.1:9001")
```

**Security note**: Only bind to localhost or use firewall rules. Never expose PHP-FPM directly to the internet.

## Status Output Formats

The status endpoint supports multiple output formats via query string:

| Query String | Format | Use Case |
|--------------|--------|----------|
| (none) | Plain text | Human readable |
| `?json` | JSON | API consumption |
| `?json&full` | JSON with processes | Detailed monitoring |
| `?xml` | XML | Legacy systems |
| `?html` | HTML | Browser viewing |

## Verification

Test your configuration:

```bash
# Check PHP-FPM config syntax
php-fpm -t

# Verify sockets exist after restart
ls -la /var/run/php-fpm*.sock

# Test with fcgx (see Quickstart)
```

## Next Steps

- [Quickstart](quickstart) - Make your first request
- [PHP-FPM Monitoring](advanced-usage/php-fpm-monitoring) - Detailed monitoring guide including OPcache
- [Error Handling](advanced-usage/error-handling) - Handle connection errors
