// Command gallant is the CLI entry point for gallant.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gallant:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fmt.Println("Hello from gallant")
	return nil
}
