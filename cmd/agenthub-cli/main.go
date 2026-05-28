package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("agenthub-cli dev")
		return
	}
	fmt.Fprintln(os.Stderr, "agenthub-cli requires application wiring; use internal/interfaces/cli.CodeCommand from a configured binary")
	os.Exit(2)
}
