package io

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Client a hermes client
type Client struct {
	appPort    int
	serverAddr *net.TCPAddr
	localHost  string
	remoteHost string
	remotePort int
}

// NewClient creates a client
func NewClient(appPort int, remotePort int, serverPort int, remoteHost string) (*Client, error) {
	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", remoteHost, serverPort))
	if err != nil {
		return nil, err
	}
	client := &Client{
		appPort:    appPort,
		serverAddr: serverAddr,
		remoteHost: remoteHost,
		remotePort: remotePort,
		localHost:  "127.0.0.1",
	}
	return client, nil
}

// Start the client
func (c *Client) Start() error {
	log.WithField("server", c.serverAddr).Info("Client started")
	serverConn, err := net.DialTCP("tcp", nil, c.serverAddr)
	if err != nil {
		return err
	}
	serverConn.Write([]byte(fmt.Sprintf("%d\n", c.remotePort)))
	reader := bufio.NewReader(serverConn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err)
			break
		}
		msg = strings.TrimSpace(msg)
		if msg == "reject" {
			log.WithField("remotePort", c.remotePort).Error("Server rejected binding")
			return fmt.Errorf("Server rejected binding")
		}
		tunPort, err := strconv.Atoi(msg)
		if err != nil {
			log.Error(err)
			continue
		}
		tunAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", c.remoteHost, tunPort))
		if err != nil {
			log.Error(err)
			continue
		}
		appAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", c.localHost, c.appPort))
		if err != nil {
			log.Error(err)
			continue
		}
		log.WithField("tunAddr", tunAddr).WithField("appAddr", appAddr).Debug("Tunneling")
		go pipeClientConn(tunAddr, appAddr)
	}
	return nil
}

func pipeClientConn(tunAddr *net.TCPAddr, appAddr *net.TCPAddr) {
	connA, err := net.DialTCP("tcp", nil, tunAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	connB, err := net.DialTCP("tcp", nil, appAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	pipe(connA, connB)
}
