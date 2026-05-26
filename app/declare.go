package main

import (
	"strings"
	"unicode"
)

var variables = make(map[string]string)

func declareVariable(name string, value string) {
	variables[name] = value
}

func getVariable(name string) (string, bool) {
	value, ok := variables[name]
	return value, ok
}

func validateVariableName(name string) bool {
	if name == "" {
		return false
	}

	first := rune(name[0])
	if !(unicode.IsLetter(first) || first == '_') {
		return false
	}

	for _, ch := range name[1:] {
		if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_') {
			return false
		}
	}

	return true
}

func replaceVariables(args []string) {
	for i, arg := range args {
		if len(arg) > 1 && strings.Contains(arg, "$") {
			dollarIdx := strings.Index(arg, "$")
			varName := arg[dollarIdx+1:]
			if value, ok := getVariable(varName); ok {
				args[i] = arg[:dollarIdx] + value
			} else {
				args[i] = "" // Undefined variables are replaced with empty string
			}
		}
	}
}