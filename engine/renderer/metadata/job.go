package metadata

/** @brief Describes a type of job */
type JobType int

const (
	/**
	 * @brief A general job that does not have any specific thread requirements.
	 * This means it matters little which job thread this job runs on.
	 */
	JOB_TYPE_GENERAL JobType = 0x02
	/**
	 * @brief A resource loading job. Resources should always load on the same thread
	 * to avoid potential disk thrashing.
	 */
	JOB_TYPE_RESOURCE_LOAD JobType = 0x04
	/**
	 * @brief Jobs using GPU resources should be bound to a thread using this job type. Multithreaded
	 * renderers will use a specific job thread, and this type of job will run on that thread.
	 * For single-threaded renderers, this will be on the main thread.
	 */
	JOB_TYPE_GPU_RESOURCE JobType = 0x08
)

/**
 * @brief Determines which job queue a job uses. The high-priority queue is always
 * exhausted first before processing the normal-priority queue, which must also
 * be exhausted before processing the low-priority queue.
 */
type JobPriority int

const (
	/** @brief The lowest-priority job, used for things that can wait to be done if need be, such as log flushing. */
	JOB_PRIORITY_LOW JobPriority = iota
	/** @brief A normal-priority job. Should be used for medium-priority tasks such as loading assets. */
	JOB_PRIORITY_NORMAL
	/** @brief The highest-priority job. Should be used sparingly, and only for time-critical operations.*/
	JOB_PRIORITY_HIGH
)

/**
 * @brief Describes a job to be run.
 */
type JobTask struct {
	/** @brief The type of job. Used to determine which thread the job executes on. */
	JobType JobType
	/** @brief The priority of this job. Higher priority jobs obviously run sooner. */
	InputParams          interface{}
	Priority             JobPriority
	OnStart              func(params interface{}, output chan<- interface{}) error // Called when job starts
	OnComplete           func(paramsChan <-chan interface{})                       // Called when job completes successfully
	OnFailure            func(paramsChan <-chan interface{})                       // Called when job fails
	OnCompletionCallback func()                                                    // Optional callback after job completion
}

// The max number of job results that can be stored at once.
const MAX_JOB_RESULTS int = 512
