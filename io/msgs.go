package io

import (
	"encoding/json"
	"net"
)

type clientIntroMsg struct {
	RemotePort int    `json:"port"`
	Token      []byte `json:"token"`
}

type connRespMsg struct {
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

func writeMsg(conn net.Conn, msg interface{}) {
	conn.Write(encodeMsg(msg))
}
