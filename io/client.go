package io

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ClientOption applies client options
type ClientOption func(*Client)

type hermesDialer func(*net.TCPAddr) (net.Conn, error)

// Client a hermes client
type Client struct {
	appPort    int
	serverAddr *net.TCPAddr
	localHost  string
	remoteHost string
	remotePort int
	tlsCfg     *tls.Config
}

// NewClient creates a client
func NewClient(appPort int, remotePort int, remoteHost string, opts ...ClientOption) (*Client, error) {
	client := &Client{
		appPort:    appPort,
		remoteHost: remoteHost,
		remotePort: remotePort,
		localHost:  "127.0.0.1",
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

// WithServerAddress sets a custom hermes address
func WithServerAddress(addr *net.TCPAddr) ClientOption {
	return func(c *Client) {
		c.serverAddr = addr
	}
}

// WithTLS sets up a TLS connection
func WithTLS(addr *net.TCPAddr, tlsCfg *tls.Config) ClientOption {
	return func(c *Client) {
		c.serverAddr = addr
		c.tlsCfg = tlsCfg
	}
}

// Start the client
func (c *Client) Start() error {
	log.WithFields(log.Fields{
		"server":     c.serverAddr,
		"remotePort": c.remotePort,
		"appPort":    c.appPort,
		"encrypted":  c.tlsCfg != nil,
	}).Info("Client started")
	var serverConn net.Conn
	var err error
	var dialHermes hermesDialer
	if c.tlsCfg != nil {
		serverConn, err = tls.Dial("tcp", c.serverAddr.String(), c.tlsCfg)
		dialHermes = func(tunAddr *net.TCPAddr) (net.Conn, error) {
			return tls.Dial("tcp", tunAddr.String(), c.tlsCfg)
		}
	} else {
		serverConn, err = net.DialTCP("tcp", nil, c.serverAddr)
		dialHermes = func(tunAddr *net.TCPAddr) (net.Conn, error) {
			return net.DialTCP("tcp", nil, tunAddr)
		}
	}
	if err != nil {
		return err
	}
	return c.startWithConn(serverConn, dialHermes)
}

func (c *Client) startWithConn(serverConn net.Conn, dialHermes hermesDialer) error {
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
		go pipeClientConn(tunAddr, appAddr, dialHermes)
	}
	return nil
}

func pipeClientConn(tunAddr *net.TCPAddr, appAddr *net.TCPAddr, dialHermes hermesDialer) {
	connA, err := dialHermes(tunAddr)
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
