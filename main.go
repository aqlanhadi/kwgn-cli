package main

import (
	"fmt"
	"os"

	"github.com/aqlanhadi/kwgn/cmd"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("kwgn version %s\n", version)
		os.Exit(0)
	}

	cmd.Execute()
}
