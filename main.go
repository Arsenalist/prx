package main

import (
	"os"

	"github.com/Arsenalist/prx/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
