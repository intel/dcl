package glidepack

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var (
	errNoWorkingStrategies = errors.New("all strategies failed")
)

type Manager struct {
	strategies   []StrategyType
	GlobalPolicy PolicyFunc
	qat          *QatHandler
	isal         *ISALHandler
	iaa          *IAAHandler
	fallback     *DefaultHandler
	jobs         map[JobID]*Job
}

type Direction int
type JobID int64

const (
	COMPRESS Direction = iota
	DECOMPRESS
)

var instance *Manager
var once sync.Once

func initManager() {
	instance = &Manager{
		strategies:   GetStrategies(),
		GlobalPolicy: GetDefaultPolicy(),
		qat:          NewQATHandler(),
		isal:         NewISALHandler(),
		iaa:          NewIAAHandler(),
		fallback:     NewDefaultHandler(),
		jobs:         make(map[JobID]*Job),
	}
}

func GetManager() *Manager {
	once.Do(initManager)
	return instance
}

func (m *Manager) SubmitJob(p []byte, jp JobParams) (n int, id JobID, err error) {
	return m.SubmitWithPolicy(p, jp, m.GlobalPolicy)
}

type Job struct {
	id      JobID
	p       []byte
	JobType Direction
	params  JobParams
	w       io.Writer
	r       io.Reader
	h       Handler
	// dir Direction TODO Add direction, e.g. compress or decompress
}

type JobParams struct {
	a       Algorithm
	level   int
	id      JobID
	JobType Direction
	w       io.Writer
	r       io.Reader
}

var (
	nextID   int64 // Counter for the next job ID
	uniqueID int64
	mu       sync.Mutex
	freeIDs  []int64 // Slice to store available IDs for recycling
)

func createJob() *Job {
	var id int64

	mu.Lock()
	if len(freeIDs) > 0 {
		// Reuse an ID from the pool
		// id, freeIDs = freeIDs[len(freeIDs)-1], freeIDs[:len(freeIDs)-1]
		uniqueID = 1
		id = atomic.AddInt64(&nextID, 1)
	} else {
		// No free IDs, generate a new one
		uniqueID = 1
		id = atomic.AddInt64(&nextID, uniqueID)
	}
	mu.Unlock()

	return &Job{id: (JobID(id))}
}

func freeJobID(id JobID) {
	mu.Lock()
	freeIDs = append(freeIDs, int64(id))
	mu.Unlock()
}

func (m *Manager) getHandler(s StrategyType) Handler {
	switch s {
	case QAT:
		return m.qat
	case IAA:
		return m.iaa
	case ISAL:
		return m.isal
	default:
		return m.fallback
	}
}

func (m *Manager) SubmitWithPolicy(p []byte, jp JobParams, policy PolicyFunc) (n int, id JobID, err error) {
	if _, present := m.jobs[jp.id]; present && jp.JobType == DECOMPRESS {
		currentJob := m.jobs[jp.id]
		currentJob.p = p
		n, err = currentJob.h.Request(currentJob)
		if err == io.EOF {
			currentJob.h.Release(currentJob.id)
			delete(m.jobs, currentJob.id)
			freeJobID(currentJob.id)
		}
		return n, currentJob.id, err
	}

	params := &PolicyParameters{
		BufferSize: len(p),
		Strategies: m.strategies,
		JobParams:  jp,
	}

	job := createJob()
	job.p = p
	job.params = jp
	job.w = jp.w
	job.r = jp.r
	priority := policy(params)
	//TODO Filter by algorithm, installed (default by having all of them installed, then remove when proved otherwise)
	for _, strategy := range priority {

		if !strategy.IsValid() {
			return 0, job.id, errors.New("invalid strategy given by the policy")
		}
		h := m.getHandler(strategy)
		n, err := h.Request(job)
		if err == ErrNotAvailable || err == ErrUnsupported {
			continue
		} else if err == ErrNotInstalled {
			// TODO Remove from the global strategy options
			continue
		} else if err != nil && err != io.EOF {
			return 0, job.id, err
		}
		job.h = h
		m.jobs[job.id] = job

		if job.params.JobType == DECOMPRESS && err == io.EOF || job.params.JobType == COMPRESS && err == nil {
			h.Release(job.id)
			delete(m.jobs, job.id) //delete job from manager list in addition to handler list
			freeJobID(job.id)
		}
		return n, job.id, err
	}
	return 0, job.id, errNoWorkingStrategies
}
