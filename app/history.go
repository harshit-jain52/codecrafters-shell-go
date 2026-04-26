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

func appendHistoryToFile(filePath string, historyCmds []string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	marker := "history -a " + filePath
	lastIdx := -1

	// Find last occurrence of the marker
	for i := len(historyCmds) - 2; i >= 0; i-- {
		if historyCmds[i] == marker {
			lastIdx = i
			break
		}
	}

	// Append everything after the marker
	start := lastIdx + 1
	for i := start; i < len(historyCmds); i++ {
		if _, err := file.WriteString(historyCmds[i] + "\n"); err != nil {
			return
		}
	}
}