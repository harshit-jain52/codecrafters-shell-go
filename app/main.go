package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var _ = fmt.Fprint
var builtin_commands = []string{"exit", "echo", "type", "pwd", "cd", "history", "jobs", "complete"}
var history_cmds []string = readHistoryFromFile(os.Getenv("HISTFILE"))


// runBuiltin executes a builtin command, reading from stdin and writing to stdout/stderr.
// Returns the (possibly updated) current_dir.
func runBuiltin(cmdArgs []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, current_dir []string) []string {
	switch cmdArgs[0] {
	case "echo":
		fmt.Fprintln(stdout, strings.Join(cmdArgs[1:], " "))
	case "pwd":
		fmt.Fprintln(stdout, dirPartsToPath(current_dir))
	case "type":
		if len(cmdArgs) < 2 {
			return current_dir
		}
		name := cmdArgs[1]
		if slices.Contains(builtin_commands, name) {
			fmt.Fprintf(stdout, "%s is a shell builtin\n", name)
		} else if full_path, ok := searchCommandInPath(name); ok {
			fmt.Fprintf(stdout, "%s is %s\n", name, full_path)
		} else {
			fmt.Fprintf(stderr, "%s: not found\n", name)
		}
	case "cd":
		tmp := make([]string, len(current_dir))
		copy(tmp, current_dir)
		dir_path := cmdArgs[1]
		valid := true
		if dir_path[0] == '/' {
			dir_path = dir_path[1:]
			tmp = []string{}
		} else if dir_path == "~" {
			home_parts := strings.Split(os.Getenv("HOME"), "/")[1:]
			tmp = home_parts
			dir_path = dir_path[1:]
		}
		for _, part := range strings.Split(dir_path, "/") {
			if part == ".." {
				if len(tmp) > 0 {
					tmp = tmp[:len(tmp)-1]
				}
			} else if part != "." && part != "" {
				tmp = append(tmp, part)
				tmp_path := dirPartsToPath(tmp)
				fileInfo, err := os.Stat(tmp_path)
				if err != nil {
					fmt.Fprintf(stderr, "cd: %s: No such file or directory\n", tmp_path)
					valid = false
					break
				} else if !fileInfo.IsDir() {
					fmt.Fprintf(stderr, "cd: %s: Not a directory\n", tmp_path)
					valid = false
					break
				}
			}
		}
		if valid {
			current_dir = tmp
		}
	}
	return current_dir
}

func executePipeline(segments [][]string, current_dir []string) []string {
	n := len(segments)
	var pipeReaders []*os.File
	var pipeWriters []*os.File

	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pipe: %v\n", err)
			return current_dir
		}
		pipeReaders = append(pipeReaders, r)
		pipeWriters = append(pipeWriters, w)
	}

	var wg sync.WaitGroup

	for i, seg := range segments {
		args := seg
		stdout_redir, stderr_redir, is_append := posRedirect(args)
		pos_redirect := min(stdout_redir, stderr_redir)
		cmdArgs := args[:pos_redirect]

		var stdin *os.File
		if i == 0 {
			stdin = os.Stdin
		} else {
			stdin = pipeReaders[i-1]
		}

		var stdout *os.File
		var stdoutOpened bool
		if i == n-1 {
			if stdout_redir < len(args) {
				stdout = openRedirectFile(args[stdout_redir+1], is_append)
				stdoutOpened = true
			} else {
				stdout = os.Stdout
			}
		} else {
			stdout = pipeWriters[i]
		}

		var stderr *os.File
		var stderrOpened bool
		if i == n-1 && stderr_redir < len(args) {
			stderr = openRedirectFile(args[stderr_redir+1], is_append)
			stderrOpened = true
		} else {
			stderr = os.Stderr
		}

		isBuiltin := slices.Contains(builtin_commands, cmdArgs[0])

		if isBuiltin {
			wg.Add(1)
			go func(cArgs []string, in, out, errOut *os.File, outOpened, errOpened bool) {
				defer wg.Done()
				runBuiltin(cArgs, in, out, errOut, current_dir)
				if outOpened {
					out.Close()
				}
				if errOpened {
					errOut.Close()
				}
			}(cmdArgs, stdin, stdout, stderr, stdoutOpened, stderrOpened)
		} else {
			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			cmd.Start()
			wg.Add(1)
			go func(c *exec.Cmd, out *os.File, outOpened bool, errOut *os.File, errOpened bool) {
				defer wg.Done()
				c.Wait()
				if outOpened {
					out.Close()
				}
				if errOpened {
					errOut.Close()
				}
			}(cmd, stdout, stdoutOpened, stderr, stderrOpened)
		}
	}

	// Close write ends so readers get EOF
	for i := 0; i < len(pipeWriters); i++ {
		pipeWriters[i].Close()
	}

	wg.Wait()

	for i := 0; i < len(pipeReaders); i++ {
		pipeReaders[i].Close()
	}

	return current_dir
}


func main() {
	dir, _ := os.Getwd()
	current_dir := strings.Split(dir, "/")[1:]

	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, _ := readLineWithTabCompletion()
		history_cmds = append(history_cmds, command)
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		args := splitIntoArgs(command)

		hasBackground := slices.Contains(args, "&")
		hasPipe := slices.Contains(args, "|")

		if hasBackground {
			bg_job_num := getNextJobNum()
			bg_pid := runInBackground(args[:len(args)-1], current_dir, bg_job_num)
			fmt.Printf("[%d] %d\n", bg_job_num, bg_pid)
			continue
		}

		if hasPipe {
			segments := splitByPipe(args)
			current_dir = executePipeline(segments, current_dir)
			continue
		}

		stdout_redir, stderr_redir, is_append := posRedirect(args)
		pos_redirect := min(stdout_redir, stderr_redir)
		stdout := ""
		stderr := ""
		if args[0] == "exit" {
			writeHistoryToFile(os.Getenv("HISTFILE"), history_cmds)
			os.Exit(0)
		} else if args[0] == "echo" {
			echoed_string := strings.Join(args[1:pos_redirect], " ")
			stdout = echoed_string + "\n"
		} else if args[0] == "type" {
			command_string := args[1]
			builtin_found := false
			if slices.Contains(builtin_commands, command_string) {
					stdout = fmt.Sprintf("%s is a shell builtin\n", command_string)
					builtin_found = true
				}
			if !builtin_found {
				if full_path, ok := searchCommandInPath(command_string); ok {
					stdout = fmt.Sprintf("%s is %s\n", command_string, full_path)
				} else {
					stderr = fmt.Sprintf("%s: not found\n", command_string)
				}
			}
		} else if args[0] == "pwd"{
			stdout = dirPartsToPath(current_dir) + "\n"
		} else if args[0] == "cd"{
			tmp_current_dir := make([]string, len(current_dir))
			copy(tmp_current_dir, current_dir)
			dir_path := args[1]
			valid_path := true
			if dir_path[0] == '/' {
				dir_path = dir_path[1:]
				tmp_current_dir = []string{}
			} else if dir_path == "~" {
				home_dir := os.Getenv("HOME")
				home_parts := strings.Split(home_dir, "/")[1:]
				tmp_current_dir = home_parts
				dir_path = dir_path[1:]
			}
			dir_parts := strings.Split(dir_path, "/")
			for _, part := range dir_parts {
				if part == ".." {
					if len(tmp_current_dir) > 0 {
						tmp_current_dir = tmp_current_dir[:len(tmp_current_dir)-1]
					}
				} else if part != "." && part != "" {
					tmp_current_dir = append(tmp_current_dir, part)
					tmp_path := dirPartsToPath(tmp_current_dir)
					fileInfo, err := os.Stat(tmp_path)
					if err != nil {
						stderr = fmt.Sprintf("cd: %s: No such file or directory\n", tmp_path)
						valid_path = false
						break
					} else if !fileInfo.IsDir(){
						stderr = fmt.Sprintf("cd: %s: Not a directory\n", tmp_path)
						valid_path = false
						break
					}
				}
			}

			if valid_path {
				current_dir = tmp_current_dir
			}
		} else if args[0] == "history" {
			n := len(history_cmds)
			if len(args) > 2 {
				switch args[1] {
				case "-r":
					history_file_cmds := readHistoryFromFile(args[2])
					for _, cmd := range history_file_cmds {
						history_cmds = append(history_cmds, cmd)
					}
				case "-w":
					writeHistoryToFile(args[2], history_cmds)
				case "-a":
					appendHistoryToFile(args[2], history_cmds)
				}
			}
			if len(args) > 1 {
				n, _ = strconv.Atoi(args[1])
				n = min(n, len(history_cmds))
			}
			for i := len(history_cmds)-n; i < len(history_cmds); i++ {
				stdout += fmt.Sprintf("%d %s\n", i+1, history_cmds[i])
			}
		} else if args[0] == "jobs" {
			if len(jobs) == 0 {
				stdout += ""
			} else {
				stdout += jobsOutput()
			}
			removeCompletedJobs()
		} else if args[0] == "complete" {
			if len(args) >= 2{
				switch args[1] {
				case "-p":
					path, ok := getCompletionSpec(args[2])
					if ok {
						stdout += fmt.Sprintf("complete -C '%s' %s\n", path, args[2])
					} else {
						stderr += fmt.Sprintf("complete: %s: no completion specification\n", args[2])
					}
				case "-C":
					if len(args) >= 4 {
						registerCompletion(args[3], args[2])
					}
				}
			}
		} else if _ , ok := searchCommandInPath(args[0]); ok{
			cmd := exec.Command(args[0], args[1:pos_redirect]...)
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf

			cmd.Run()

			stdout = stdoutBuf.String()
			stderr = stderrBuf.String()
		} else {
			stderr = fmt.Sprintf("%s: command not found\n", args[0])
		}

		stdout += reapOutput()
		removeCompletedJobs()

		if stdout_redir < len(args) {
			filename := args[stdout_redir+1]
			if is_append {
				file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				file.WriteString(stdout)
				file.Close()
			} else {
				file, _ := os.Create(filename)
				file.WriteString(stdout)
				file.Close()
			}
		} else {
			fmt.Fprint(os.Stdout, stdout)
		}

		if stderr_redir < len(args) {
			filename := args[stderr_redir+1]
			if is_append {
				file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				file.WriteString(stderr)
				file.Close()
			} else {
				file, _ := os.Create(filename)
				file.WriteString(stderr)
				file.Close()
			}
		} else {
			fmt.Fprint(os.Stderr, stderr)
		}
	}
}
