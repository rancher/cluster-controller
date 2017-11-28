package main

import (
	"os"

	"github.com/rancher/cluster-controller/controller"
	"github.com/rancher/types/config"
	"github.com/urfave/cli"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Kube config for accessing kubernetes cluster",
			EnvVar: "KUBECONFIG",
		},
	}

	app.Action = func(c *cli.Context) error {
		return run(c.String("config"))
	}

	app.Run(os.Args)
}

func run(kubeConfigFile string) error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return err
	}

	cluster, err := config.NewClusterContext(*kubeConfig)
	if err != nil {
		return err
	}

	controller.Register(cluster)

	return cluster.StartAndWait()
}
