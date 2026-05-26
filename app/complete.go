package main

var completions	= make(map[string]string)

func registerCompletion(name string, spec string) {
	completions[name] = spec
}

func getCompletionSpec(name string) (string, bool) {
	spec, ok := completions[name]
	return spec, ok
}