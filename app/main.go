package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var _ = fmt.Fprint
var builtin_commands = []string{"exit", "echo", "type", "pwd", "cd", "history", "jobs"}
var history_cmds []string = readHistoryFromFile(os.Getenv("HISTFILE"))

func searchFileWithPerms(dir string, command string, perms os.FileMode) (bool) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if file.Name() == command {
			fileInfo, _ := file.Info()
			mode := fileInfo.Mode()
			if mode&perms != 0 {
				return true
			}
		}
	}
	return false
}

func searchCommandInPath(command string) (string, bool){
	path_var := os.Getenv("PATH")
	path_dirs := filepath.SplitList(path_var)

	for _, dir := range path_dirs {
		if ok := searchFileWithPerms(dir, command, os.FileMode(0111)); ok {
			full_path := filepath.Join(dir, command)
			return full_path, true
		}
	}
	return "", false
}

func dirPartsToPath(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}


func splitIntoArgs(arg_str string) []string {
	var args []string
	var current_arg strings.Builder
	in_single_quotes := false
	in_double_quotes := false
	for i := 0; i < len(arg_str); i++ {
		if arg_str[i] == ' '{
			if in_single_quotes || in_double_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if current_arg.Len() > 0 {
					args = append(args, current_arg.String())
					current_arg.Reset()
				}
			}
		} else if arg_str[i] == '"' {
			if in_single_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if i+1 < len(arg_str) && arg_str[i+1] == '"' {
					i++ // ignore adjacent quotes
				} else {
					in_double_quotes = !in_double_quotes
				}
			}
		} else if arg_str[i] == '\'' {
			if in_double_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if i+1 < len(arg_str) && arg_str[i+1] == '\'' {
					i++ // ignore adjacent quotes
				} else {
					in_single_quotes = !in_single_quotes
				}
			}
		} else if arg_str[i] == '\\' {
			if !in_single_quotes && !in_double_quotes {
				if i+1 < len(arg_str) {
					current_arg.WriteByte(arg_str[i+1])
					i++
				}
			} else if in_double_quotes {
				if i+1 < len(arg_str) && (arg_str[i+1] == '"' || arg_str[i+1] == '\\') {
					current_arg.WriteByte(arg_str[i+1])
					i++
				} else {
					current_arg.WriteByte(arg_str[i]) // No escaping
				}
			} else if in_single_quotes {
				current_arg.WriteByte(arg_str[i]) // No escaping
			}
		} else {
			current_arg.WriteByte(arg_str[i])
		}
	}
	if current_arg.Len() > 0 {
		args = append(args, current_arg.String())
	}
	return args
}

func posRedirect(args []string) (int, int, bool) {
	for i, arg := range args {
		if arg == ">" {
			return i, len(args), false
		}
		if arg == "1>" {
			return i, len(args), false
		}
		if arg == "2>" {
			return len(args), i, false
		}
		if arg == ">>" {
			return i, len(args), true
		}
		if arg == "1>>" {
			return i, len(args), true
		}
		if arg == "2>>" {
			return len(args), i, true
		}
	}
	return len(args), len(args), false
}

func searchExecutableForCompletion(dir string, prefix string) (string, bool) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	
	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) {
			fileInfo, _ := file.Info()
			mode := fileInfo.Mode()
			if mode&os.FileMode(0111) != 0 {
				return file.Name(), true
			}
		}
	}
	return "", false
}

func removeDuplicatesAndSort(s []string) []string {
	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueStrings []string
	for _, str := range s {
		if !seen[str] {
			seen[str] = true
			uniqueStrings = append(uniqueStrings, str)
		}
	}

	// Sort the unique strings
	sort.Strings(uniqueStrings)

	return uniqueStrings
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, str := range strs[1:] {
		for strings.Index(str, prefix) != 0 {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

func fileCompletions(prefix string) []string {
	dir := "."
	filePrefix := prefix
	if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
		dir = prefix[:idx]
		if dir == "" {
			dir = "/"
		}
		filePrefix = prefix[idx+1:]
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, filePrefix) {
			completed := name
			if entry.IsDir() {
				completed += "/"
			}
			if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
				completed = prefix[:idx+1] + completed
			}
			matches = append(matches, completed)
		}
	}
	return matches
}

func tryTabCompletion(input string) (string, bool, int) {
	// If input contains a space, complete the last argument as a filename
	if strings.Contains(input, " ") {
		lastSpace := strings.LastIndex(input, " ")
		prefix := input[lastSpace+1:]
		fileMatches := fileCompletions(prefix)
		fileMatches = removeDuplicatesAndSort(fileMatches)
		if len(fileMatches) == 0 {
			return input, false, 0
		}
		lcp := longestCommonPrefix(fileMatches)
		base := input[:lastSpace+1]
		if len(fileMatches) > 1 && lcp != "" && lcp != prefix {
			return base + lcp, true, 1
		}
		if len(fileMatches) == 1 {
			if fileMatches[0][len(fileMatches[0])-1] == '/' {
				return base + fileMatches[0], true, 1
			} else {
				return base + fileMatches[0] + " ", true, 1
			}
		} else if len(fileMatches) > 1 {
			return strings.Join(fileMatches, "  ") + " ", true, len(fileMatches)
		}
	}

	trimmed := strings.TrimSpace(input)
	matches := []string{}
	for _, cmd := range builtin_commands {
		if strings.HasPrefix(cmd, trimmed) && len(trimmed) > 0 && len(trimmed) < len(cmd) {
			matches = append(matches, cmd)
		}
	}

	path_var := os.Getenv("PATH")
	path_dirs := filepath.SplitList(path_var)
	for _, dir := range path_dirs {
		if cmd, ok := searchExecutableForCompletion(dir, trimmed); ok {
			matches = append(matches, cmd)
		}
	}
	matches = removeDuplicatesAndSort(matches)
	lcp := longestCommonPrefix(matches)
	if len(matches) > 1 && lcp != "" && lcp != trimmed {
		return lcp, true, 1
	}
	if len(matches) > 0 {
		return strings.Join(matches, "  ") + " ", true, len(matches)
	}
	return input, false, 0
}

func readLineWithTabCompletion() (string, error) {
	oldState, err := makeRaw(int(os.Stdin.Fd()))
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		return strings.TrimSpace(line), err
	}
	defer restore(int(os.Stdin.Fd()), oldState)
	
	var input strings.Builder
	buf := make([]byte, 1)
	var matches string
	historyPos := len(history_cmds)
	inHistoryNav := false
	pendingInput := ""

	redraw := func(newInput string) {
		fmt.Print("\r\x1b[2K$ ")
		fmt.Print(newInput)
		input.Reset()
		input.WriteString(newInput)
	}
	
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", err
		}
		
		ch := buf[0]
		
		switch ch {
		case 27: // Escape sequence (e.g. arrow keys)
			escSeq := make([]byte, 2)
			if _, err := io.ReadFull(os.Stdin, escSeq); err != nil {
				continue
			}
			if escSeq[0] != '[' {
				continue
			}

			switch escSeq[1] {
			case 'A': // Up arrow
				if len(history_cmds) == 0 {
					fmt.Print("\x07")
					continue
				}
				if !inHistoryNav {
					pendingInput = input.String()
					historyPos = len(history_cmds)
					inHistoryNav = true
				}
				if historyPos > 0 {
					historyPos--
					redraw(history_cmds[historyPos])
				} else {
					fmt.Print("\x07")
				}
			case 'B': // Down arrow
				if !inHistoryNav {
					fmt.Print("\x07")
					continue
				}
				if historyPos < len(history_cmds)-1 {
					historyPos++
					redraw(history_cmds[historyPos])
				} else {
					inHistoryNav = false
					historyPos = len(history_cmds)
					redraw(pendingInput)
				}
			}
		case '\t': // Tab key
			inHistoryNav = false
			historyPos = len(history_cmds)
			pendingInput = ""
			currentInput := input.String()
			if matches != "" {
				// print the matches on new line
				fmt.Println()
				fmt.Println(matches)
				// reprint the prompt and current input
				fmt.Print("$ " + currentInput)
			}
			if completed, ok, m := tryTabCompletion(currentInput); ok {
				if m > 1 {
					matches = completed
					fmt.Print("\x07")
				} else{
					matches = ""
					for i := 0; i < input.Len(); i++ {
						fmt.Print("\b \b")
					}
					fmt.Print(completed)
					input.Reset()
					input.WriteString(completed)
				}
			} else {
				fmt.Print("\x07")
			}
		case '\r', '\n': // Enter key
			matches = ""
			fmt.Println()
			return input.String(), nil
		case 127, 8: // Backspace
			inHistoryNav = false
			historyPos = len(history_cmds)
			pendingInput = ""
			matches = ""
			if input.Len() > 0 {
				fmt.Print("\b \b")
				str := input.String()
				input.Reset()
				input.WriteString(str[:len(str)-1])
			}
		case 3: // Ctrl+C
			fmt.Println()
			return "", fmt.Errorf("interrupted")
		default:
			inHistoryNav = false
			historyPos = len(history_cmds)
			pendingInput = ""
			matches = ""
			if ch >= 32 && ch <= 126 { // Printable characters
				fmt.Print(string(ch))
				input.WriteByte(ch)
			}
		}
	}
}

func splitByPipe(args []string) [][]string {
	var segments [][]string
	var current []string
	for _, arg := range args {
		if arg == "|" {
			if len(current) > 0 {
				segments = append(segments, current)
				current = nil
			}
		} else {
			current = append(current, arg)
		}
	}
	if len(current) > 0 {
		segments = append(segments, current)
	}
	return segments
}

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

func openRedirectFile(path string, append bool) *os.File {
	if append {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		return f
	}
	f, _ := os.Create(path)
	return f
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

func runInBackground(cmdArgs []string, current_dir []string) int {
	if slices.Contains(builtin_commands, cmdArgs[0]) {
        go runBuiltin(cmdArgs, os.Stdin, os.Stdout, os.Stderr, current_dir)
        return os.Getpid()
    }
    cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
    cmd.Start()
    return cmd.Process.Pid
}

func main() {
	dir, _ := os.Getwd()
	current_dir := strings.Split(dir, "/")[1:]
	bg_job_num := 0

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
			bg_job_num++
			bg_pid := runInBackground(args[:len(args)-1], current_dir)
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
				if args[1] == "-r" {
					history_file_cmds := readHistoryFromFile(args[2])
					for _, cmd := range history_file_cmds {
						history_cmds = append(history_cmds, cmd)
					}
				} else if args[1] == "-w" {
					writeHistoryToFile(args[2], history_cmds)
				} else if args[1] == "-a" {
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
			stdout += ""
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
