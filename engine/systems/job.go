package systems

import (
	"sync"

	"github.com/spaghettifunk/anima/engine/containers"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems/loaders"
)

type JobSystem struct {
	Running     bool
	ThreadCount uint8
	JobThreads  chan<- metadata.JobThread

	LowPriorityQueue    *containers.RingQueue
	NormalPriorityQueue *containers.RingQueue
	HighPriorityQueue   *containers.RingQueue

	// Mutexes for each queue, since a job could be kicked off from another job (thread).
	LowPriQueueMutex    sync.Mutex
	NormalPriQueueMutex sync.Mutex
	HighPriQueueMutex   sync.Mutex

	PendingResults [metadata.MAX_JOB_RESULTS]*metadata.JobResultEntry
	ResultMutex    sync.Mutex
	// A mutex for the result array
}

/**
 * @brief Initializes the job system. Call once to retrieve job_system_memory_requirement, passing 0 to state. Then
 * call a second time with allocated state memory block.
 * @param job_system_memory_requirement A pointer to hold the memory required for the job system state in bytes.
 * @param state A block of memory to hold the state of the job system.
 * @param max_job_thread_count The maximum number of job threads to be spun up.
 * Should be no more than the number of cores on the CPU, minus one to account for the main thread.
 * @param type_masks A collection of type masks for each job thread. Must match max_job_thread_count.
 * @returns True if the job system started up successfully; otherwise false.
 */
func NewJobSystem(maxJobThreadCount uint8, typeMasks []uint32) (*JobSystem, error) {
	js := &JobSystem{}
	return js, nil
}

/**
 * @brief Shuts the job system down.
 */
func (js *JobSystem) Shutdown() error {
	return nil
}

/**
 * @brief Updates the job system. Should happen once an update cycle.
 */
func (js *JobSystem) Update() {}

/**
 * @brief Submits the provided job to be queued for execution.
 * @param info The description of the job to be executed.
 */
func (js *JobSystem) Submit(info *metadata.JobInfo) {}

/**
 * @brief Creates a new job with default type (Generic) and priority (Normal).
 * @param entryPoint A pointer to a function to be invoked when the job starts. Required.
 * @param onSuccess A pointer to a function to be invoked when the job completes successfully. Optional.
 * @param onFail A pointer to a function to be invoked when the job fails. Optional.
 * @param paramData Data to be passed to the entry point upon execution.
 * @returns The newly created job information to be submitted for execution.
 */
func (js *JobSystem) JobCreate(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}) (*metadata.JobInfo, error) {
	return nil, nil
}

/**
 * @brief Creates a new job with default priority (Normal).
 * @param entryPoint A pointer to a function to be invoked when the job starts. Required.
 * @param onSuccess A pointer to a function to be invoked when the job completes successfully. Optional.
 * @param onFail A pointer to a function to be invoked when the job fails. Optional.
 * @param paramData Data to be passed to the entry point upon execution.
 * @param type The type of job. Used to determine which thread the job executes on.
 * @returns The newly created job information to be submitted for execution.
 */
func (js *JobSystem) JobCreateType(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}, jobType metadata.JobType) (*metadata.JobInfo, error) {
	return nil, nil
}

/**
 * @brief Creates a new job with the provided priority.
 * @param entryPoint A pointer to a function to be invoked when the job starts. Required.
 * @param onSuccess A pointer to a function to be invoked when the job completes successfully. Optional.
 * @param onFail A pointer to a function to be invoked when the job fails. Optional.
 * @param paramData Data to be passed to the entry point upon execution.
 * @param type The type of job. Used to determine which thread the job executes on.
 * @param priority The priority of this job. Higher priority jobs obviously run sooner.
 * @returns The newly created job information to be submitted for execution.
 */
func (js *JobSystem) JobCreatePriority(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}, jobType metadata.JobType, priority metadata.JobPriority) (*metadata.JobInfo, error) {
	return nil, nil
}

func (js *JobSystem) StoreResult(callback metadata.JobOnComplete, param_size uint32, params interface{}) {
	// Create the new entry.
	entry := &metadata.JobResultEntry{
		ID:        loaders.InvalidIDUint16,
		ParamSize: param_size,
		Callback:  callback,
	}
	if entry.ParamSize > 0 {
		// Take a copy, as the job is destroyed after this.
		entry.Params = nil
	} else {
		entry.Params = 0
	}

	// Lock, find a free space, store, unlock.
	js.ResultMutex.Lock()
	for i := uint16(0); i < uint16(metadata.MAX_JOB_RESULTS); i++ {
		if js.PendingResults[i].ID == loaders.InvalidIDUint16 {
			js.PendingResults[i] = entry
			js.PendingResults[i].ID = i
			break
		}
	}
	js.ResultMutex.Unlock()
}

func (js *JobSystem) JobThreadRun(params interface{}) uint32 {
	return 1
}
