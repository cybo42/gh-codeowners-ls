package main

import (
	"os"

	"github.com/cybo42/gh-codeowners-ls/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
