package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
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

func handleConns(conns chan net.Conn) {
	ln, err := net.Listen("tcp", "0.0.0.0:5202")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		conns <- conn
	}
}

var k = 4010

func pipeConns(port int, in net.Conn, conn net.Conn) {
	fmt.Println("solve notify")
	conn.Write([]byte(fmt.Sprint(port) + "\n"))
	ln, err := net.Listen("tcp", "0.0.0.0:"+fmt.Sprint(port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("solve listen")
	conn2, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("solve accept")
	pipe(conn2, in)
}

func solveConns(conns chan net.Conn, conn net.Conn) {
	for {
		in := <-conns
		k++
		go pipeConns(k, in, conn)
	}
}

func main() {
	ln, err := net.Listen("tcp", "0.0.0.0:4000")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	for {
		val, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("port", val)
		inConns := make(chan net.Conn)
		go handleConns(inConns)
		go solveConns(inConns, conn)
	}
}
