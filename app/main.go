package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Fprint

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")
		// Wait for user input
		command, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		command_split := strings.Split(command, " ")
		if command_split[0] == "exit" {
			exit_code, _ := strconv.Atoi(command_split[1])
			os.Exit(exit_code)
		}
		fmt.Println(command[:len(command)-1] + ": command not found")
	}
}
