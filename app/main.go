package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var _ = fmt.Fprint
var builtin_commands = []string{"exit", "echo", "type", "pwd", "cd"}

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

func tryTabCompletion(input string) (string, bool, int) {
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
	
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", err
		}
		
		ch := buf[0]
		
		switch ch {
		case '\t': // Tab key
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
			matches = ""
			if ch >= 32 && ch <= 126 { // Printable characters
				fmt.Print(string(ch))
				input.WriteByte(ch)
			}
		}
	}
}

func main() {
	dir, _ := os.Getwd()
	current_dir := strings.Split(dir, "/")[1:]
	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, _ := readLineWithTabCompletion()
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		args := splitIntoArgs(command)
		stdout_redir, stderr_redir, is_append := posRedirect(args)
		pos_redirect := min(stdout_redir, stderr_redir)
		stdout := ""
		stderr := ""
		if args[0] == "exit" {
			os.Exit(0)
		} else if args[0] == "echo" {
			echoed_string := strings.Join(args[1:pos_redirect], " ")
			stdout = echoed_string + "\n"
		} else if args[0] == "type" {
			command_string := args[1]
			builtin_found := false
			for _, cmd := range builtin_commands {
				if cmd == command_string {
					stdout = fmt.Sprintf("%s is a shell builtin\n", command_string)
					builtin_found = true
					break
				}
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
