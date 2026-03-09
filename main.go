package main

import (
	"os"

	"github.com/chapsuk/wait-jobs/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
