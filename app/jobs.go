package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type Job struct {
	Pid     int
	Command string
	IsRunning bool
}

func runInBackground(cmdArgs []string, current_dir []string, job_num int) int {

	// Builtin commands
	if slices.Contains(builtin_commands, cmdArgs[0]) {
		pid := os.Getpid()

		jobs = append(jobs, Job{
			Pid:       pid,
			Command:   strings.Join(cmdArgs, " "),
			IsRunning: true,
		})

		go func() {
			runBuiltin(cmdArgs, os.Stdin, os.Stdout, os.Stderr, current_dir)
			jobs[job_num-1].IsRunning = false
		}()

		return pid
	}

	// External commands
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	jobs = append(jobs, Job{
		Pid:       cmd.Process.Pid,
		Command:   strings.Join(cmdArgs, " "),
		IsRunning: true,
	})

	go func() {
		cmd.Wait()
		jobs[job_num - 1].IsRunning = false
	}()

	return cmd.Process.Pid
}

func formatJobOutput(job_num int) string {
	job := jobs[job_num-1]
	status := "Running"
	if !job.IsRunning {
		status = "Done"
	}

	marker := " "
	if job_num == len(jobs) {
		marker = "+"
	} else if job_num == len(jobs)-1 {
		marker = "-"
	}
	return fmt.Sprintf("[%d]%s  %s                 %s\n", job_num, marker, status, job.Command)
}