package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	controller "github.com/rancher/cluster-controller/controller"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
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
		runControllers(c.String("config"))
		return nil
	}
	app.Run(os.Args)
}

func runControllers(config string) {
	logrus.Info("Staring cluster manager")
	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	logrus.Info("Creating controller config")
	controllerConfig, err := controller.NewControllerConfig(config)
	if err != nil {
		logrus.Fatalf("Failed to create controller config: [%v]", err)
	}
	logrus.Info("Created controller config")
	wg.Go(func() error { return controllerConfig.Run(ctx.Done()) })

	logrus.Info("Staring controllers")
	for name := range controller.GetControllers() {
		logrus.Infof("Starting [%s] controller", name)
		c := controller.GetControllers()[name]
		c.Init(controllerConfig)
		wg.Go(func() error { return c.Run(ctx.Done()) })
	}

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	select {
	case <-term:
		logrus.Infof("Received SIGTERM, shutting down")
	case <-ctx.Done():
	}

	cancel()
	for name, c := range controller.GetControllers() {
		logrus.Infof("Shutting down [%s] controller", name)
		c.Shutdown()
	}

	if err := wg.Wait(); err != nil {
		logrus.Errorf("Unhandled error received, shutting down: [%v]", err)
		os.Exit(1)
	}
	os.Exit(0)
}
