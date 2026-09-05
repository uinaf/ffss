package main

import (
	"fmt"
	"os"
	"strings"
)

func verificationProcessZombie(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	// The parenthesized command name may itself contain spaces and ')'.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return false
	}
	fields := strings.Fields(string(data)[end+1:])
	return len(fields) > 0 && fields[0] == "Z"
}
