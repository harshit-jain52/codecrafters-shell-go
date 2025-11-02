package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var _ = fmt.Fprint
var builtin_commands = []string{"exit", "echo", "type"}

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

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		command = strings.TrimSpace(command)
		command_split := strings.Split(command, " ")
		if command_split[0] == "exit" {
			exit_code, _ := strconv.Atoi(command_split[1])
			os.Exit(exit_code)
		} else if command_split[0] == "echo" {
			echoed_string := strings.Join(command_split[1:], " ")
			fmt.Println(echoed_string)
		} else if command_split[0] == "type" {
			command_string := strings.Join(command_split[1:], " ")
			builtin_found := false
			for _, cmd := range builtin_commands {
				if cmd == command_string {
					fmt.Printf("%s is a shell builtin\n", command_string)
					builtin_found = true
					break
				}
			}
			if !builtin_found {
				if full_path, ok := searchCommandInPath(command_string); ok {
					fmt.Printf("%s is %s\n", command_string, full_path)
				} else {
					fmt.Printf("%s: not found\n", command_string)
				}
			}
		} else if _ , ok := searchCommandInPath(command_split[0]); ok{
			args := command_split[1:]
			cmd := exec.Command(command_split[0], args...)
			output, _ := cmd.Output()
			fmt.Printf("%s", output)
		} else {
			fmt.Printf("%s: command not found\n", command)
		}
	}
}
