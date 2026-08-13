package main

import (
	"os"

	"github.com/MikelCalvo/go-metin2-server/internal/migratecli"
)

func main() {
	os.Exit(migratecli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
