package io

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// ServerOption applies server options
type ServerOption func(*Server)

type hermesListener func(listenAddr string) (net.Listener, error)

// Server a hermes server
type Server struct {
	port          int
	tlsPort       int
	tlsCfg        *tls.Config
	host          string
	portPool      map[int]bool
	incomingConns chan net.Conn
	portMux       sync.Mutex
}

// NewServer creates a server
func NewServer(metaPort int, opts ...ServerOption) (*Server, error) {
	pool := make(map[int]bool)
	for k := 4500; k < 5000; k++ {
		pool[k] = true
	}
	server := &Server{
		port:          metaPort,
		host:          "0.0.0.0",
		portPool:      pool,
		incomingConns: make(chan net.Conn, 10),
	}
	for _, opt := range opts {
		opt(server)
	}
	return server, nil
}

// WithServerTLS enables TLS
func WithServerTLS(port int, tlsCfg *tls.Config) ServerOption {
	return func(srv *Server) {
		srv.tlsPort = port
		srv.tlsCfg = tlsCfg
	}
}

// Start the server
func (srv *Server) Start() error {
	errChan := make(chan error)
	if srv.tlsCfg != nil {
		go srv.startWithListener(srv.tlsPort, errChan, func(listenAddr string) (net.Listener, error) {
			return tls.Listen("tcp", listenAddr, srv.tlsCfg)
		})
	}
	go srv.startWithListener(srv.port, errChan, func(listenAddr string) (net.Listener, error) {
		return net.Listen("tcp", listenAddr)
	})
	return <-errChan
}

func (srv *Server) startWithListener(port int, errChan chan<- error, listener hermesListener) {
	listenAddr := fmt.Sprintf("%s:%d", srv.host, port)
	log.WithField("server", listenAddr).Info("Server started")
	ln, err := listener(listenAddr)
	if err != nil {
		errChan <- err
		return
	}
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Print(err)
		}
		go srv.handleClient(clientConn, listener)
	}
}

func (srv *Server) lockPort(port int) bool {
	srv.portMux.Lock()
	defer srv.portMux.Unlock()
	if val, ok := srv.portPool[port]; ok {
		if val {
			srv.portPool[port] = false
			return true
		}
		return false
	}
	srv.portPool[port] = false
	return true
}

func (srv *Server) genAndLockPort() int {
	srv.portMux.Lock()
	defer srv.portMux.Unlock()
	var port int = -1
	for key, val := range srv.portPool {
		if val {
			port = key
			break
		}
	}
	if port == -1 {
		log.Fatal("ran out of ports")
	}
	srv.portPool[port] = false
	log.WithField("port", port).Debug("Port locked")
	return port
}

func (srv *Server) releasePort(port int) {
	srv.portMux.Lock()
	defer srv.portMux.Unlock()
	srv.portPool[port] = true
	log.WithField("port", port).Debug("Port released")
}

func (srv *Server) handleClient(clientConn net.Conn, hermesListener hermesListener) {
	reader := bufio.NewReader(clientConn)
	portLocked := -1
	ctx, cancel := context.WithCancel(context.Background())
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.WithField("err", err).WithField("portLocked", portLocked).Info("Client disconnected")
			break
		}
		remotePort, err := strconv.Atoi(strings.TrimSpace(msg))
		if err != nil {
			log.Error(err)
			break
		}
		if !srv.lockPort(remotePort) {
			log.WithField("remotePort", remotePort).Error("Client requested binding failed")
			clientConn.Write([]byte("reject\n"))
			break
		}
		portLocked = remotePort
		log.WithField("remotePort", remotePort).Info("Client requested binding")
		inRemoteConns := make(chan net.Conn, 1)
		remoteAddr := fmt.Sprintf("%s:%d", srv.host, remotePort)
		go funnelIncomingConns(ctx, remoteAddr, inRemoteConns)
		go srv.serveTunnels(ctx, clientConn, inRemoteConns, hermesListener)
	}
	cancel()
	if portLocked != -1 {
		srv.releasePort(portLocked)
	}
}

func (srv *Server) serveTunnels(ctx context.Context, clientConn net.Conn, inRemoteConns chan net.Conn, hermesListener hermesListener) {
	waitForConns := true
	for waitForConns {
		select {
		case inConn := <-inRemoteConns:
			port := srv.genAndLockPort()
			log.WithField("tunnelPort", port).Debug("Serving tunnel")
			funnelAddr := fmt.Sprintf("%s:%d", srv.host, port)
			clientConn.Write([]byte(fmt.Sprint(port) + "\n"))
			go pipeIncoming(funnelAddr, inConn, hermesListener, func() {
				srv.releasePort(port)
			})
		case <-ctx.Done():
			waitForConns = false
		}
	}
}

func funnelIncomingConns(ctx context.Context, listenAddr string, conns chan<- net.Conn) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Print(err)
		return
	}
	initConns := make(chan net.Conn, 0)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				initConns <- conn
			}
		}
	}()
	waitForConns := true
	for waitForConns {
		select {
		case conn := <-initConns:
			conns <- conn
		case <-ctx.Done():
			waitForConns = false
		}
	}
	ln.Close()
}

func pipeIncoming(listenAddr string, inConn net.Conn, hermesListener hermesListener, done func()) {
	ln, err := hermesListener(listenAddr)
	if err != nil {
		log.Print(err)
		return
	}
	clientConn, err := ln.Accept()
	if err != nil {
		log.Print(err)
		return
	}
	pipe(clientConn, inConn)
	ln.Close()
	done()
}
