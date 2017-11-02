package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Kube config for accessing kubernetes cluster",
		},
	}

	app.Action = func(c *cli.Context) error {
		go handleSigterm()
		return nil
	}
	app.Run(os.Args)
}

func handleSigterm() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	logrus.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	logrus.Infof("Exiting with %v", exitCode)
	// TODO - graceful shutdown for all the operators
	os.Exit(exitCode)
}

func registerControllers() error {

	return nil
}
