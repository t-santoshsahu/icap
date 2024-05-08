// Copyright 2011 Andy Balholm. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Network connections and request dispatch for the ICAP server.

package icap

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

// Objects implementing the Handler interface can be registered
// to serve ICAP requests.
//
// ServeICAP should write reply headers and data to the ResponseWriter
// and then return.
type Handler interface {
	ServeICAP(ResponseWriter, *Request)
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as ICAP handlers.  If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler object that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeICAP calls f(w, r).
func (f HandlerFunc) ServeICAP(w ResponseWriter, r *Request) {
	f(w, r)
}

// A conn represents the server side of an ICAP connection.
type conn struct {
	remoteAddr string            // network address of remote side
	handler    Handler           // request handler
	rwc        net.Conn          // i/o connection
	buf        *bufio.ReadWriter // buffered rwc
}

// Create new connection from rwc.
func newConn(rwc net.Conn, handler Handler) (c *conn, err error) {
	c = new(conn)
	c.remoteAddr = rwc.RemoteAddr().String()
	c.handler = handler
	c.rwc = rwc
	br := bufio.NewReader(rwc)
	bw := bufio.NewWriter(rwc)
	c.buf = bufio.NewReadWriter(br, bw)

	return c, nil
}

// Read next request from connection.
func (c *conn) readRequest() (w *respWriter, err error) {
	var req *Request
	if req, err = ReadRequest(c.buf); err != nil {
		return nil, err
	}

	req.RemoteAddr = c.remoteAddr

	w = new(respWriter)
	w.conn = c
	w.req = req
	w.header = make(http.Header)
	return w, nil
}

// Close the connection.
func (c *conn) close() {
	if c.buf != nil {
		c.buf.Flush()
		c.buf = nil
	}
	if c.rwc != nil {
		c.rwc.Close()
		c.rwc = nil
	}
}

// Serve a new connection.
func (c *conn) serve() {
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		c.rwc.Close()

		var buf bytes.Buffer
		fmt.Fprintf(&buf, "icap: panic serving %v: %v\n", c.remoteAddr, err)
		buf.Write(debug.Stack())
		log.Print(buf.String())
	}()

	w, err := c.readRequest()
	if err != nil {
		if err != io.ErrUnexpectedEOF {
			log.Println("error while reading request:", err)
		}

		c.rwc.Close()
		return
	}

	c.handler.ServeICAP(w, w.req)
	w.finishRequest()

	c.close()
}

// A Server defines parameters for running an ICAP server.
type Server struct {
	Addr         string  // TCP address to listen on, ":1344" if empty
	Handler      Handler // handler to invoke
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.  If
// srv.Addr is blank, ":1344" is used.
func (srv *Server) ListenAndServe() error {
	addr := srv.Addr
	if addr == "" {
		addr = ":1344"
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return e
	}
	return srv.Serve(l)
}

// ListenAndServeSSL listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.  If
// srv.Addr is blank, ":1344" is used.
func (srv *Server) ListenAndServeSSL(cert, key string) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":1344"
	}
	cer, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		log.Println(err)
		return err
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	ln, err := tls.Listen("tcp", addr, config)
	if err != nil {
		log.Println(err)
		return err
	}
	defer ln.Close()

	return srv.Serve(ln)
}

// Serve accepts incoming connections on the Listener l, creating a
// new service thread for each.  The service threads read requests and
// then call srv.Handler to reply to them.
func (srv *Server) Serve(l net.Listener) error {
	defer l.Close()
	handler := srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}

	for {
		rw, e := l.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				log.Printf("icap: Accept error: %v", e)
				continue
			}
			return e
		}
		if srv.ReadTimeout != 0 {
			rw.SetReadDeadline(time.Now().Add(srv.ReadTimeout))
		}
		if srv.WriteTimeout != 0 {
			rw.SetWriteDeadline(time.Now().Add(srv.WriteTimeout))
		}
		c, err := newConn(rw, handler)
		if err != nil {
			continue
		}
		go c.serve()
	}
	panic("not reached")
}

// Serve accepts incoming ICAP connections on the listener l,
// creating a new service thread for each.  The service threads
// read requests and then call handler to reply to them.
func Serve(l net.Listener, handler Handler) error {
	srv := &Server{Handler: handler}
	return srv.Serve(l)
}

// ListenAndServe listens on the TCP network address addr
// and then calls Serve with handler to handle requests
// on incoming connections.
func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}

func ListenAndServeSSL(addr string, cert, key string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServeSSL(cert, key)
}
