package io

import (
	"context"
	"io"
	"net"
)

func copy(ctx context.Context, cancel context.CancelFunc, conn net.Conn, conn2 net.Conn) {
	io.Copy(conn, NewCtxReader(ctx, conn2))
	cancel()
}

func pipe(conn net.Conn, conn2 net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	go copy(ctx, cancel, conn, conn2)
	go copy(ctx, cancel, conn2, conn)
}
