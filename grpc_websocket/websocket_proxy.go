package grpc_websocket

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/context"
	"pkg.moe/pkg/logger"
)

// MethodOverrideParam defines the special URL parameter that is translated into the subsequent proxied streaming http request's method.
//
// Deprecated: it is preferable to use the Options parameters to WebSocketProxy to supply parameters.
var MethodOverrideParam = "method"

// TokenCookieName defines the cookie name that is translated to an 'Authorization: Bearer' header in the streaming http request's headers.
//
// Deprecated: it is preferable to use the Options parameters to WebSocketProxy to supply parameters.
var TokenCookieName = "token"

// RequestMutatorFunc can supply an alternate outgoing request.
type RequestMutatorFunc func(incoming *http.Request, outgoing *http.Request) *http.Request

// Proxy provides websocket transport upgrade to compatible endpoints.
type Proxy struct {
	h                      http.Handler
	maxRespBodyBufferBytes int
	methodOverrideParam    string
	tokenCookieName        string
	requestMutator         RequestMutatorFunc
	headerForwarder        func(header string) bool
	pingInterval           time.Duration
	pingWait               time.Duration
	pongWait               time.Duration
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		p.h.ServeHTTP(w, r)
		return
	}
	p.proxy(w, r)
}

// Option allows customization of the proxy.
type Option func(*Proxy)

// WithMaxRespBodyBufferSize allows specification of a custom size for the
// buffer used while reading the response body. By default, the bufio.Scanner
// used to read the response body sets the maximum token size to MaxScanTokenSize.
func WithMaxRespBodyBufferSize(nBytes int) Option {
	return func(p *Proxy) {
		p.maxRespBodyBufferBytes = nBytes
	}
}

// WithMethodParamOverride allows specification of the special http parameter that is used in the proxied streaming request.
func WithMethodParamOverride(param string) Option {
	return func(p *Proxy) {
		p.methodOverrideParam = param
	}
}

// WithTokenCookieName allows specification of the cookie that is supplied as an upstream 'Authorization: Bearer' http header.
func WithTokenCookieName(param string) Option {
	return func(p *Proxy) {
		p.tokenCookieName = param
	}
}

// WithRequestMutator allows a custom RequestMutatorFunc to be supplied.
func WithRequestMutator(fn RequestMutatorFunc) Option {
	return func(p *Proxy) {
		p.requestMutator = fn
	}
}

// WithForwardedHeaders allows controlling which headers are forwarded.
func WithForwardedHeaders(fn func(header string) bool) Option {
	return func(p *Proxy) {
		p.headerForwarder = fn
	}
}

// WithPingControl allows specification of ping pong control. The interval
// parameter specifies the pingInterval between pings. The allowed wait time
// for a pong response is (pingInterval * 10) / 9.
func WithPingControl(interval time.Duration) Option {
	return func(proxy *Proxy) {
		proxy.pingInterval = interval
		proxy.pongWait = (interval * 10) / 9
		proxy.pingWait = proxy.pongWait / 6
	}
}

var defaultHeadersToForward = map[string]bool{
	"Origin":       true,
	"origin":       true,
	"Referer":      true,
	"referer":      true,
	"Content-Type": true,
}

func defaultHeaderForwarder(header string) bool {
	return defaultHeadersToForward[header]
}

// WebsocketProxy attempts to expose the underlying handler as a bidi websocket stream with newline-delimited
// JSON as the content encoding.
//
// The HTTP Authorization header is either populated from the Sec-Websocket-Protocol field or by a cookie.
// The cookie name is specified by the TokenCookieName value.
//
// example:
//
//	Sec-Websocket-Protocol: Bearer, foobar
//
// is converted to:
//
//	Authorization: Bearer foobar
//
// Method can be overwritten with the MethodOverrideParam get parameter in the requested URL
func WebsocketProxy(h http.Handler, opts ...Option) http.Handler {
	p := &Proxy{
		h:                   h,
		methodOverrideParam: MethodOverrideParam,
		tokenCookieName:     TokenCookieName,
		headerForwarder:     defaultHeaderForwarder,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// TODO(tmc): allow modification of upgrader settings?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func isClosedConnError(err error) bool {
	str := err.Error()
	if strings.Contains(str, "use of closed network connection") {
		return true
	}
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
}

func (p *Proxy) proxy(w http.ResponseWriter, r *http.Request) {
	var responseHeader http.Header
	// If Sec-WebSocket-Protocol starts with "Bearer", respond in kind.
	// TODO(tmc): consider customizability/extension point here.
	if strings.HasPrefix(r.Header.Get("Sec-WebSocket-Protocol"), "Bearer") {
		responseHeader = http.Header{
			"Sec-WebSocket-Protocol": []string{"Bearer"},
		}
	}
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		logger.Get().Warnln("error upgrading websocket:", err)
		return
	}
	defer conn.Close()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	requestBodyR, requestBodyW := io.Pipe()
	request, err := http.NewRequestWithContext(r.Context(), "POST", r.URL.String(), requestBodyR)
	if err != nil {
		logger.Get().Warnln("error preparing request:", err)
		return
	}
	if swsp := r.Header.Get("Sec-WebSocket-Protocol"); swsp != "" {
		request.Header.Set("Authorization", transformSubProtocolHeader(swsp))
	}
	for header := range r.Header {
		if p.headerForwarder(header) {
			request.Header.Set(header, r.Header.Get(header))
		}
	}
	// If token cookie is present, populate Authorization header from the cookie instead.
	if cookie, err := r.Cookie(p.tokenCookieName); err == nil {
		request.Header.Set("Authorization", "Bearer "+cookie.Value)
	}
	if m := r.URL.Query().Get(p.methodOverrideParam); m != "" {
		request.Method = m
	}

	if p.requestMutator != nil {
		request = p.requestMutator(r, request)
	}

	responseBodyR, responseBodyW := io.Pipe()
	response := newInMemoryResponseWriter(responseBodyW)
	go func() {
		<-ctx.Done()
		logger.Get().Debugln("closing pipes")
		requestBodyW.CloseWithError(io.EOF)
		responseBodyW.CloseWithError(io.EOF)
		response.closed <- true
	}()

	go func() {
		defer cancelFn()
		p.h.ServeHTTP(response, request)
	}()

	// read loop -- take messages from websocket and write to http request
	go func() {
		if p.pingInterval > 0 && p.pingWait > 0 && p.pongWait > 0 {
			conn.SetReadDeadline(time.Now().Add(p.pongWait))
			conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(p.pongWait)); return nil })
		}
		defer func() {
			cancelFn()
		}()
		for {
			select {
			case <-ctx.Done():
				logger.Get().Debugln("read loop done")
				return
			default:
			}
			logger.Get().Debugln("[read] reading from socket.")
			_, payload, err := conn.ReadMessage()
			if err != nil {
				if isClosedConnError(err) {
					logger.Get().Debugln("[read] websocket closed:", err)
					return
				}
				logger.Get().Warnln("error reading websocket message:", err)
				return
			}
			logger.Get().Debugln("[read] read payload:", string(payload), payload)

			// 0b10000000, 0x00, 0x00, 0x00, 0x00
			if len(payload) == 5 && payload[0] == 0b10000000 && payload[1] == 0x00 && payload[2] == 0x00 && payload[3] == 0x00 && payload[4] == 0x00 {
				logger.Get().Warnln("[read] client closed conn")
				return
			}

			logger.Get().Debugln("[read] writing to requestBody:")
			n, err := requestBodyW.Write(payload)
			//requestBodyW.Write([]byte("\n"))
			logger.Get().Debugln("[read] wrote to requestBody", n)
			if err != nil {
				logger.Get().Warnln("[read] error writing message to upstream http server:", err)
				return
			}
		}
	}()
	// ping write loop
	if p.pingInterval > 0 && p.pingWait > 0 && p.pongWait > 0 {
		go func() {
			ticker := time.NewTicker(p.pingInterval)
			defer func() {
				ticker.Stop()
				conn.Close()
			}()
			for {
				select {
				case <-ctx.Done():
					logger.Get().Debugln("ping loop done")
					return
				case <-ticker.C:
					conn.SetWriteDeadline(time.Now().Add(p.pingWait))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}
		}()
	}
	// write loop -- take messages from response and write to websocket
	scanner := bufio.NewScanner(responseBodyR)

	sep := []byte{0b11111111, 0b10000000, 0b11111111, 0b10000000, 0b11111111, 0b10000001}
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.Index(data, sep); i >= 0 {
			return i + len(sep), data[0:i], nil
		}

		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	})

	// if maxRespBodyBufferSize has been specified, use custom buffer for scanner
	var scannerBuf []byte
	if p.maxRespBodyBufferBytes > 0 {
		scannerBuf = make([]byte, 0, 64*1024)
		scanner.Buffer(scannerBuf, p.maxRespBodyBufferBytes)
	}

	// send header
	header := []byte{0b10000000, 0x00, 0x00, 0x00, 0x00}
	if err = conn.WriteMessage(websocket.BinaryMessage, header); err != nil {
		logger.Get().Warnln("[write] error writing websocket header:", err)
		return
	}

	var writer io.WriteCloser
	var needSendCount uint32
	var sendCount uint32
	var sendBuffer = make([]byte, 0, 400000)
	var recvCount uint32

	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) != 0 {
			sendBuffer = append(sendBuffer, data...)
			recvCount += uint32(len(data))
		}

		if len(sendBuffer) > 0 && (sendBuffer[0] == 0b00000000 || (len(sendBuffer) > 1 && sendBuffer[0] == '\n' && sendBuffer[1] == 0b00000000)) && sendCount == 0 {
			if sendBuffer[0] == '\n' {
				sendBuffer = sendBuffer[1:]
				recvCount -= 1
			}

			if len(sendBuffer) < 5 {
				logger.Get().Warnln("[write] invalid message", sendBuffer)
				continue
			}

			needSendCount = binary.BigEndian.Uint32([]byte{sendBuffer[1], sendBuffer[2], sendBuffer[3], sendBuffer[4]}) + 5
			logger.Get().Debugf("new head sendBuffer: %v, needSendCount: %d", sendBuffer[0:5], needSendCount)

			writer, err = conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				logger.Get().Warnln("[write] error writing websocket message:", err)
				return
			}
		}

		if needSendCount-sendCount > 0 {
			if recvCount >= needSendCount-sendCount {
				tempBuffer := sendBuffer[0 : needSendCount-sendCount]
				sendBuffer = sendBuffer[needSendCount-sendCount:]
				recvCount -= needSendCount - sendCount
				writer.Write(tempBuffer)
				sendCount += uint32(len(tempBuffer))
			} else {
				if recvCount != 0 {
					tempBuffer := sendBuffer[0:recvCount]
					sendBuffer = sendBuffer[recvCount:]
					recvCount -= recvCount
					writer.Write(tempBuffer)
					sendCount += uint32(len(tempBuffer))
				}
			}
		}

		logger.Get().Debugf("sendCount: %d, needSendCount: %d, recvCount: %d", sendCount, needSendCount, recvCount)

		if sendCount == needSendCount {
			writer.Close()
			sendCount = 0
			needSendCount = 0
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Get().Warnln("scanner err:", err)
	}
}

type inMemoryResponseWriter struct {
	io.Writer
	header http.Header
	code   int
	closed chan bool
}

func newInMemoryResponseWriter(w io.Writer) *inMemoryResponseWriter {
	return &inMemoryResponseWriter{
		Writer: w,
		header: http.Header{},
		closed: make(chan bool, 1),
	}
}

// IE and Edge do not delimit Sec-WebSocket-Protocol strings with spaces
func transformSubProtocolHeader(header string) string {
	tokens := strings.SplitN(header, "Bearer,", 2)

	if len(tokens) < 2 {
		return ""
	}

	return fmt.Sprintf("Bearer %v", strings.Trim(tokens[1], " "))
}

func (w *inMemoryResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
func (w *inMemoryResponseWriter) Header() http.Header {
	return w.header
}
func (w *inMemoryResponseWriter) WriteHeader(code int) {
	w.code = code
}
func (w *inMemoryResponseWriter) CloseNotify() <-chan bool {
	return w.closed
}
func (w *inMemoryResponseWriter) Flush() {}
