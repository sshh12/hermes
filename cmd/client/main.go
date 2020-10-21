package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

func copy(from net.Conn, to net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	if _, err := io.Copy(to, from); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("copy done")
}

func pipe(conn net.Conn, conn2 net.Conn) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go copy(conn, conn2, wg)
	go copy(conn2, conn, wg)
	wg.Wait()
}

func pipeFromPort(from string, forward string) {
	forwardAddr, err := net.ResolveTCPAddr("tcp", forward)
	if err != nil {
		fmt.Println(err)
		return
	}
	fromAddr, err := net.ResolveTCPAddr("tcp", from)
	if err != nil {
		fmt.Println(err)
		return
	}
	connA, err := net.DialTCP("tcp", nil, forwardAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	connB, err := net.DialTCP("tcp", nil, fromAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	pipe(connA, connB)
}

func main() {
	hermAddr, err := net.ResolveTCPAddr("tcp", "161.35.15.163:4000")
	if err != nil {
		fmt.Println(err)
		return
	}
	hermConn, err := net.DialTCP("tcp", nil, hermAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	hermConn.Write([]byte("5202\n"))
	reader := bufio.NewReader(hermConn)
	for {
		val, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("port", val)
		go pipeFromPort("161.35.15.163:"+strings.TrimSpace(val), "127.0.0.1:5201")
	}
}
