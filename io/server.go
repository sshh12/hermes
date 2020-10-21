package io

import (
	"bufio"
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

func (srv *Server) genPort() int {
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
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		remotePort, err := strconv.Atoi(strings.TrimSpace(msg))
		log.WithField("remotePort", remotePort).Info("Client requested binding")
		if err != nil {
			fmt.Println(err)
			return
		}
		inRemoteConns := make(chan net.Conn)
		remoteAddr := fmt.Sprintf("%s:%d", srv.host, remotePort)
		go funnelIncomingConns(remoteAddr, inRemoteConns)
		go (func() {
			for {
				inConn := <-inRemoteConns
				port := srv.genPort()
				log.WithField("tunnelPort", port).Debug("Serving tunnel")
				funnelAddr := fmt.Sprintf("%s:%d", srv.host, port)
				clientConn.Write([]byte(fmt.Sprint(port) + "\n"))
				go pipeIncoming(funnelAddr, inConn, func() {
					srv.releasePort(port)
				})
			}
		})()
	}
}

func funnelIncomingConns(listenAddr string, conns chan<- net.Conn) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Print(err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			return
		}
		conns <- conn
	}
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
