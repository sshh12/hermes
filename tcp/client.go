package io

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	hio "github.com/sshh12/hermes/io"
)

// ClientOption applies client options
type ClientOption func(*Client)

type hermesDialer func(*net.TCPAddr) (net.Conn, error)

// ClientAuthenticator generates a password for a connection
type ClientAuthenticator func(int) string

const clientTokenSize = 256

// Client a hermes client
type Client struct {
	appPort    int
	serverAddr *net.TCPAddr
	localHost  string
	remoteHost string
	remotePort int
	token      []byte
	tlsCfg     *tls.Config
	restarts   bool
	auth       ClientAuthenticator
}

// NewClient creates a client
func NewClient(appPort int, remotePort int, remoteHost string, opts ...ClientOption) (*Client, error) {
	token := make([]byte, clientTokenSize)
	rand.Read(token)
	client := &Client{
		appPort:    appPort,
		remoteHost: remoteHost,
		remotePort: remotePort,
		token:      token,
		localHost:  "127.0.0.1",
		restarts:   false,
		auth:       func(port int) string { return "" },
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

// WithRestarts will auto restart on failure
func WithRestarts() ClientOption {
	return func(c *Client) {
		c.restarts = true
	}
}

// WithLocalHost update the local IP to forward to
func WithLocalHost(host string) ClientOption {
	return func(c *Client) {
		c.localHost = host
	}
}

// WithPassword sets client password
func WithPassword(password string) ClientOption {
	return func(c *Client) {
		c.auth = func(port int) string { return password }
	}
}

// Start the client
func (c *Client) Start() error {
	log.WithFields(log.Fields{
		"server":     c.serverAddr,
		"remotePort": c.remotePort,
		"appPort":    c.appPort,
		"encrypted":  c.tlsCfg != nil,
		"restarts":   c.restarts,
	}).Info("Client started")
	var serverConn net.Conn
	var err error
	var dialHermes hermesDialer
	for {
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
			if !c.restarts {
				return err
			}
			log.Error(err)
		}
		if err := c.startWithConn(serverConn, dialHermes); err != nil {
			if !c.restarts {
				return err
			}
			log.Error(err)
		}
		// prevent oof
		time.Sleep(2 * time.Second)
	}
}

func (c *Client) startWithConn(serverConn net.Conn, dialHermes hermesDialer) error {
	hio.WriteMsg(serverConn, hio.ClientIntroMsg{
		RemotePort: c.remotePort,
		Token:      c.token,
		Password:   c.auth(c.remotePort),
	})
	reader := bufio.NewReader(serverConn)
	for {
		resp, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err)
			break
		}
		var msg hio.ConnRespMsg
		if err := json.Unmarshal([]byte(resp), &msg); err != nil {
			log.Error(err)
			break
		}
		if msg.Rejection {
			log.WithField("remotePort", c.remotePort).Error("Server rejected binding")
			return fmt.Errorf("Server rejected binding: " + msg.RejectionMsg)
		}
		tunAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", c.remoteHost, msg.TunnelPort))
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
		go pipeClientConn(tunAddr, appAddr, c.token, dialHermes)
	}
	return nil
}

func pipeClientConn(tunAddr *net.TCPAddr, appAddr *net.TCPAddr, token []byte, dialHermes hermesDialer) {
	connA, err := dialHermes(tunAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	connA.Write(token)
	connB, err := net.DialTCP("tcp", nil, appAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	hio.Pipe(connA, connB)
}

func verifyToken(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
