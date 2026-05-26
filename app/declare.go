package main

var variables = make(map[string]string)

func declareVariable(name string, value string) {
	variables[name] = value
}

func getVariable(name string) (string, bool) {
	value, ok := variables[name]
	return value, ok
}