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
	ToReap bool
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
			ToReap: false,
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
		ToReap: false,
	})
	jobsMux.Unlock()

	go func() {
		cmd.Wait()
		setStatusDone(job_num)
	}()

	return cmd.Process.Pid
}

func formatJobOutput(job_idx int) string {
	job := jobs[job_idx]
	num := job.Number
	cmd := job.Command
	
	status := "Running"
	if !job.IsRunning {
		status = "Done"
		jobs[job_idx].ToReap = true	
	}

	marker := " "
	switch job_idx {
		case len(jobs) - 1:
			marker = "+"
		case len(jobs) - 2:
			marker = "-"
	}
	return fmt.Sprintf("[%d]%s  %s                 %s\n", num, marker, status, cmd)
}

func jobsOutput() string {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	
	var output string
	for job_idx := range jobs {
		output += formatJobOutput(job_idx)
	}
	return output
}

func reapOutput() string {
	jobsMux.Lock()
	defer jobsMux.Unlock()

	var output string
	for job_idx, job := range jobs {
		if !job.IsRunning {
			output += formatJobOutput(job_idx)
		}
	}
	return output
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

func removeCompletedJobs() {
	jobsMux.Lock()
	defer jobsMux.Unlock()

	for job_idx, job := range jobs {
		if job.ToReap {
			jobs = append(jobs[:job_idx], jobs[job_idx+1:]...)
		}
	}
}