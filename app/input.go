package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func splitIntoArgs(arg_str string) []string {
	var args []string
	var current_arg strings.Builder
	in_single_quotes := false
	in_double_quotes := false
	for i := 0; i < len(arg_str); i++ {
		switch arg_str[i] {
		case ' ':
			if in_single_quotes || in_double_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if current_arg.Len() > 0 {
					args = append(args, current_arg.String())
					current_arg.Reset()
				}
			}
		case '"':
			if in_single_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if i+1 < len(arg_str) && arg_str[i+1] == '"' {
					i++ // ignore adjacent quotes
				} else {
					in_double_quotes = !in_double_quotes
				}
			}
		case '\'':
			if in_double_quotes {
				current_arg.WriteByte(arg_str[i])
			} else {
				if i+1 < len(arg_str) && arg_str[i+1] == '\'' {
					i++ // ignore adjacent quotes
				} else {
					in_single_quotes = !in_single_quotes
				}
			}
		case '\\':
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
		default:
			current_arg.WriteByte(arg_str[i])
		}
	}
	if current_arg.Len() > 0 {
		args = append(args, current_arg.String())
	}
	replaceVariables(args)
	return cleanUpEmptyArgs(args)
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
	args := splitIntoArgs(input)
	path, found := getCompletionSpec(args[0])
	if found {
		os.Setenv("COMP_LINE", input)
		os.Setenv("COMP_POINT", strconv.Itoa(len(input)))
		argv1 := args[0]
		argv2 := ""
		if !strings.HasSuffix(input, " ") {
			argv2 = args[len(args)-1]
		}
		argv3 := ""
		if len(args) >= 2 {
			argv3 = args[len(args)-2]
		}
		cmd := exec.Command(path, argv1, argv2, argv3)
		output, _ := cmd.Output()
		result := strings.TrimSpace(string(output))
		matches := strings.Split(result, "\n")
		lcp := longestCommonPrefix(matches)
		if len(matches) > 1 && lcp != "" && lcp != argv2 {
			return input[:len(input)-len(argv2)] + lcp, true, 1
		}
		if len(matches) > 1 {
			sort.Strings(matches)
			return strings.Join(matches, "  ") + " ", true, len(matches)
		} 
		if result == "" {
			return input, false, 0
		}
		if argv2 != "" && strings.HasPrefix(result, argv2) {
			result = result[len(argv2):]
		}
		return input + result + " ", true, 1
	}

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