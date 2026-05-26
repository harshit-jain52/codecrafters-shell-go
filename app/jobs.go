package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
)

type Job struct {
	Number int
	Pid     int
	Command string
	IsRunning bool
}

var (
	jobs    []Job
	jobsMux sync.Mutex
)

func runInBackground(cmdArgs []string, current_dir []string, job_num int) int {

	// Builtin commands
	if slices.Contains(builtin_commands, cmdArgs[0]) {
		pid := os.Getpid()

		jobsMux.Lock()
		jobs = append(jobs, Job{
			Number: job_num,
			Pid:       pid,
			Command:   strings.Join(cmdArgs, " "),
			IsRunning: true,
		})
		jobsMux.Unlock()

		go func() {
			runBuiltin(cmdArgs, os.Stdin, os.Stdout, os.Stderr, current_dir)
			setStatusDone(job_num)
		}()

		return pid
	}

	// External commands
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	jobsMux.Lock()
	jobs = append(jobs, Job{
		Number: job_num,
		Pid:       cmd.Process.Pid,
		Command:   strings.Join(cmdArgs, " "),
		IsRunning: true,
	})
	jobsMux.Unlock()

	go func() {
		cmd.Wait()
		setStatusDone(job_num)
	}()

	return cmd.Process.Pid
}

func formatJobOutput(job_idx int) string {
	jobsMux.Lock()
	job := jobs[job_idx]
	jobsMux.Unlock()
	
	num := job.Number
	cmd := job.Command
	
	status := "Running"
	if !job.IsRunning {
		status = "Done"
		removeJob(num)
	}

	marker := " "
	switch num {
		case bg_job_num:
			marker = "+"
		case bg_job_num - 1:
			marker = "-"
	}
	return fmt.Sprintf("[%d]%s  %s                 %s\n", num, marker, status, cmd)
}

func setStatusDone(job_num int) {
	jobsMux.Lock()
	defer jobsMux.Unlock()

	for job_idx, job := range jobs {
		if job.Number == job_num {
			jobs[job_idx].IsRunning = false
			return
		}
	}
}

func removeJob(job_num int) {
	jobsMux.Lock()
	defer jobsMux.Unlock()

	for job_idx, job := range jobs {
		if job.Number == job_num {
			jobs = append(jobs[:job_idx], jobs[job_idx+1:]...)
			return
		}
	}
}