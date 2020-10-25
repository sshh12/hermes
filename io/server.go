package io

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	cmap "github.com/orcaman/concurrent-map"
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
	portPool      cmap.ConcurrentMap
	incomingConns chan net.Conn
	portMux       sync.Mutex
}

// NewServer creates a server
func NewServer(metaPort int, opts ...ServerOption) (*Server, error) {
	portPool := cmap.New()
	server := &Server{
		port:          metaPort,
		host:          "0.0.0.0",
		portPool:      portPool,
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

func (srv *Server) genAndLockPort() int {
	for k := 4500; k < 5000; k++ {
		if srv.portPool.SetIfAbsent(fmt.Sprint(k), true) {
			return k
		}
	}
	return -1
}

func (srv *Server) handleClient(clientConn net.Conn, hermesListener hermesListener) {
	reader := bufio.NewReader(clientConn)
	portLocked := -1
	ctx, cancel := context.WithCancel(context.Background())
	for {
		resp, err := reader.ReadString('\n')
		if err != nil {
			log.WithField("err", err).WithField("portLocked", portLocked).Info("Client connection failed")
			break
		}
		var msg clientIntroMsg
		if err := json.Unmarshal([]byte(resp), &msg); err != nil {
			log.Error(err)
			break
		}
		remotePort := msg.RemotePort
		if !srv.portPool.SetIfAbsent(fmt.Sprint(remotePort), true) {
			log.WithField("remotePort", remotePort).Error("Client requested binding failed")
			writeMsg(clientConn, connRespMsg{Rejection: true})
			break
		}
		portLocked = remotePort
		log.WithField("remotePort", remotePort).Info("Client requested binding")
		inRemoteConns := make(chan net.Conn, 1)
		remoteAddr := fmt.Sprintf("%s:%d", srv.host, remotePort)
		go funnelIncomingConns(ctx, remoteAddr, inRemoteConns)
		go srv.serveTunnels(ctx, clientConn, msg.Token, inRemoteConns, hermesListener)
	}
	cancel()
	if portLocked != -1 {
		srv.portPool.Remove(fmt.Sprint(portLocked))
	}
}

func (srv *Server) serveTunnels(ctx context.Context, clientConn net.Conn, clientToken []byte, inRemoteConns chan net.Conn, hermesListener hermesListener) {
	waitForConns := true
	for waitForConns {
		select {
		case inConn := <-inRemoteConns:
			port := srv.genAndLockPort()
			log.WithField("tunnelPort", port).Debug("Serving tunnel")
			funnelAddr := fmt.Sprintf("%s:%d", srv.host, port)
			writeMsg(clientConn, connRespMsg{TunnelPort: port})
			go pipeIncoming(funnelAddr, inConn, clientToken, hermesListener, func() {
				srv.portPool.Remove(fmt.Sprint(port))
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

func pipeIncoming(listenAddr string, inConn net.Conn, clientToken []byte, hermesListener hermesListener, done func()) {
	ln, err := hermesListener(listenAddr)
	if err != nil {
		log.Print(err)
		return
	}
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		token := make([]byte, clientTokenSize)
		n, err := clientConn.Read(token)
		if err != nil {
			log.Error(err)
			continue
		}
		if n != clientTokenSize || !verifyToken(token, clientToken) {
			log.Error("Client tunnel token invalid")
			continue
		}
		pipe(clientConn, inConn)
		break
	}
	ln.Close()
	done()
}
