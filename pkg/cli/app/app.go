package app

import (
	"fmt"

	"github.com/macula-io/macula-os/pkg/cli/config"
	"github.com/macula-io/macula-os/pkg/cli/diag"
	"github.com/macula-io/macula-os/pkg/cli/encrypt"
	"github.com/macula-io/macula-os/pkg/cli/install"
	"github.com/macula-io/macula-os/pkg/cli/rc"
	"github.com/macula-io/macula-os/pkg/cli/reset"
	"github.com/macula-io/macula-os/pkg/cli/upgrade"
	"github.com/macula-io/macula-os/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	Debug bool
)

// New CLI App
func New() *cli.App {
	app := cli.NewApp()
	app.Name = "maculaos"
	app.Usage = "Lightweight Linux for Macula edge nodes"
	app.Version = version.Version
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
	}
	// required flags without defaults will break symlinking to exe with name of sub-command as target
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			EnvVar:      "MACULAOS_DEBUG",
			Destination: &Debug,
		},
	}

	app.Commands = []cli.Command{
		rc.Command(),
		config.Command(),
		install.Command(),
		upgrade.Command(),
		diag.Command(),
		reset.Command(),
		encrypt.Command(),
	}

	app.Before = func(c *cli.Context) error {
		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	return app
}
