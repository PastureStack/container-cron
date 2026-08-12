package main

import (
	"context"
	"os"
	"time"

	"github.com/PastureStack/container-cron/events"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// VERSION of the application
var (
	MetadataURL = "http://169.254.169.250/2016-07-29"
	VERSION     = "v0.0.0-dev"
)

func beforeApp(c *cli.Context) error {
	if c.GlobalBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "container-cron"
	app.Version = VERSION
	app.Usage = "Run scheduled actions against Docker containers"
	app.Action = start
	app.Before = beforeApp
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "debug,d",
		},
		cli.BoolFlag{
			Name:  "metadata-mode,m",
			Usage: "Enable service-state checks through the metadata API",
		},
		cli.StringFlag{
			Name:  "metadata-url",
			Value: MetadataURL,
			Usage: "Provide full URL of Metadata",
		},
		cli.BoolFlag{
			Name: "metrics",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func start(c *cli.Context) error {
	handler, err := events.NewDockerHandler(&events.DockerHandlerOpts{
		MetadataMode: c.GlobalBool("metadata-mode"),
		MetadataURL:  c.GlobalString("metadata-url"),
	})
	if err != nil {
		return err
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return err
	}

	if c.GlobalBool("metrics") {
		go MetricsServer(handler)
	}

	return events.StartRouter(context.Background(), router, handler, time.Second)
}
