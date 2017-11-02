package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	operator "github.com/rancher/cluster-controller/operator"
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
		runOperators(c.String("config"))
		return nil
	}
	app.Run(os.Args)
}

func runOperators(config string) {
	logrus.Info("Staring cluster manager")
	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	operatorConfig, err := operator.NewOperatorConfig(config)
	if err != nil {
		logrus.Fatalf("Failed to create operator config: [%v]", err)
	}
	logrus.Info("Staring operators")
	for name, operator := range operator.GetOperators() {
		logrus.Infof("Starting [%s] operator", name)
		operator.Init(operatorConfig)
		wg.Go(func() error { return operator.Run(ctx.Done()) })
	}

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	select {
	case <-term:
		logrus.Infof("Received SIGTERM, shutting down")
	case <-ctx.Done():
	}

	cancel()
	if err := wg.Wait(); err != nil {
		logrus.Errorf("Unhandled error received, shutting down: [%v]", err)
		os.Exit(1)
	}
	os.Exit(0)
}
