package main

import (
	"io"

	contractschema "github.com/uinaf/autoreview/schema"
)

func runSchema(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h" || arguments[0] == "help") {
		report(stdout, "usage: autoreview schema <review|result>\n\nreview print the canonical provider-review JSON Schema\nresult print the canonical CLI-result JSON Schema\n")
		return 0
	}
	if len(arguments) != 1 {
		report(stderr, "usage: autoreview schema <review|result>\n")
		return 2
	}
	var document []byte
	switch arguments[0] {
	case "review":
		document = contractschema.ReviewV1()
	case "result":
		document = contractschema.ResultV1()
	default:
		report(stderr, "unknown schema %q; expected review or result\n", arguments[0])
		return 2
	}
	if _, err := stdout.Write(document); err != nil {
		report(stderr, "write %s schema: %v\n", arguments[0], err)
		return 2
	}
	return 0
}
