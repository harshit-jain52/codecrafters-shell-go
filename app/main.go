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

func main() {
	dir, _ := os.Getwd()
	current_dir := strings.Split(dir, "/")[1:]
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
		} else if command_split[0] == "pwd"{
			fmt.Println(dirPartsToPath(current_dir))
		} else if command_split[0] == "cd"{
			tmp_current_dir := make([]string, len(current_dir))
			copy(tmp_current_dir, current_dir)
			dir_path := command_split[1]
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
						fmt.Printf("cd: %s: No such file or directory\n", tmp_path)
						valid_path = false
						break
					} else if !fileInfo.IsDir(){
						fmt.Printf("cd: %s: Not a directory\n", tmp_path)
						valid_path = false
						break
					}
				}
			}

			if valid_path {
				current_dir = tmp_current_dir
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
