package systems

import (
	"sync"

	"github.com/spaghettifunk/anima/engine/containers"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources/loaders"
)

type jobSystemState struct {
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

var onceJobSystem sync.Once
var jsState *jobSystemState

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
func NewJobSystem(max_job_thread_count uint8, type_masks []uint32) error {
	onceJobSystem.Do(func() {
		jsState = &jobSystemState{}
	})
	return nil
}

/**
 * @brief Shuts the job system down.
 */
func JobSystemShutdown() {

}

/**
 * @brief Updates the job system. Should happen once an update cycle.
 */
func JobSystemUpdate() {}

/**
 * @brief Submits the provided job to be queued for execution.
 * @param info The description of the job to be executed.
 */
func JobSystemSubmit(info *metadata.JobInfo) {}

/**
 * @brief Creates a new job with default type (Generic) and priority (Normal).
 * @param entryPoint A pointer to a function to be invoked when the job starts. Required.
 * @param onSuccess A pointer to a function to be invoked when the job completes successfully. Optional.
 * @param onFail A pointer to a function to be invoked when the job fails. Optional.
 * @param paramData Data to be passed to the entry point upon execution.
 * @returns The newly created job information to be submitted for execution.
 */
func JobSystemJobCreate(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}) (*metadata.JobInfo, error) {
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
func JobSystemJobCreateType(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}, jobType metadata.JobType) (*metadata.JobInfo, error) {
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
func JobSystemJobCreatePriority(entryPoint metadata.JobStart, onSuccess, onFail metadata.JobOnComplete, paramData interface{}, jobType metadata.JobType, priority metadata.JobPriority) (*metadata.JobInfo, error) {
	return nil, nil
}

func (js *jobSystemState) store_result(callback metadata.JobOnComplete, param_size uint32, params interface{}) {
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

func (js *jobSystemState) job_thread_run(params interface{}) uint32 {
	// index := params.(uint32)
	// thread := js.JobThreads[index];
	// thread_id := thread.Thread.ThreadID;

	// core.logDebug("Starting job thread #%d (id=%#x, type=%#x).", thread.Index, thread_id, thread.TypeMask);

	// // A mutex to lock info for this thread.
	// thread.InfoMutex.Lock()

	// // Run forever, waiting for jobs.
	// while (true) {
	//     if (!state_ptr || !js.running || !thread) {
	//         break;
	//     }

	//     // Lock and grab a copy of the info
	//     if (!kmutex_lock(&thread->info_mutex)) {
	//         KERROR("Failed to obtain lock on job thread mutex!");
	//     }
	//     job_info info = thread->info;
	//     if (!kmutex_unlock(&thread->info_mutex)) {
	//         KERROR("Failed to release lock on job thread mutex!");
	//     }

	//     if (info.entry_point) {
	//         b8 result = info.entry_point(info.param_data, info.result_data);

	//         // Store the result to be executed on the main thread later.
	//         // Note that store_result takes a copy of the result_data
	//         // so it does not have to be held onto by this thread any longer.
	//         if (result && info.on_success) {
	//             store_result(info.on_success, info.result_data_size, info.result_data);
	//         } else if (!result && info.on_fail) {
	//             store_result(info.on_fail, info.result_data_size, info.result_data);
	//         }

	//         // Clear the param data and result data.
	//         if (info.param_data) {
	//             kfree(info.param_data, info.param_data_size, MEMORY_TAG_JOB);
	//         }
	//         if (info.result_data) {
	//             kfree(info.result_data, info.result_data_size, MEMORY_TAG_JOB);
	//         }

	//         // Lock and reset the thread's info object
	//         if (!kmutex_lock(&thread->info_mutex)) {
	//             KERROR("Failed to obtain lock on job thread mutex!");
	//         }
	//         kzero_memory(&thread->info, sizeof(job_info));
	//         if (!kmutex_unlock(&thread->info_mutex)) {
	//             KERROR("Failed to release lock on job thread mutex!");
	//         }
	//     }

	//     if (js.running) {
	//         // TODO: Should probably find a better way to do this, such as sleeping until
	//         // a request comes through for a new job.
	//         kthread_sleep(&thread->thread, 10);
	//     } else {
	//         break;
	//     }
	// }

	// // Destroy the mutex for this thread.
	// kmutex_destroy(&thread->info_mutex);
	return 1
}
