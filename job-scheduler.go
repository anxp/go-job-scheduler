package job_scheduler

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type JobScheduler struct {
	jobsAll             *jobsAll
	jobsInProgress      *jobsInProgress
	jobSchedulerRunning bool
}

type jobsAll struct {
	jobs map[string]job // All jobs, added to JobScheduler, currently running and not
	sync.RWMutex
}

type jobsInProgress struct {
	// jobsInProgress is a registry of jobs currently executing.
	// We WILL NOT attempt to run a job again, if it is still executing.
	// We store jobID only in this registry, no reason to store whole job entity.
	jobs map[string]struct{}
	sync.RWMutex
}

type job struct {
	jobID                      string
	callbackTypeFunc           func(...interface{}) bool // Experimental
	callbackTypeEmptyInterface interface{}               // Currently implemented
	arguments                  []interface{}
	schedule                   Schedule
	description                string
}

type JobExecutionReport struct {
	JobID          string
	Error          error // If Error == nil job completed successfully
	Timestamp      int64
	AdditionalInfo string
	Job            job
}

type Schedule struct {
	Minute     string
	Hour       string
	DayOfMonth string
	Month      string
	DayOfWeek  string // 0-6, Sunday == 0, Monday == 1, Tuesday == 2
}

// NewJobScheduler - constructor of JobScheduler object
func NewJobScheduler() *JobScheduler {
	jobsAll := &jobsAll{
		jobs: make(map[string]job),
	}

	jobsInProgress := &jobsInProgress{
		jobs: make(map[string]struct{}),
	}

	return &JobScheduler{
		jobsAll:             jobsAll,
		jobsInProgress:      jobsInProgress,
		jobSchedulerRunning: false,
	}
}

func (js *JobScheduler) RunJobScheduler() *chan JobExecutionReport {
	// Problem - we can Stop the ticker, but channel will remain opened.
	// If we want implement Stop Job Scheduler, we should figure out how to close the channel (or no?)
	// https://stackoverflow.com/questions/17797754/ticker-stop-behaviour-in-golang
	// https://github.com/golang/go/issues/2650

	if js.jobSchedulerRunning {
		panic("Job Scheduler instance is already running. No need to run the same set of tasks in parallel goroutine!")
	}

	js.jobSchedulerRunning = true

	reportPublishChannel := make(chan JobExecutionReport)

	// Function sends job execution report to parent goroutine, OR DROPS IT IF THERE IS NO LISTENER!
	sendExecutionReportToParentGoroutine := func(job job, additionalInfo string, err error) {
		select {
		case reportPublishChannel <- JobExecutionReport{
			JobID:          job.jobID,
			Error:          err,
			Timestamp:      time.Now().Unix(),
			AdditionalInfo: additionalInfo,
			Job:            job,
		}:
			// Message successfully sent
		default:
			// Message was not sent because channel is blocked. Usually it means we don't have a reader/listener
		}
	}

	go func() {
		ticker := time.NewTicker(time.Minute)

		for t := range ticker.C {
			js.jobsAll.RLock()

			for jobID, jobEntity := range js.jobsAll.jobs {

				if isTimeToRun, err := js.isTimeToRun(t.Minute(), t.Hour(), t.Day(), t.Month(), t.Weekday(), jobEntity.schedule); err != nil {
					sendExecutionReportToParentGoroutine(jobEntity, fmt.Sprintf("Schedule parsing error: %s", err), err)
					continue // We don't interrupt tasks execution process, just skip problematic task
				} else if !isTimeToRun {
					continue
				}

				if js.isJobRunning(jobID) {
					// Slow work does not necessarily mean that something is wrong with the task. But we have to notify the user about it.
					// TODO: Maybe add "Resource intensive task" flag to report?
					info := fmt.Sprintf("Job \"%s\" executing for more than one minute. This can mean: 1) the task is stuck, 2) the task takes more time to complete.", jobEntity.description)
					sendExecutionReportToParentGoroutine(jobEntity, info, nil)
				} else {
					go func(job job) {
						js.addJobToRunningRegistry(job.jobID)

						args := make([]reflect.Value, len(job.arguments))
						for i := 0; i < len(args); i++ {
							args[i] = reflect.ValueOf(job.arguments[i])
						}

						fv := reflect.ValueOf(job.callbackTypeEmptyInterface)
						result := fv.Call(args)

						var err error
						info := fmt.Sprintf("Job \"%s\" completed successfully.", job.description)

						// Magic explanation: https://stackoverflow.com/questions/17262238/how-to-cast-reflect-value-to-its-type
						if result[0].Interface() != nil {
							err = result[0].Interface().(error)
							info = fmt.Sprintf("Job \"%s\" WAS NOT completed successfully. Please recheck and debug callback describing the task.", job.description)
						}

						sendExecutionReportToParentGoroutine(job, info, err)

						js.removeJobFromRunningRegistry(job.jobID)
						return
					}(jobEntity)
				}
			}

			js.jobsAll.RUnlock()
		}
	}()

	return &reportPublishChannel
}

// CreateNewJob returns newly created Job ID and nil, or error if creating a job was unsuccessful
func (js *JobScheduler) CreateNewJob(schedule Schedule, description string, fn interface{}, args ...interface{}) (string, error) {

	errMessagePrefix := fmt.Sprintf("can't create \"%s\" job: ", description)

	if fn == nil || reflect.ValueOf(fn).Kind() != reflect.Func {
		return "", fmt.Errorf("%s job must be a function (not nil)", errMessagePrefix)
	}

	fnType := reflect.TypeOf(fn)
	if len(args) != fnType.NumIn() {
		return "", fmt.Errorf("%s number of expected arguments in callback and number of provided arguments doesn't match", errMessagePrefix)
	}

	// Magic explanation: https://stackoverflow.com/questions/30688514/go-reflect-how-to-check-whether-reflect-type-is-an-error-type
	if errorInterface := reflect.TypeOf((*error)(nil)).Elem(); fnType.NumOut() != 1 || !fnType.Out(0).Implements(errorInterface) {
		return "", fmt.Errorf("%s scheduled job function should return only one value, and this value should implement error interface", errMessagePrefix)
	}

	for i := 0; i < fnType.NumIn(); i++ {
		a := args[i]
		expectedType := fnType.In(i)
		providedType := reflect.TypeOf(a)

		if expectedType != providedType {
			if expectedType.Kind() != reflect.Interface {
				return "", fmt.Errorf("%s argument [%d] should be \"%s\", got \"%s\" instead", errMessagePrefix, i, expectedType, providedType)
			}
			if !providedType.Implements(expectedType) {
				return "", fmt.Errorf("%s argument [%d] of type \"%s\" doesn't implement interface \"%s\"", errMessagePrefix, i, providedType, expectedType)
			}
		}
	}

	js.jobsAll.Lock()

	jobID := ""
	for true {
		jobID = js.generateRandomString(5)
		if _, ok := js.jobsAll.jobs[jobID]; ok == false {
			break // Generated jobID is unique, that's what we want!
		}
	}

	js.jobsAll.jobs[jobID] = job{
		jobID:                      jobID,
		callbackTypeEmptyInterface: fn,
		arguments:                  args,
		schedule:                   schedule,
		description:                description,
	}

	js.jobsAll.Unlock()

	return jobID, nil
}

func (js *JobScheduler) isTimeToRun(currMin, currHour, currDayOfMonth int, currMonth time.Month, currDayOfWeek time.Weekday, schedule Schedule) (bool, error) {
	if schedule.Minute == "*" && schedule.Hour == "*" && schedule.DayOfMonth == "*" && schedule.Month == "*" && schedule.DayOfWeek == "*" {
		return true, nil
	}

	timeToRun := true

	if schedule.Minute != "*" {
		if minute, err := strconv.Atoi(schedule.Minute); err != nil {
			return false, err
		} else if minute != currMin {
			timeToRun = false
		}
	}

	if schedule.Hour != "*" {
		if hour, err := strconv.Atoi(schedule.Hour); err != nil {
			return false, err
		} else if hour != currHour {
			timeToRun = false
		}
	}

	if schedule.DayOfMonth != "*" {
		if dayOfMonth, err := strconv.Atoi(schedule.DayOfMonth); err != nil {
			return false, err
		} else if dayOfMonth != currDayOfMonth {
			timeToRun = false
		}
	}

	if schedule.Month != "*" {
		if month, err := strconv.Atoi(schedule.Month); err != nil {
			return false, err
		} else if month != int(currMonth) {
			timeToRun = false
		}
	}

	if schedule.DayOfWeek != "*" {
		if dayOfWeek, err := strconv.Atoi(schedule.DayOfWeek); err != nil {
			return false, err
		} else if dayOfWeek != int(currDayOfWeek) {
			timeToRun = false
		}
	}

	return timeToRun, nil
}

func (js *JobScheduler) generateRandomString(length int) string {
	rand.Seed(time.Now().Unix())
	randomStr := make([]byte, length)

	for i := 0; i < length; i++ {
		randomStr[i] = byte(65 + rand.Intn(26))
	}

	return string(randomStr)
}

func (js *JobScheduler) addJobToRunningRegistry(jobID string) {
	js.jobsInProgress.Lock()
	(*js).jobsInProgress.jobs[jobID] = struct{}{} // We don't need to store WHOLE JOB ENTITY, jobID is ENOUGH!
	js.jobsInProgress.Unlock()
}

func (js *JobScheduler) removeJobFromRunningRegistry(jobID string) {
	js.jobsInProgress.Lock()
	delete((*js).jobsInProgress.jobs, jobID)
	js.jobsInProgress.Unlock()
}

func (js *JobScheduler) isJobRunning(jobID string) bool {
	js.jobsInProgress.RLock()
	_, present := (*js).jobsInProgress.jobs[jobID]
	js.jobsInProgress.RUnlock()

	return present
}
