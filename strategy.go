package glidepack

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/intel/qatgo/qatzip"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"

	isal "github.com/intel/ISALgo"
	ixl "github.com/intel/ixl-go/compress"
)

type Algorithm int
type QatMode int

const (
	DIRECT QatMode = iota
	STREAM
)

const (
	DEFLATE Algorithm = iota
	GZIP
	LZ4
	ZSTD
)

const (
	MAX_QAT_BINDINGS      = 8
	MAX_IAA_BINDINGS      = 8
	DEFAULT_OUT_BUFFER_SZ = 1024 * 10
)

var (
	IAA_ALGORITHMS     = []Algorithm{DEFLATE, GZIP}
	QAT_ALGORITHMS     = []Algorithm{DEFLATE, GZIP, ZSTD}
	ISAL_ALGORITHMS    = []Algorithm{GZIP}
	DEFAULT_ALGORITHMS = []Algorithm{DEFLATE, LZ4, GZIP, ZSTD}
)

var (
	ErrNotAvailable = errors.New("compression strategy not available")
	ErrNotInstalled = errors.New("strategy not installed on the system")
	ErrJobNotFound  = errors.New("unable to find job with ID")
	ErrUnsupported  = errors.New("option is unsupported by this handler")
)

func (alg Algorithm) isValid() bool {
	switch alg {
	case DEFLATE, GZIP, LZ4, ZSTD:
		return true
	}
	return false
}

func (a Algorithm) String() (str string) {
	switch a {
	case DEFLATE:
		str = "deflate"
	case GZIP:
		str = "gzip"
	case LZ4:
		str = "lz4"
	case ZSTD:
		str = "zstd"
	}
	return str
}
func (a Algorithm) GetQATSymbol() (qatzip.Algorithm, error) {
	switch a {
	case DEFLATE:
		return qatzip.DEFLATE, nil
	case GZIP:
		return qatzip.DEFLATE, nil
	case LZ4:
		return qatzip.LZ4, nil
	case ZSTD:
		return qatzip.ZSTD, nil
	}
	return 0, errors.New("invalid algorithm type")
}

type StrategyType int

const (
	QAT StrategyType = iota
	ISAL
	IAA
	DEFAULT
)

func (s StrategyType) String() string {
	switch s {
	case QAT:
		return "QAT"
	case ISAL:
		return "ISAL"
	case IAA:
		return "IAA"
	case DEFAULT:
		return "default"
	}
	return "default"
}

func (s StrategyType) IsValid() bool {
	switch s {
	case QAT, ISAL, IAA, DEFAULT:
		return true
	}
	return false
}

var strategyBank = []StrategyType{QAT, ISAL, IAA, DEFAULT}

func GetStrategies() []StrategyType {
	return strategyBank
}

type Handler interface {
	Request(job *Job) (n int, err error)
	Release(id JobID) (err error)
}

type DefaultHandler struct {
	algs []Algorithm
}

func NewDefaultHandler() (h *DefaultHandler) {
	h = &DefaultHandler{
		algs: DEFAULT_ALGORITHMS,
	}
	return h
}

func (h *DefaultHandler) Request(job *Job) (n int, err error) {
	if !contains(h.algs, job.params.a) {
		return 0, ErrUnsupported
	}

	if job.params.JobType == COMPRESS {
		switch job.params.a {
		case GZIP:
			gw, err := gzip.NewWriterLevel(job.w, job.params.level)
			if err != nil {
				return 0, ErrUnsupported
			}
			defer gw.Close()

			n, err = gw.Write(job.p)
			if err != nil {
				return n, err
			}
			if err := gw.Close(); err != nil {
				return n, err
			}

		case LZ4:
			lz4w := lz4.NewWriter(job.w)
			if err := lz4w.Apply(lz4.CompressionLevelOption(lz4.CompressionLevel(job.params.level))); err != nil {
				return 0, ErrUnsupported
			}
			defer lz4w.Close()

			n, err = lz4w.Write(job.p)
			if err != nil {
				return n, err
			}
			if err := lz4w.Close(); err != nil {
				return n, err
			}

		case ZSTD:
			zstdw, err := zstd.NewWriter(job.w, zstd.WithEncoderLevel(zstd.EncoderLevel(job.params.level)))
			if err != nil {
				return 0, ErrUnsupported
			}
			defer zstdw.Close()

			n, err = zstdw.Write(job.p)
			if err != nil {
				return n, err
			}
			if err := zstdw.Close(); err != nil {
				return n, err
			}

		default:
			return 0, fmt.Errorf("unsupported compression algorithm")
		}
	}

	if job.params.JobType == DECOMPRESS {
		switch job.params.a {
		case GZIP:
			gr, err := gzip.NewReader(job.r)
			if err != nil {
				return 0, err
			}
			defer gr.Close()

			n, err = gr.Read(job.p)
			if err != nil {
				return n, err
			}
			if err := gr.Close(); err != nil {
				return n, err
			}

		case LZ4:
			lz4r := lz4.NewReader(job.r)
			// defer lz4r.Close()

			n, err = lz4r.Read(job.p)
			if err != nil {
				return n, err
			}
			// if err := lz4r.Close(); err != nil {
			// 	return n, err
			// }

		case ZSTD:
			zstdr, err := zstd.NewReader(job.r)
			if err != nil {
				return 0, err
			}
			defer zstdr.Close()

			n, err = zstdr.Read(job.p)
			if err != nil {
				return n, err
			}
			// if err := zstdr.Close(); err != nil {
			// 	return n, err
			// }

		default:
			return 0, fmt.Errorf("unsupported decompression algorithm")
		}
	}

	return n, nil
}

func (h *DefaultHandler) Release(id JobID) (err error) {
	return nil
}

type IAAHandler struct {
	jobs     map[JobID]*ixl.BufWriter
	readjobs map[JobID]*ixl.Inflate
	jobsLock sync.Mutex
	algs     []Algorithm
}

func NewIAAHandler() (h *IAAHandler) {
	h = &IAAHandler{
		jobs:     make(map[JobID]*ixl.BufWriter),
		readjobs: make(map[JobID]*ixl.Inflate),
		jobsLock: sync.Mutex{},
		algs:     IAA_ALGORITHMS,
	}
	return h
}

func (h *IAAHandler) Request(job *Job) (n int, err error) {
	if !ixl.Ready() {
		return 0, ErrNotInstalled
	}
	if !contains(h.algs, job.params.a) {
		return 0, ErrUnsupported
	}
	h.jobsLock.Lock()
	if len(h.jobs) >= MAX_IAA_BINDINGS {
		h.jobsLock.Unlock()
		return 0, ErrNotAvailable
	}

	if job.params.JobType == COMPRESS {
		var nw *ixl.BufWriter
		if job.params.a == DEFLATE {
			if nw, err = ixl.NewDeflateWriter(job.w); err != nil {
				return 0, err
			}
		} else if job.params.a == GZIP {
			nw = ixl.NewGzipWriter(job.w)
		} else {
			return 0, errors.New("does not support algorithm")
		}
		h.jobs[job.id] = nw
		h.jobsLock.Unlock()
		n, err = nw.Write(job.p)
	}

	if job.params.JobType == DECOMPRESS {
		var iar *ixl.Inflate
		if job.params.a == DEFLATE || job.params.a == GZIP {
			if iar, err = ixl.NewInflate(job.r); err != nil {
				return 0, err
			}
		} else {
			return 0, errors.New("does not support algorithm")
		}
		h.readjobs[job.id] = iar
		h.jobsLock.Unlock()
		n, err = iar.Read(job.p)
	}
	return n, err
}

func (h *IAAHandler) Release(id JobID) (err error) {
	w, exists := h.jobs[id]
	if !exists {
		return errors.New("could not find the job")
	}
	err = w.Close()
	h.jobsLock.Lock() // This may be an unnecessary lock since ids are unique
	defer h.jobsLock.Unlock()
	delete(h.jobs, id)
	return err
}

type ISALHandler struct {
	jobs     map[JobID]*isal.Writer
	readjobs map[JobID]*isal.Reader
	algs     []Algorithm
}

func NewISALHandler() (h *ISALHandler) {
	h = &ISALHandler{
		jobs:     make(map[JobID]*isal.Writer),
		readjobs: make(map[JobID]*isal.Reader),
		algs:     ISAL_ALGORITHMS,
	}
	return h
}

func (h *ISALHandler) Request(job *Job) (n int, err error) {
	// TODO Check for algorithm
	if !isal.Ready() {
		return 0, ErrNotInstalled
	}
	if !contains(h.algs, job.params.a) {
		return 0, ErrUnsupported
	}

	var isar *isal.Reader
	var isaw *isal.Writer

	switch job.params.JobType {
	case COMPRESS:
		if val, ok := h.jobs[job.id]; ok {
			isaw = val
		} else {
			isaw, err = isal.NewWriterLevel(job.w, job.params.level)
			if err != nil {
				return 0, ErrUnsupported
			}
		}
		h.jobs[job.id] = isaw
		n, err = isaw.Write(job.p)
		if err != nil {
			return n, err
		}
	case DECOMPRESS:
		if val, ok := h.readjobs[job.id]; ok {
			isar = val
		} else {
			isar, err = isal.NewReader(job.r)
			if err != nil {
				return 0, err
			}
		}
		h.readjobs[job.id] = isar
		n, err = isar.Read(job.p)
		if err != nil {
			return n, err
		}
	}

	return n, err
}

func (h *ISALHandler) Release(id JobID) (err error) {
	w, writematch := h.jobs[id]
	r, readmatch := h.readjobs[id]

	if !writematch && !readmatch {
		return errors.New("could not find the job")
	}

	if writematch {
		err = w.Close()
		delete(h.jobs, id)
	}

	if readmatch {
		err = r.Close()
		delete(h.readjobs, id)
	}

	return err
}

type QatHandler struct {
	jobs     map[JobID]*QATJob
	jobsLock sync.Mutex
	algs     []Algorithm
}

type QATJob struct {
	outBuf []byte
	b      *qatzip.QzBinding
	w      *qatzip.Writer
	r      *qatzip.Reader
	job    *Job
	mode   QatMode
}

func NewQATHandler() (h *QatHandler) {
	h = &QatHandler{
		jobsLock: sync.Mutex{},
		jobs:     make(map[JobID]*QATJob),
		algs:     QAT_ALGORITHMS,
	}
	return h
}

func (h *QatHandler) ready() bool {
	// TODO: Implement ready func in qatgo util package
	return true
}

func (h *QatHandler) Request(job *Job) (n int, err error) {
	if !h.ready() {
		return 0, ErrNotInstalled
	}
	if !contains(h.algs, job.params.a) {
		return 0, ErrUnsupported
	}
	var qat *QATJob
	sym, _ := job.params.a.GetQATSymbol()

	if val, ok := h.jobs[job.id]; ok {
		qat = val
	} else {
		qat, err = h.newQatJob(job, DIRECT)
		if job.params.JobType == COMPRESS {
			if err := qat.w.Apply(
				qatzip.AlgorithmOption(qatzip.Algorithm(sym)),
				qatzip.CompressionLevelOption(job.params.level),
				qatzip.OutputBufLengthOption(2_580_000)); err != nil {
				return 0, ErrUnsupported
			}
			if job.params.a == GZIP {
				if err := qat.w.Apply(qatzip.DeflateFmtOption(qatzip.DeflateGzip)); err != nil {
					return 0, err
				}
			}
			if err != nil {
				return 0, err
			}
		}
		if job.params.JobType == DECOMPRESS {
			if err := qat.r.Apply(qatzip.AlgorithmOption(qatzip.Algorithm(sym))); err != nil {
				return 0, ErrUnsupported
			}
			if job.params.a == GZIP {
				if err := qat.r.Apply(qatzip.DeflateFmtOption(qatzip.DeflateGzip)); err != nil {
					return 0, err
				}
			}
		}

	}

	if job.params.JobType == COMPRESS {
		n, err = qat.w.Write(job.p)
		if err != nil {
			return 0, err
		}
		if err != nil {
			return 0, err
		}
	}

	if job.params.JobType == DECOMPRESS {
		n, err = qat.r.Read(job.p)

		if err != nil && err != io.EOF {
			return 0, err
		}
		// err = qat.w.Close()
		// if err != nil {
		// 	return 0, err
		// }
	}
	return n, err
}

// Functional but unused for now, the direct mode will be the default path
func (h *QatHandler) RequestStream(job *Job) (n int, err error) {
	if !h.ready() {
		return 0, ErrNotInstalled
	}

	qat, err := h.newQatJob(job, STREAM)
	if err != nil {
		return 0, err
	}
	sym, _ := job.params.a.GetQATSymbol()
	if err := qat.b.Apply(qatzip.AlgorithmOption(qatzip.Algorithm(sym)), qatzip.CompressionLevelOption(job.params.level)); err != nil {
		return 0, ErrUnsupported
	}
	if job.params.a == GZIP {
		if err := qat.b.Apply(qatzip.DeflateFmtOption(qatzip.DeflateGzip)); err != nil {
			return 0, err
		}
	}
	if err := qat.b.StartSession(); err != nil {
		return 0, err
	}

	np := 0
	nc := 0
	qat.b.SetLast(true)
	for {
		c, p, err := qat.b.Compress(qat.job.p[nc:], qat.outBuf[np:])
		if err == qatzip.ErrBuffer {
			qat.outBuf = append(qat.outBuf, make([]byte, len(qat.outBuf))...)
			continue
		}
		if err != nil {
			return 0, err
		}
		nc += c
		np += p
		if nc == len(job.p) {
			break
		}
	}
	n, err = job.w.Write(qat.outBuf[:np])
	return n, err
}

func (h *QatHandler) Release(id JobID) (err error) {
	job, err := h.findJob(id)
	if err != nil {
		return err
	}
	if job.mode == DIRECT {
		if job.w != nil {
			err = job.w.Close()
		} else if job.r != nil {
			err = job.r.Close()
		}
	} else {
		err = job.b.Close()
	}

	if err != nil {
		return err
	}
	h.jobsLock.Lock() // This may be an unnecessary lock since ids are unique
	defer h.jobsLock.Unlock()
	delete(h.jobs, id)
	return nil
}

func (h *QatHandler) newQatJob(job *Job, mode QatMode) (qat *QATJob, err error) {
	h.jobsLock.Lock()
	defer h.jobsLock.Unlock()
	if len(h.jobs) >= MAX_QAT_BINDINGS {
		return nil, ErrNotAvailable
	}

	var q *qatzip.QzBinding
	var w *qatzip.Writer
	var r *qatzip.Reader
	if mode == DIRECT {
		if job.w != nil {
			w = qatzip.NewWriter(job.w)
		} else if job.r != nil {
			r, err = qatzip.NewReader(job.r)
			if err != nil {
				return nil, err
			}
		}
	} else if mode == STREAM {
		q, err = qatzip.NewQzBinding()
		if err != nil {
			return nil, err
		}
	}

	qat = &QATJob{
		outBuf: make([]byte, DEFAULT_OUT_BUFFER_SZ),
		b:      q,
		job:    job,
		w:      w,
		r:      r,
		mode:   mode,
	}
	h.jobs[job.id] = qat
	return qat, nil
}

func (h *QatHandler) findJob(id JobID) (*QATJob, error) {
	for _, qat := range h.jobs {
		if qat.job.id == id {
			return qat, nil
		}
	}
	return nil, ErrJobNotFound
}

func contains(slice []Algorithm, algorithm Algorithm) bool {
	for _, a := range slice {
		if a == algorithm {
			return true
		}
	}
	return false
}
