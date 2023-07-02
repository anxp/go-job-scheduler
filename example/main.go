package main

import (
	"errors"
	"fmt"
	"github.com/anxp/job-scheduler"
)

func main() {

	scheduler := job_scheduler.NewJobScheduler()
	reportChannel := scheduler.RunJobScheduler() // We can start Job Scheduler immediately, even if job pool is empty at the moment
	additionalDataAForJob := "Some text message"
	additionalDataBForJob := 998

	//=== This is normal job which can be successfully completed: ======================================================
	_, err := scheduler.CreateNewJob(job_scheduler.Schedule{
		Minute:     "10",
		Hour:       "*",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}, "Normal job with variables", func(v1 string, v2 int) error {

		fmt.Printf("We are running job 1! We also have additional variables for this job: \nVariableA: %s\nVariableB: %d\n", v1, v2)
		return nil // We return error = nil if job is successfully done

	}, additionalDataAForJob, additionalDataBForJob)

	if err != nil {
		fmt.Print(err)
		return
	}
	//==================================================================================================================

	//=== This is example of job that cannot be successfully completed, it runs every minute and issues an error: ======
	_, err = scheduler.CreateNewJob(job_scheduler.Schedule{
		Minute:     "*",
		Hour:       "*",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}, "Failing job", func() error {

		fmt.Printf("We are running job 2!\n")
		return errors.New("can't make this every-minute job done")

	})

	if err != nil {
		fmt.Print(err)
		return
	}
	//==================================================================================================================

	//=== This is example of job that will never run because there are no 60th minute, use 0 instead! ==================
	_, err = scheduler.CreateNewJob(job_scheduler.Schedule{
		Minute:     "60",
		Hour:       "*",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}, "Never run job", func() error {

		fmt.Printf("We are running job 3!\n") // We should never see this
		return nil

	})

	if err != nil {
		fmt.Print(err)
		return
	}
	//==================================================================================================================

	//=== This is example of job that will run once an hour at 0 minute: ===============================================
	_, err = scheduler.CreateNewJob(job_scheduler.Schedule{
		Minute:     "0",
		Hour:       "*",
		DayOfMonth: "*",
		Month:      "*",
		DayOfWeek:  "*",
	}, "Hourly job that run every 0 minute of hour", func() error {

		fmt.Printf("We are running job 4!\n")
		return nil

	})

	if err != nil {
		fmt.Print(err)
		return
	}
	//==================================================================================================================

	for report := range *reportChannel {
		if report.Error != nil {
			fmt.Printf("Error on executing the job #%s: %s; additional info: %s\n", report.JobID, report.Error, report.AdditionalInfo)
		} else {
			fmt.Printf("Job #%s completed sucessfully; additional info: %s\n", report.JobID, report.AdditionalInfo)
		}
	}
}
