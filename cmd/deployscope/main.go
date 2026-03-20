package main

import (
	"github.com/ppiankov/deployscope/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
