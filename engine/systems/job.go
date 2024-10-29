package systems

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type JobSystem struct {
	numWorkers int
	jobQueue   chan metadata.JobTask
	wg         sync.WaitGroup
}

var ErrNoWorkers = fmt.Errorf("attempting to create worker pool with less than 1 worker")
var ErrNegativeChannelSize = fmt.Errorf("attempting to create worker pool with a negative channel size")

func NewJobSystem(numWorkers int, channelSize int) (*JobSystem, error) {
	if numWorkers <= 0 {
		return nil, ErrNoWorkers
	}
	if channelSize < 0 {
		return nil, ErrNegativeChannelSize
	}

	jq := make(chan metadata.JobTask, channelSize)
	js := &JobSystem{
		numWorkers: numWorkers,
		jobQueue:   jq,
	}

	js.start()

	return js, nil
}

func (js *JobSystem) start() {
	for i := 0; i < js.numWorkers; i++ {
		js.wg.Add(1)
		go func() {
			defer js.wg.Done()
			for job := range js.jobQueue {
				paramsChan := make(chan interface{}, 1)
				// Run the job and handle potential errors
				err := job.OnStart(job.InputParams, paramsChan)
				if err != nil {
					core.LogError(err.Error())
					if job.OnFailure != nil {
						// TODO: refactor to take actual values
						job.OnFailure(paramsChan)
					}
				} else {
					if job.OnComplete != nil {
						// TODO: refactor to take actual values
						job.OnComplete(paramsChan)
					}
				}

				// Call the completion callback if set
				if job.OnCompletionCallback != nil {
					job.OnCompletionCallback()
				}
			}
		}()
	}
}

/**
 * @brief Shuts the job system down.
 */
func (js *JobSystem) Shutdown() error {
	close(js.jobQueue)
	js.wg.Wait()
	return nil
}

/**
 * @brief Updates the job system. Should happen once an update cycle.
 */
func (js *JobSystem) Update() {}

// AddWorkNonBlocking adds work to the SimplePool and returns immediately
func (js *JobSystem) AddWorkNonBlocking(jt metadata.JobTask) {
	go js.Submit(jt)
}

/**
 * @brief Submits the provided job to be queued for execution.
 * @param info The description of the job to be executed.
 */
func (js *JobSystem) Submit(jt metadata.JobTask) {
	js.jobQueue <- jt
}
