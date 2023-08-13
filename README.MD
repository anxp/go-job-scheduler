# Simple Task Manager on Go
This is simple task manager for Go applications. 
Task should be set up as a closure with an arbitrary number of arguments.

*Every task is executed in separate go routine, 
therefore, if the task is cumbersome and time-consuming, 
it will not affect the launch of other tasks in any way.*

## Example of usage:
```
scheduler := go_job_scheduler.NewJobScheduler()
reportChannel := scheduler.RunJobScheduler() // We can start Job Scheduler immediately, even if job pool is empty at the moment

additionalDataAForJob := "Some text message"
additionalDataBForJob := 998

schedule := go_job_scheduler.Schedule{
    Minute:     "10",
    Hour:       "*",
    DayOfMonth: "*",
    Month:      "*",
    DayOfWeek:  "*",
}

_, err := scheduler.CreateNewJob(schedule, "Normal job with variables", func(v1 string, v2 int) error {

    fmt.Printf("We are running job 1! We also have additional variables for this job: \nVariableA: %s\nVariableB: %d\n", v1, v2)
    return nil // We return error = nil if job is successfully done
    
    }, additionalDataAForJob, additionalDataBForJob)

if err != nil {
    fmt.Print(err)
    return
}

// Listening report channel:
for report := range *reportChannel {
    if report.Error != nil {
        fmt.Printf("Error on executing the job #%s: %s; additional info: %s\n", report.JobID, report.Error, report.AdditionalInfo)
    } else {
        fmt.Printf("Job #%s completed sucessfully; additional info: %s\n", report.JobID, report.AdditionalInfo)
    }
}
```

## How to set schedule:
In order to maintain maximum simplicity we don't support common Unix scheduler format (https://crontab.guru/).
Instead we use as simple as possible approach:

```
type Schedule struct {
    Minute     string // "*" or "0"-"59"
    Hour       string // "*" or "0"-"23"
    DayOfMonth string // "*" or "1"-"31"
    Month      string // "*" or "1"-"12"
    DayOfWeek  string // "*" or "0"-"6", Sunday == 0, Monday == 1, Tuesday == 2
}
```
