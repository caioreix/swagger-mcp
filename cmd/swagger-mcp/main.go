package main

import (
	"os"

	"github.com/caioreix/swagger-mcp/cmd/swagger-mcp/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
