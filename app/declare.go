package main

import (
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