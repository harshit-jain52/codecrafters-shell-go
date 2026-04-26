package main

import (
	"bufio"
	"os"
)

func readHistoryFromFile(file_path string) []string {
	file, err := os.Open(file_path)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		lines = append(lines, line)
	}

	return lines
}

func writeHistoryToFile(file_path string, history_cmds []string) {
	file, err := os.Create(file_path)
	if err != nil {
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, cmd := range history_cmds {
		writer.WriteString(cmd + "\n")
	}
	writer.Flush()
}