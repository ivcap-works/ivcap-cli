package main

import (
	"fmt"
	"github.com/reinventingscience/ivcap-client/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Execute(fmt.Sprintf("%s|%s|%s", version, commit[:7], date))
}
