package greet

func greeting(name string) string {
	return "Hello, " + name
}

func Greet(name string) string {
	return greeting(name)
}
