package glidepack

import (
	"errors"
	"io"
)

var (
	errClosed = errors.New("writer already closed")
)

const (
	DEFAULT_LEVEL = 1
)

type Writer struct {
	w      io.Writer
	closed bool
	m      *Manager
	policy PolicyFunc
	p      JobParams
}

func NewWriter(w io.Writer) *Writer {
	z := new(Writer)
	z.p.w = w
	z.p.JobType = COMPRESS
	z.closed = false
	z.m = GetManager()
	z.p.a = GZIP
	z.p.level = DEFAULT_LEVEL
	return z
}

func (z *Writer) Write(p []byte) (n int, err error) {
	if z.closed {
		return 0, errClosed
	}
	if z.policy == nil {
		n, z.p.id, err = z.m.SubmitJob(p, z.p)
		return n, err
	} else {
		n, z.p.id, err = z.m.SubmitWithPolicy(p, z.p, z.policy)
		return n, err
	}
}

func (z *Writer) SetPolicy(p PolicyFunc) {
	z.policy = p
}

func (z *Writer) Close() (err error) {
	if z.closed {
		return errClosed
	}
	z.closed = true
	return nil
}

func (z *Writer) Reset(w io.Writer) {
	z.w = w
	z.closed = false
}

// Apply options to Writer
func (z *Writer) Apply(options ...Option) (err error) {
	if z.closed {
		return errClosed
	}

	for _, op := range options {
		if err = op(z); err != nil {
			return
		}
	}
	return
}
