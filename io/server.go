package io

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

// Server a hermes server
type Server struct {
	port          int
	host          string
	k             int
	incomingConns chan net.Conn
	portMux       sync.Mutex
}

// NewServer creates a server
func NewServer(metaPort int) (*Server, error) {
	server := &Server{port: metaPort, host: "0.0.0.0", k: 4050, incomingConns: make(chan net.Conn, 10)}
	return server, nil
}

// Start the server
func (srv *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", srv.host, srv.port))
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
	return srv.k
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
		fmt.Println(remotePort)
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
				funnelAddr := fmt.Sprintf("%s:%d", srv.host, port)
				clientConn.Write([]byte(fmt.Sprint(port) + "\n"))
				go pipeIncoming(funnelAddr, inConn)
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

func pipeIncoming(listenAddr string, inConn net.Conn) {
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
}
