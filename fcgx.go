// Package fcgx provides a minimal, robust, and modern FastCGI client library for Go.
//
// This package is designed for integrating with PHP-FPM and other FastCGI servers,
// aiming for idiomatic Go code, high testability, and correct protocol handling.
// It supports context, deadlines, timeouts, and structured error handling.
//
// Example usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
//	defer cancel()
//
//	client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
//	if err != nil {
//		panic(err)
//	}
//	defer client.Close()
//
//	params := map[string]string{
//		"SCRIPT_FILENAME": "/usr/share/phpmyadmin/index.php",
//		"SCRIPT_NAME":     "/index.php",
//		"REQUEST_METHOD":  "GET",
//		"SERVER_PROTOCOL": "HTTP/1.1",
//		"REMOTE_ADDR":     "127.0.0.1",
//	}
//
//	resp, err := client.Get(ctx, params)
//	if err != nil {
//		panic(err)
//	}
//	defer resp.Body.Close()
//
//	body, err := fcgx.ReadBody(resp)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println(string(body))
package fcgx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

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

// Config holds configuration options for FastCGI client behavior.
// Zero values provide sensible defaults for most use cases.
type Config struct {
	// MaxWriteSize controls the maximum size of data chunks sent to the FastCGI server.
	// Default: 65500 bytes (slightly under 64KB for protocol safety)
	MaxWriteSize int

	// ConnectTimeout sets the timeout for establishing initial connections.
	// Default: 5 seconds
	ConnectTimeout time.Duration

	// RequestTimeout sets a default timeout for requests when context has no deadline.
	// Default: 30 seconds
	RequestTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults for most use cases
func DefaultConfig() *Config {
	return &Config{
		MaxWriteSize:   65500,
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 30 * time.Second,
	}
}

// wrap enhances errors with contextual information and error classification
func wrap(err, kind error, msg string) error {
	return fmt.Errorf("%w: %s: %v", kind, msg, err)
}

// wrapWithContext enhances errors with additional debugging context
func wrapWithContext(err, kind error, msg string, context map[string]interface{}) error {
	if len(context) == 0 {
		return wrap(err, kind, msg)
	}

	var ctxParts []string
	for k, v := range context {
		ctxParts = append(ctxParts, fmt.Sprintf("%s=%v", k, v))
	}
	contextStr := strings.Join(ctxParts, " ")
	return fmt.Errorf("%w: %s (%s): %v", kind, msg, contextStr, err)
}

// isTimeout checks if an error is timeout-related, including various timeout error types
// that can be returned by the network layer or context cancellation
func isTimeout(err error) bool {
	return errors.Is(err, ErrTimeout) ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		(strings.Contains(err.Error(), "i/o timeout"))
}

// isEOF checks if an error indicates end-of-file, including EOF variations
// that can occur during FastCGI protocol communication
func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF")
}

const (
	// FastCGI protocol constants
	FCGI_HEADER_LEN = 8 // FastCGI record header length in bytes
	fcgiVersion1    = 1 // FastCGI protocol version

	// FastCGI record types
	fcgiBeginRequest = 1 // Begin request record
	fcgiAbortRequest = 2 // Abort request record
	fcgiEndRequest   = 3 // End request record
	fcgiParams       = 4 // Parameters record
	fcgiStdin        = 5 // STDIN data record
	fcgiStdout       = 6 // STDOUT data record
	fcgiStderr       = 7 // STDERR data record

	// FastCGI application roles and status
	fcgiResponder       = 1 // Responder role (handles HTTP requests)
	fcgiRequestComplete = 0 // Request completed successfully

)

// header represents a FastCGI record header as defined in the FastCGI specification
type header struct {
	Version       uint8  // Protocol version (always 1)
	Type          uint8  // Record type (FCGI_BEGIN_REQUEST, FCGI_PARAMS, etc.)
	RequestID     uint16 // Request ID to multiplex multiple requests over one connection
	ContentLength uint16 // Length of the content data that follows this header
	PaddingLength uint8  // Number of padding bytes that follow the content
	Reserved      uint8  // Reserved for future use (always 0)
}

// Client represents a FastCGI client connection.
// It maintains state for communicating with a FastCGI server (typically PHP-FPM).
// All methods are thread-safe and can be called concurrently.
type Client struct {
	conn   net.Conn     // Underlying network connection to FastCGI server
	mu     sync.Mutex   // Protects concurrent access to client state
	reqID  uint16       // Current request ID (incremented for each request)
	closed bool         // Whether the client has been closed
	buf    bytes.Buffer // Reusable buffer for building FastCGI records
	config *Config      // Configuration options for this client
}

// writeRecord constructs and sends a FastCGI record to the server.
// It handles proper header construction, padding calculation, and thread-safe transmission.
func (c *Client) writeRecord(recType uint8, content []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buf.Reset()
	contentLen := len(content)
	padLen := uint8((8 - (contentLen % 8)) % 8)

	h := header{
		Version:       fcgiVersion1,
		Type:          recType,
		RequestID:     c.reqID,
		ContentLength: uint16(contentLen),
		PaddingLength: padLen,
	}

	if err := binary.Write(&c.buf, binary.BigEndian, h); err != nil {
		return wrap(err, ErrWrite, "writing record header")
	}

	if contentLen > 0 {
		c.buf.Write(content)
	}

	if padLen > 0 {
		c.buf.Write(make([]byte, padLen))
	}

	_, err := c.conn.Write(c.buf.Bytes())
	if err != nil {
		if isTimeout(err) {
			return wrap(err, ErrTimeout, "timeout while writing record")
		}
		return wrap(err, ErrWrite, "writing record")
	}
	return nil
}

// writeBeginRequest sends a FCGI_BEGIN_REQUEST record to start a new request
func (c *Client) writeBeginRequest(role uint16, flags uint8) error {
	b := [8]byte{byte(role >> 8), byte(role), flags}
	return c.writeRecord(fcgiBeginRequest, b[:])
}

// encodePair encodes a key-value pair in FastCGI name-value format.
// It handles both short (< 128 bytes) and long (>= 128 bytes) length encoding
// as specified in the FastCGI protocol.
func encodePair(w *bytes.Buffer, k, v string) {
	writeSize := func(size int) {
		if size < 128 {
			w.WriteByte(byte(size))
		} else {
			sz := uint32(size) | (1 << 31)
			_ = binary.Write(w, binary.BigEndian, sz)
		}
	}
	writeSize(len(k))
	writeSize(len(v))
	w.WriteString(k)
	w.WriteString(v)
}

// writePairs encodes and sends name-value pairs as a FastCGI record.
// This is used for sending environment variables and request parameters.
// It uses a buffer pool to reduce memory allocations.
func (c *Client) writePairs(recType uint8, pairs map[string]string) error {
	// Get a buffer from the pool to reduce allocations
	w := bufferPool.Get().(*bytes.Buffer)
	w.Reset()
	defer bufferPool.Put(w)

	for k, v := range pairs {
		encodePair(w, k, v)
	}
	return c.writeRecord(recType, w.Bytes())
}

func (c *Client) DoRequest(ctx context.Context, params map[string]string, body io.Reader) (*http.Response, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, wrap(err, ErrContextCancelled, "context error")
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClientClosed
	}
	c.mu.Unlock()

	// Set deadline from context
	deadline, ok := ctx.Deadline()
	if ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return nil, wrapWithContext(err, ErrWrite, "setting deadline", map[string]interface{}{
				"deadline": deadline.Format(time.RFC3339),
				"reqID":    c.reqID,
			})
		}
		// Reset deadline after request
		defer func() { _ = c.conn.SetDeadline(time.Time{}) }()
	}

	// BEGIN_REQUEST record
	if err := c.writeBeginRequest(uint16(fcgiResponder), 0); err != nil {
		return nil, wrap(err, ErrWrite, "writing begin request")
	}

	// Check context after each major operation
	if err := ctx.Err(); err != nil {
		return nil, wrap(err, ErrContextCancelled, "context error")
	}

	// PARAMS records
	if err := c.writePairs(fcgiParams, params); err != nil {
		return nil, wrap(err, ErrWrite, "writing params")
	}

	// Send terminating empty PARAMS record
	if err := c.writeRecord(fcgiParams, nil); err != nil {
		return nil, wrap(err, ErrWrite, "writing empty params")
	}

	// Check context after params
	if err := ctx.Err(); err != nil {
		return nil, wrap(err, ErrContextCancelled, "context error")
	}

	// STDIN records
	if body != nil {
		bodyBuf := bufferPool.Get().(*bytes.Buffer)
		bodyBuf.Reset()
		defer bufferPool.Put(bodyBuf)

		if _, err := io.Copy(bodyBuf, body); err != nil {
			return nil, wrap(err, ErrRead, "reading request body")
		}
		data := bodyBuf.Bytes()

		total := len(data)
		offset := 0
		for offset < total {
			// Check context before each chunk
			if err := ctx.Err(); err != nil {
				return nil, wrap(err, ErrContextCancelled, "context error")
			}

			chunkSize := total - offset
			if chunkSize > c.config.MaxWriteSize {
				chunkSize = c.config.MaxWriteSize
			}
			chunk := data[offset : offset+chunkSize]
			if err := c.writeRecord(fcgiStdin, chunk); err != nil {
				return nil, wrap(err, ErrWrite, "writing stdin chunk")
			}
			offset += chunkSize
		}
	}

	// Always send terminating empty STDIN record
	if err := c.writeRecord(fcgiStdin, nil); err != nil {
		return nil, wrap(err, ErrWrite, "writing empty stdin")
	}

	// Read response - use buffer pool for better memory management
	respBuf := bufferPool.Get().(*bytes.Buffer)
	respBuf.Reset()
	defer bufferPool.Put(respBuf)
	endRequestReceived := false

	for {
		// Check context before each read
		if err := ctx.Err(); err != nil {
			return nil, wrap(err, ErrContextCancelled, "context error")
		}

		h := header{}
		if err := binary.Read(c.conn, binary.BigEndian, &h); err != nil {
			if isEOF(err) {
				if respBuf.Len() > 0 && endRequestReceived {
					break
				}
				return nil, wrap(err, ErrUnexpectedEOF, "unexpected EOF while reading header")
			}
			if isTimeout(err) {
				return nil, wrap(err, ErrTimeout, "timeout while reading header")
			}
			return nil, wrap(err, ErrRead, "reading response header")
		}

		if h.Type == fcgiStdout || h.Type == fcgiStderr {
			b := make([]byte, h.ContentLength)
			if _, err := io.ReadFull(c.conn, b); err != nil {
				if isTimeout(err) {
					return nil, wrap(err, ErrTimeout, "timeout while reading response body")
				}
				return nil, wrap(err, ErrRead, "reading response body")
			}
			respBuf.Write(b)

			if h.PaddingLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.PaddingLength)); err != nil {
					if isTimeout(err) {
						return nil, wrap(err, ErrTimeout, "timeout while reading padding")
					}
					return nil, wrap(err, ErrRead, "reading padding")
				}
			}
		} else if h.Type == fcgiEndRequest {
			endRequestReceived = true
			if h.ContentLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.ContentLength)); err != nil {
					if isTimeout(err) {
						return nil, wrap(err, ErrTimeout, "timeout while reading end request body")
					}
					return nil, wrap(err, ErrRead, "reading end request body")
				}
			}
			if h.PaddingLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.PaddingLength)); err != nil {
					if isTimeout(err) {
						return nil, wrap(err, ErrTimeout, "timeout while reading end request padding")
					}
					return nil, wrap(err, ErrRead, "reading end request padding")
				}
			}
			if respBuf.Len() > 0 {
				break
			}
		}
	}

	resp, err := parseHTTPResponse(respBuf)
	if err != nil {
		return nil, wrap(err, ErrInvalidResponse, "parsing HTTP response")
	}
	return resp, nil
}

func parseHTTPResponse(buf *bytes.Buffer) (*http.Response, error) {
	reader := bufio.NewReader(buf)
	tp := textproto.NewReader(reader)

	line, err := tp.ReadLine()
	if err != nil {
		if isEOF(err) {
			err = ErrUnexpectedEOF
		}
		return nil, err
	}
	// If missing HTTP headers, fallback to plain-text body, but parse simple MIME headers if present
	if !strings.HasPrefix(line, "HTTP/") && !strings.HasPrefix(line, "Status:") {
		// Attempt to parse MIME headers if present
		headers := http.Header{}
		if strings.Contains(line, ":") {
			headersParts := []string{line}
			for {
				hline, err := tp.ReadLine()
				if err != nil {
					break
				}
				if hline == "" {
					break
				}
				headersParts = append(headersParts, hline)
			}
			for _, h := range headersParts {
				if parts := strings.SplitN(h, ":", 2); len(parts) == 2 {
					headers.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}

		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     headers,
			Body:       io.NopCloser(reader),
		}, nil
	}
	// Handle status lines without protocol, e.g., "Status: 200 OK"
	if strings.HasPrefix(line, "Status: ") {
		line = "HTTP/1.1 " + strings.TrimPrefix(line, "Status: ")
	}
	if i := strings.IndexByte(line, ' '); i == -1 {
		return nil, wrap(fmt.Errorf("malformed HTTP response %q", line), ErrInvalidResponse, "malformed HTTP response")
	} else {
		resp := new(http.Response)
		resp.Proto = line[:i]
		resp.Status = strings.TrimLeft(line[i+1:], " ")

		statusCode := resp.Status
		if i := strings.IndexByte(resp.Status, ' '); i != -1 {
			statusCode = resp.Status[:i]
		}
		if len(statusCode) != 3 {
			return nil, wrap(fmt.Errorf("malformed HTTP status code %q", statusCode), ErrInvalidResponse, "malformed HTTP status code")
		}
		resp.StatusCode, err = strconv.Atoi(statusCode)
		if err != nil || resp.StatusCode < 0 {
			return nil, wrap(fmt.Errorf("invalid HTTP status code %q", statusCode), ErrInvalidResponse, "invalid HTTP status code")
		}

		var ok bool
		if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
			return nil, wrap(fmt.Errorf("malformed HTTP version %q", resp.Proto), ErrInvalidResponse, "malformed HTTP version")
		}

		// Headers
		mimeHeader, err := tp.ReadMIMEHeader()
		if err != nil {
			if isEOF(err) {
				err = ErrUnexpectedEOF
			}
			return nil, err
		}

		resp.Header = http.Header(mimeHeader)
		resp.TransferEncoding = resp.Header["Transfer-Encoding"]
		resp.ContentLength, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

		if chunked(resp.TransferEncoding) {
			resp.Body = io.NopCloser(httputil.NewChunkedReader(reader))
		} else {
			resp.Body = io.NopCloser(reader)
		}

		return resp, nil
	}
}

func (c *Client) Get(ctx context.Context, params map[string]string) (*http.Response, error) {
	params["REQUEST_METHOD"] = "GET"
	params["CONTENT_LENGTH"] = "0"
	return c.DoRequest(ctx, params, nil)
}

func (c *Client) Post(ctx context.Context, params map[string]string, body io.Reader, contentLength int) (*http.Response, error) {
	params["REQUEST_METHOD"] = "POST"
	params["CONTENT_LENGTH"] = strconv.Itoa(contentLength)
	if _, ok := params["CONTENT_TYPE"]; !ok {
		params["CONTENT_TYPE"] = "application/x-www-form-urlencoded"
	}

	// Ensure we have a valid body reader
	if body == nil {
		body = bytes.NewReader(nil)
	}

	// If body is a string reader, ensure it's properly formatted
	if sr, ok := body.(*strings.Reader); ok {
		buf := make([]byte, sr.Len())
		_, _ = sr.Read(buf)
		body = bytes.NewReader(buf)
	}

	return c.DoRequest(ctx, params, body)
}

func chunked(te []string) bool {
	return len(te) > 0 && te[0] == "chunked"
}

// Dial establishes a connection to the FastCGI server at the specified network address
// using default configuration options.
func Dial(network, address string) (*Client, error) {
	return DialWithConfig(network, address, DefaultConfig())
}

// DialWithConfig establishes a connection to the FastCGI server with custom configuration.
func DialWithConfig(network, address string, config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	dialer := net.Dialer{
		Timeout: config.ConnectTimeout,
	}
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, wrap(err, ErrConnect, "dialing connection")
	}
	return &Client{conn: conn, reqID: 1, config: config}, nil
}

// ReadBody reads and returns the actual response body as a []byte.
// It also strips any HTTP headers if present (as in FastCGI/PHP-FPM responses).
// It closes the response body after reading.
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Look for double CRLF (end of headers)
	if idx := bytes.Index(all, []byte("\r\n\r\n")); idx != -1 {
		return all[idx+4:], nil
	}
	// If not found, return all
	return all, nil
}

// ReadJSON reads and unmarshals the actual response body as JSON into out.
// It also strips any HTTP headers if present (as in FastCGI/PHP-FPM responses).
// It closes the response body after reading.
func ReadJSON(resp *http.Response, out any) error {
	b, err := ReadBody(resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// DialContext establishes a connection to the FastCGI server at the specified network address
// with the given context using default configuration.
func DialContext(ctx context.Context, network, address string) (*Client, error) {
	return DialContextWithConfig(ctx, network, address, DefaultConfig())
}

// DialContextWithConfig establishes a connection to the FastCGI server with context and custom configuration.
func DialContextWithConfig(ctx context.Context, network, address string, config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	dialer := net.Dialer{
		Timeout: config.ConnectTimeout,
	}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, wrap(err, ErrConnect, "dialing connection with context")
	}
	return &Client{conn: conn, reqID: 1, config: config}, nil
}

// Close closes the FastCGI connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.conn.Close()
}
