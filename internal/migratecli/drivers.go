package migratecli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
)

const sqlDriversFormat = "go-metin2-sql-drivers-v1"

type sqlDrivers struct {
	Format  string   `json:"format"`
	Drivers []string `json:"drivers"`
}

func runDrivers(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("drivers", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var requireDriver string
	flags.StringVar(&requireDriver, "require-driver", "", "fail unless this database/sql driver is linked into the binary")
	flags.Usage = func() { printDriversUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected drivers argument %q\n", flags.Arg(0))
		printDriversUsage(stderr)
		return exitUsage
	}
	var requireDriverSet bool
	flags.Visit(func(flag *flag.Flag) {
		requireDriverSet = flag.Name == "require-driver"
	})
	if requireDriverSet && strings.TrimSpace(requireDriver) == "" {
		printDriversUsage(stderr)
		return exitUsage
	}
	if requireDriverSet {
		if err := config.RequireRegisteredDatabaseDriver(requireDriver); err != nil {
			fmt.Fprintf(stderr, "drivers: %v\n", err)
			return exitError
		}
	}
	return writeJSON(stdout, stderr, sqlDrivers{
		Format:  sqlDriversFormat,
		Drivers: config.RegisteredDatabaseDrivers(),
	})
}
