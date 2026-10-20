package greet

func format(name string) string {
	return "Hello, " + name
}

func Greet(name string) string {
	return format(name)
}
