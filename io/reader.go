package io

import (
	"context"
	"io"
)

type readerCtx struct {
	ctx context.Context
	r   io.Reader
}

func (r *readerCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// NewCtxReader gets a context-aware io.Reader.
// https://pace.dev/blog/2020/02/03/context-aware-ioreader-for-golang-by-mat-ryer.html
func NewCtxReader(ctx context.Context, r io.Reader) io.Reader {
	return &readerCtx{
		ctx: ctx,
		r:   r,
	}
}
