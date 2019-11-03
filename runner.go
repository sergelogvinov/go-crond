package main

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron"
)

type Job struct {
	Id      int
	cronId  cron.EntryID
	Name    string
	Updated bool
	Status  error
	Elapsed time.Duration
}

type Runner struct {
	cron   *cron.Cron
	jobsMu sync.Mutex
	jobs   []Job
	nextId int
}

func NewRunner() *Runner {
	r := &Runner{
		jobsMu: sync.Mutex{},
	}
	return r
}

// Recreate crontab jobs
func (r *Runner) CreateCronjobs(crontabEntries []CrontabEntry) error {
	r.cron = cron.New()
	r.jobs = []Job{}

	for _, crontabEntry := range crontabEntries {
		if opts.EnableUserSwitching {
			r.AddWithUser(crontabEntry)
		} else {
			r.Add(crontabEntry)
		}
	}

	return nil
}

// Add crontab entry
func (r *Runner) Add(cronjob CrontabEntry) error {
	cronSpec := cronjob.Spec
	r.jobsMu.Lock()
	defer r.jobsMu.Unlock()

	id, err := r.cron.AddFunc(cronSpec, r.cmdFunc(r.nextId, cronjob, func(execCmd *exec.Cmd) bool {
		// before exec callback
		LoggerInfo.CronjobExec(cronjob)
		return true
	}))

	if err != nil {
		LoggerError.Printf("Failed add cron job spec:%v cmd:%v err:%v", cronjob.Spec, cronjob.Command, err)
	} else {
		LoggerInfo.CronjobAdd(cronjob)

		r.jobs = append(r.jobs, Job{Id: r.nextId, cronId: id, Name: cronjob.Name})
		r.nextId++
	}

	return err
}

// Add crontab entry with user
func (r *Runner) AddWithUser(cronjob CrontabEntry) error {
	cronSpec := cronjob.Spec
	r.jobsMu.Lock()
	defer r.jobsMu.Unlock()

	id, err := r.cron.AddFunc(cronSpec, r.cmdFunc(r.nextId, cronjob, func(execCmd *exec.Cmd) bool {
		// before exec callback
		LoggerInfo.CronjobExec(cronjob)

		// lookup username
		u, err := user.Lookup(cronjob.User)
		if err != nil {
			LoggerError.Printf("user lookup failed: %v", err)
			return false
		}

		// convert userid to int
		userId, err := strconv.ParseUint(u.Uid, 10, 32)
		if err != nil {
			LoggerError.Printf("Cannot convert user to id:%v", err)
			return false
		}

		// convert groupid to int
		groupId, err := strconv.ParseUint(u.Gid, 10, 32)
		if err != nil {
			LoggerError.Printf("Cannot convert group to id:%v", err)
			return false
		}

		// add process credentials
		execCmd.SysProcAttr = &syscall.SysProcAttr{}
		execCmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(userId), Gid: uint32(groupId)}
		return true
	}))

	if err != nil {
		LoggerError.Printf("Failed add cron job %v; Error:%v", LoggerError.CronjobToString(cronjob), err)
	} else {
		LoggerInfo.Printf("Add cron job %v", LoggerError.CronjobToString(cronjob))

		r.jobs = append(r.jobs, Job{Id: r.nextId, cronId: id, Name: cronjob.Name})
		r.nextId++
	}

	return err
}

// Return number of jobs
func (r *Runner) Len() int {
	return len(r.cron.Entries())
}

// Start runner
func (r *Runner) Start() {
	LoggerInfo.Printf("Start runner with %d jobs\n", r.Len())
	r.cron.Start()
}

// Stop runner
func (r *Runner) Stop() {
	r.cron.Stop()
	LoggerInfo.Println("Stop runner")
}

func (r *Runner) GetJobs() []Job {
	r.jobsMu.Lock()
	defer r.jobsMu.Unlock()

	var entries = make([]Job, len(r.jobs))

	for i, e := range r.jobs {
		entries[i] = e
	}
	return entries
}

// Execute crontab command
func (r *Runner) cmdFunc(id int, cronjob CrontabEntry, cmdCallback func(*exec.Cmd) bool) func() {
	cmdFunc := func() {
		// fall back to normal shell if not specified
		taskShell := cronjob.Shell
		if taskShell == "" {
			taskShell = DEFAULT_SHELL
		}

		start := time.Now()

		// Init command
		execCmd := exec.Command(taskShell, "-c", cronjob.Command)
		execCmd.Dir = cronjob.Pwd

		// add custom env to cronjob
		if len(cronjob.Env) >= 1 {
			execCmd.Env = append(os.Environ(), cronjob.Env...)
		}

		// exec custom callback
		if cmdCallback(execCmd) {

			// exec job
			out, err := execCmd.CombinedOutput()

			elapsed := time.Since(start)

			for i, entry := range r.jobs {
				if entry.Id == id {
					r.jobsMu.Lock()
					r.jobs[i].Status = err
					r.jobs[i].Elapsed = elapsed
					r.jobs[i].Updated = true
					r.jobsMu.Unlock()
				}
			}

			if err != nil {
				LoggerError.CronjobExecFailed(cronjob, string(out), err, elapsed)
			} else {
				LoggerInfo.CronjobExecSuccess(cronjob, string(out), err, elapsed)
			}
		}
	}
	return cmdFunc
}
