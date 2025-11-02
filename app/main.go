package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Fprint

func searchCommand(dir string, command string) (bool) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if file.Name() == command {
			fileInfo, _ := file.Info()
			mode := fileInfo.Mode()
			if mode&os.FileMode(0111) != 0 {
				return true
			}
		}
	}
	return false
}

func main() {
	for {
		builtin_commands := []string{"exit", "echo", "type"}
		path_var := os.Getenv("PATH")
		path_dirs := filepath.SplitList(path_var)

		fmt.Fprint(os.Stdout, "$ ")
		command, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		command_split := strings.Split(command, " ")
		if command_split[0] == "exit" {
			exit_code, _ := strconv.Atoi(command_split[1])
			os.Exit(exit_code)
		} else if command_split[0] == "echo" {
			echoed_string := strings.Join(command_split[1:], " ")
			echoed_string = echoed_string[:len(echoed_string)-1]
			fmt.Println(echoed_string)
		} else if command_split[0] == "type" {
			command_string := strings.Join(command_split[1:], " ")
			command_string = command_string[:len(command_string)-1]
			builtin_found := false
			for _, cmd := range builtin_commands {
				if cmd == command_string {
					fmt.Printf("%s is a shell builtin\n", command_string)
					builtin_found = true
					break
				}
			}
			if !builtin_found {
				found := false
				for _, dir := range path_dirs {
					if ok := searchCommand(dir, command_string); ok {
						full_path := filepath.Join(dir, command_string)
						fmt.Printf("%s is %s\n", command_string, full_path)
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("%s: not found\n", command_string)
				}
			}
		} else {
			fmt.Printf("%s: command not found\n", command[:len(command)-1])
		}
	}
}
