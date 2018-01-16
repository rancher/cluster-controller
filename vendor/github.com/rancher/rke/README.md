rke
========

Rancher Kubernetes Engine, an extremely simple, lightning fast Kubernetes installer that works everywhere.

## Download

Please check the [releases](https://github.com/rancher/rke/releases/) page.

## Requirements

- Docker versions 1.12.6, 1.13.1, or 17.03 should be installed for Kubernetes 1.8.
- OpenSSH 7.0+ must be installed on each node for stream local forwarding to work.
- The SSH user used for node access must be a member of the `docker` group:
```bash
usermod -aG docker <user_name>
```
- Ports 6443, 2379, and 2380 should be opened between cluster nodes.

## Getting Started

Standing up a Kubernetes is as simple as creating a `cluster.yml` configuration file and running the command:
```bash
./rke up --config cluster.yml
```

### Full `cluster.yml` example

You can view full sample of cluster.yml [here](https://github.com/rancher/rke/blob/master/cluster.yml).

### Minimal `cluster.yml` example
```
nodes:
  - address: 1.1.1.1
    user: ubuntu
    role: [controlplane,worker,etcd]

services:
  etcd:
    image: quay.io/coreos/etcd:latest
  kube-api:
    image: rancher/k8s:v1.8.3-rancher2
  kube-controller:
    image: rancher/k8s:v1.8.3-rancher2
  scheduler:
    image: rancher/k8s:v1.8.3-rancher2
  kubelet:
    image: rancher/k8s:v1.8.3-rancher2
  kubeproxy:
    image: rancher/k8s:v1.8.3-rancher2
```

## Network Plugins

RKE supports the following network plugins:

- Flannel
- Calico
- Cannal
- Weave

To use specific network plugin configure `cluster.yml` to include:
```
network:
  plugin: flannel
```

### Network Options

There are extra options that can be specified for each network plugin:

#### Flannel

- **flannel_image**: Flannel daemon Docker image
- **flannel_cni_image**: Flannel CNI binary installer Docker image
- **flannel_iface**: Interface to use for inter-host communication

#### Calico

- **calico_node_image**: Calico Daemon Docker image
- **calico_cni_image**: Calico CNI binary installer Docker image
- **calico_controllers_image**: Calico Controller Docker image
- **calicoctl_image**: Calicoctl tool Docker image
- **calico_cloud_provider**: Cloud provider where Calico will operate, currently supported values are: `aws`, `gce`

#### Cannal

- **canal_node_image**: Cannal Node Docker image
- **canal_cni_image**: Cannal CNI binary installer Docker image
- **canal_flannel_image**: Cannal Flannel Docker image

#### Weave

- **weave_node_image**: Weave Node Docker image
- **weave_cni_image**: Weave CNI binary installer Docker image

## Addons

RKE support pluggable addons on cluster bootstrap, user can specify the addon yaml in the cluster.yml file, and when running
```
rke up --config cluster.yml
```
RKE will deploy the addons yaml after the cluster starts, RKE first uploads this yaml file as a configmap in kubernetes cluster and then run a kubernetes job that mounts this config map and deploy the addons.

> Note that RKE doesn't support yet removal of the addons, so once they are deployed the first time you can't change them using rke

To start using addons use `addons:` option in the `cluster.yml` file for example:
```

addons: |-
    ---
    apiVersion: v1
    kind: Pod
    metadata:
      name: my-nginx
      namespace: default
    spec:
      containers:
      - name: my-nginx
        image: nginx
        ports:
        - containerPort: 80
```

Note that we are using `|-` because the addons option is a multi line string option, where you can specify multiple yaml files and separate them with `---`

## High Availability

RKE is HA ready, you can specify more than one controlplane host in the `cluster.yml` file, and rke will deploy master components on all of them, the kubelets are configured to connect to `127.0.0.1:6443` by default which is the address of `nginx-proxy` service that proxy requests to all master nodes.

to start an HA cluster, just specify more than one host with role `controlplane`, and start the cluster normally.

## Adding/Removing Nodes

RKE support adding/removing nodes for worker and controlplane hosts, in order to add additional nodes you will only need to update the `cluster.yml` file with additional nodes and run `rke up` with the same file.

To remove nodes just remove them from the hosts list in the cluster configuration file `cluster.yml`, and re run `rke up` command.

## Cluster Remove

RKE support `rke remove` command, the command does the following:

- Connect to each host and remove the kubernetes services deployed on it.
- Clean each host from the directories left by the services:
  - /etc/kubernetes/ssl
  - /var/lib/etcd
  - /etc/cni
  - /opt/cni
  - /var/run/calico

Note that this command is irreversible and will destroy the kubernetes cluster entirely.

## Cluster Upgrade

RKE support kubernetes cluster upgrade through changing the image version of services, in order to do that change the image option for each services, for example:
```
image: rancher/k8s:v1.8.2-rancher1
```
TO

```
image: rancher/k8s:v1.8.3-rancher2
```

And then run:

```
rke up --config cluster.yml
```

RKE will first look for the local `.kube_config_cluster.yml` and then tries to upgrade each service to the latest image.

> Note that rollback isn't supported in RKE and may lead to unxpected results

## RKE Config

RKE support command `rke config` which generates a cluster config template for the user, to start using this command just write:
```
rke config --name mycluster.yml
```
RKE will ask some questions around the cluster file like number of the hosts, ips, ssh users, etc, `--empty` option will generate an empty cluster.yml file, also if you just want to print on the screen and not save it in a file you can use `--print`.

## More details

More information about RKE design, configuration and usage can be found in this [blog post](http://rancher.com/an-introduction-to-rke/).


## License
Copyright (c) 2017 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
