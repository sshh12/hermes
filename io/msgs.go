package io

import (
	"encoding/json"
	"net"
)

// ClientIntroMsg is passed to server when client initially connects to server
type ClientIntroMsg struct {
	RemotePort int    `json:"port"`
	Token      []byte `json:"token"`
}

// ConnRespMsg is passed to client when the server has an update
type ConnRespMsg struct {
	Rejection  bool `json:"rejection"`
	TunnelPort int  `json:"tunnel_port"`
}

func encodeMsg(msg interface{}) []byte {
	enc, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	enc = append(enc, byte('\n'))
	return enc
}

// WriteMsg writes the given msg to the connection
func WriteMsg(conn net.Conn, msg interface{}) {
	conn.Write(encodeMsg(msg))
}
