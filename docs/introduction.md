---
title: "Introduction"
description: "A minimal, robust, and modern FastCGI client library for Go with context support"
weight: 1
---

# Introduction

**fcgx** is a minimal, robust, and modern FastCGI client library for Go. It is designed for integrating with PHP-FPM and other FastCGI servers, providing idiomatic Go code, high testability, and correct protocol handling.

## Why fcgx?

When working with PHP-FPM from Go applications, you need a reliable way to communicate via the FastCGI protocol. fcgx provides:

- **Context Support**: Full support for Go contexts, deadlines, and timeouts
- **Thread Safety**: All methods are safe for concurrent use
- **Structured Errors**: Sentinel errors for robust error handling with `errors.Is`
- **Memory Efficiency**: Buffer pooling to minimize allocations
- **Standards Compliance**: Correct FastCGI protocol implementation

## Use Cases

fcgx is ideal for:

- **PHP-FPM Monitoring**: Query PHP-FPM status pages and pool metrics
- **Health Checks**: Implement health check endpoints for PHP applications
- **Kubernetes Probes**: Build readiness and liveness probes for PHP workloads
- **Metrics Collection**: Gather performance data from PHP-FPM pools
- **API Gateways**: Route requests to PHP backends from Go services

## Features Overview

| Feature | Description |
|---------|-------------|
| Context Support | Deadlines, timeouts, and cancellation |
| HTTP Helpers | `Get()` and `Post()` convenience methods |
| Response Parsing | Automatic HTTP response parsing |
| JSON Support | Built-in JSON unmarshaling with `ReadJSON()` |
| Configuration | Customizable timeouts and buffer sizes |
| Error Handling | Structured sentinel errors |

## Next Steps

- [Installation](installation) - Install fcgx in your project
- [PHP-FPM Configuration](php-fpm-configuration) - Configure status endpoints and admin sockets
- [Quickstart](quickstart) - Get started in 5 minutes
- [Basic Usage](basic-usage/making-requests) - Learn the core features
