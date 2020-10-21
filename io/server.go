package io

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Server a hermes server
type Server struct {
	port          int
	host          string
	portPool      map[int]bool
	incomingConns chan net.Conn
	portMux       sync.Mutex
}

// NewServer creates a server
func NewServer(metaPort int) (*Server, error) {
	pool := make(map[int]bool)
	for k := 1; k < 1000; k++ {
		pool[metaPort+k] = true
	}
	server := &Server{port: metaPort, host: "0.0.0.0", portPool: pool, incomingConns: make(chan net.Conn, 10)}
	return server, nil
}

// Start the server
func (srv *Server) Start() error {
	listenAddr := fmt.Sprintf("%s:%d", srv.host, srv.port)
	log.WithField("server", listenAddr).Info("Server started")
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			log.Print(err)
		}
		go srv.handleClient(clientConn)
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

func (srv *Server) handleClient(clientConn net.Conn) {
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
		go (func() {
			waitForConns := true
			for waitForConns {
				select {
				case inConn := <-inRemoteConns:
					port := srv.genAndLockPort()
					log.WithField("tunnelPort", port).Debug("Serving tunnel")
					funnelAddr := fmt.Sprintf("%s:%d", srv.host, port)
					clientConn.Write([]byte(fmt.Sprint(port) + "\n"))
					go pipeIncoming(funnelAddr, inConn, func() {
						srv.releasePort(port)
					})
				case <-ctx.Done():
					waitForConns = false
				}
			}
		})()
	}
	cancel()
	if portLocked != -1 {
		srv.releasePort(portLocked)
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
				break
			default:
				conn, err := ln.Accept()
				if err != nil {
					log.Error(err)
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

func pipeIncoming(listenAddr string, inConn net.Conn, done func()) {
	ln, err := net.Listen("tcp", listenAddr)
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
