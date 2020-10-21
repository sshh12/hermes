package io

import (
	"fmt"
	"io"
	"net"
	"sync"
)

func copy(from net.Conn, to net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	if _, err := io.Copy(to, from); err != nil {
		fmt.Println(err)
		return
	}
}

func pipe(conn net.Conn, conn2 net.Conn) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go copy(conn, conn2, wg)
	go copy(conn2, conn, wg)
	wg.Wait()
}
