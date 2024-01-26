package glidepack

import (
	"io"
)

type Reader struct {
	closed bool
	m      *Manager
	policy PolicyFunc
	p      JobParams
}

func NewReader(r io.Reader) *Reader {
	z := new(Reader)
	z.p.r = r
	z.p.JobType = DECOMPRESS
	z.closed = false
	z.m = GetManager()
	z.p.a = GZIP
	return z
}

func (z *Reader) Read(p []byte) (n int, err error) {
	if z.policy == nil {
		n, z.p.id, err = z.m.SubmitJob(p, z.p)
		return n, err
	} else {
		n, z.p.id, err = z.m.SubmitWithPolicy(p, z.p, z.policy)
		return n, err
	}
}

func (z *Reader) SetPolicy(p PolicyFunc) {
	z.policy = p
}

func (z *Reader) Close() (err error) {
	if z.closed {
		return errClosed
	}
	z.closed = true
	return nil
}

func (z *Reader) Reset(r io.Reader) {
	z.closed = false
	z.p.r = r
}

func (z *Reader) Apply(options ...Option) (err error) {
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
