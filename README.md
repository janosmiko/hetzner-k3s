# Create production grade Kubernetes clusters in Hetzner Cloud in a couple of minutes or less

[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_hetzner-k3s&metric=ncloc)](https://sonarcloud.io/dashboard?id=janosmiko_hetzner-k3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/rewardenv/reward)](https://goreportcard.com/report/github.com/janosmiko/hetzner-k3s)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_hetzner-k3s&metric=security_rating)](https://sonarcloud.io/dashboard?id=janosmiko_hetzner-k3s)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_hetzner-k3s&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=janosmiko_hetzner-k3s)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_hetzner-k3s&metric=sqale_rating)](https://sonarcloud.io/dashboard?id=janosmiko_hetzner-k3s)

This is a CLI tool to quickly create and manage Kubernetes clusters in [Hetzner Cloud](https://www.hetzner.com/cloud)
using the lightweight Kubernetes distribution [k3s](https://k3s.io/) from [Rancher](https://rancher.com/).

Hetzner Cloud is an awesome cloud provider which offers a truly great service with the best performance/cost ratio in
the market. With Hetzner's Cloud Controller Manager and CSI driver you can provision load balancers and persistent
volumes very easily.

k3s is my favorite Kubernetes distribution now because it uses much less memory and CPU, leaving more resources to
workloads. It is also super quick to deploy because it's a single binary.

Using this tool, creating a highly available k3s cluster with 3 masters for the control plane and worker nodes (with
autoscaling support) takes about **a couple of minutes** only. This includes

- creating the infra resources (servers, private network, firewall, load balancer for the API server for HA clusters)
- deploying k3s to the nodes
- installing the [Hetzner Cloud Controller Manager](https://github.com/hetznercloud/hcloud-cloud-controller-manager) to
  provision load balancers right away
- installing the [Hetzner CSI Driver](https://github.com/hetznercloud/csi-driver) to provision persistent volumes using
  Hetzner's block storage
- installing the [Rancher System Upgrade Controller](https://github.com/rancher/system-upgrade-controller) to make
  upgrades to a newer version of k3s easy and quick
- installing the [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler) to make the cluster
  autoscaling possible

See roadmap [here](https://github.com/users/janosmiko/projects/1) for the features planned or in progress.

Also see this [wiki page](https://github.com/vitobotta/hetzner-k3s/wiki/Tutorial:---Setting-up-a-cluster) for a tutorial
on how to set up a cluster with the most common setup to get you started.

## Requirements

All that is needed to use this tool is

- an Hetzner Cloud account
- an Hetzner Cloud token: for this you need to create a project from the cloud console, and then an API token with **
  both read and write permissions** (sidebar > Security > API Tokens); you will see the token only once, so ensure you
  take note of it somewhere safe
- (optional) Github token: if you reach the rate limiting to query the available K3s releases

## Installation

### Dependency

If you use these binaries please make sure to install `kubectl` in your PATH before using `hetzner-k3s`.

### MacOS

```bash
$ brew install janosmiko/tap/hetzner-k3s
```

#### Binary Download

```bash
$ curl -fsSLO "https://github.com/janosmiko/hetzner-k3s/releases/latest/download/hetzner-k3s_`uname -s`_`uname -m`.tar.gz"
$ tar -zxvf "hetzner-k3s_`uname -s`_`uname -m`.tar.gz" -C /usr/local/bin/
$ rm -f "hetzner-k3s_`uname -s`_`uname -m`.tar.gz"
$ chmod +x /usr/local/bin/hetzner-k3s
```

### Linux

#### Ubuntu

```
$ curl -fsSLO "https://github.com/janosmiko/hetzner-k3s/releases/latest/download/hetzner-k3s_`uname -s`_`uname -m`.deb"
$ sudo dpkg -i "hetzner-k3s_`uname -s`_`uname -m`.deb"
```

##### CentOS and Fedora

```
$ yum install -y "https://github.com/janosmiko/hetzner-k3s/releases/latest/download/hetzner-k3s_`uname -s`_`uname -m`.rpm"
```

##### Binary Download

```
$ curl -fsSLO "https://github.com/janosmiko/hetzner-k3s/releases/latest/download/hetzner-k3s_`uname -s`_`uname -m`.tar.gz"
$ tar -zxvf "hetzner-k3s_`uname -s`_`uname -m`.tar.gz" -C /usr/local/bin/
$ rm -f "hetzner-k3s_`uname -s`_`uname -m`.tar.gz"
$ chmod +x /usr/local/bin/hetzner-k3s
```


```bash
$ curl -fsSLO "https://github.com/janosmiko/hetzner-k3s/releases/latest/download/hetzner-k3s-`uname -s`-`uname -m`.tar.gz"
$ tar -zxvf "hetzner-k3s-`uname -s`-`uname -m`.tar.gz" hetzner-k3s
$ sudo mv hetzner-k3s /usr/local/bin/hetzner-k3s
```

### Windows

Download and extract the [latest](https://github.com/janosmiko/hetzner-k3s/releases/latest/) binary to your PATH.

### With Docker

Alternatively, if you don't want to install the binary but have Docker installed, you can use a container. Run the
following from inside the directory where you have the config file for the cluster (described in the next section):

```bash
docker run --rm -it \
  -v ${PWD}:/cluster \
  -v ${HOME}/.ssh:/tmp/.ssh \
  janosmiko/hetzner-k3s:v0.1.7 \
  create-cluster \
  --config-file /cluster/test.yaml
```

Replace `test.yaml` with the name of your config file.

## Creating a cluster

The tool requires a simple configuration file in order to create/upgrade/delete clusters, in the YAML format like in the
example below:

```yaml
---
hetzner_token: <your token>
#github_token: <your token> # Optional
cluster_name: test
kubeconfig_path: "/cluster/kubeconfig"
k3s_version: v1.21.3+k3s1
public_ssh_key_path: "/root/.ssh/id_rsa.pub"
private_ssh_key_path: "/root/.ssh/id_rsa"
#
# Using these fields you can apply various firewall restrictions for SSH and Kubernetes API access.
# Defaults to ALLOW ANY IP address (0.0.0.0/0)
#
#ssh_allowed_networks:
#  - 0.0.0.0/0
#api_allowed_networks:
#  - 0.0.0.0/0

verify_host_key: false
location: nbg1
schedule_workloads_on_masters: false
masters:
  instance_type: cpx21
  instance_count: 3
#
# Define static worker pools
#
worker_node_pools:
  - name: small
    instance_type: cpx21
    instance_count: 4
  - name: big
    instance_type: cpx31
    instance_count: 2
#
# Cluster Autoscaler is automatically enabled when any autoscaling_node_pools are defined. 
#
autoscaling_node_pools:
  - name: small
    instance_type: cx41
    instance_min: 3
    instance_max: 5
  - name: big
    instance_type: cpx51
    instance_max: 10
#
# Pass additional parameters to the Cluster Autoscaler.
# https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca
#
#cluster_autoscaler_args:
#  - --scan-interval=2s
#  - --scale-down-delay-after-add=1h0s
#  - --scale-down-unneeded-time=1h0s
#  - --scale-down-unready-time=10m0s
#  - --scale-down-utilization-threshold=0.5
#  - --ignore-daemonsets-utilization
#  - --ignore-mirror-pods-utilization
#  - --v=4

additional_packages:
  - somepackage
post_create_commands:
  - apt update
  - apt upgrade -y
  - apt autoremove -y
  - shutdown -r now
enable_encryption: true
#
# Configure various arguments for kubernetes components
#
#kube_api_server_args:
#  - arg1
#  - ...
#kube_scheduler_args:
#  - arg1
#  - ...
#kube_controller_manager_args:
#  - arg1
#  - ...
#kube_cloud_controller_manager_args:
#  - arg1
#  - ...
#kubelet_args:
#  - arg1
#  - ...
#kube_proxy_args:
#  - arg1
#  - ...
#
# Specify if you want to use an existing network, otherwise one will be created for this cluster.
# existing_network: <network-name> or <network-id>
# This setting allows you to override the default nameservers (1.1.1.1 and 1.0.0.1)
# 
#default_nameservers:
#  - 8.8.8.8
#  - 1.1.1.1
#  - 1.0.0.1
#
# Enabling this will blacklist longhorn devices in multipathd config (required for longhorn storage)
# https://longhorn.io/kb/troubleshooting-volume-with-multipath/
#
#fix_multipath: true
#
# Running the create-cluster command always overrides the default Kubernetes Storage Class to hcloud-volume.
# By disabling this setting you can disable this behaviour leaving the default Kubernetes Storage Class as it is.
#
#hcloud_volume_is_default_storage_class: false
#
# Hcloud CSI Controller Deployment is scheduled on workers by default. If you enable this setting, the deployment will
# be upgraded to run on 
#
#schedule_csi_controller_on_master: false
#
```

It should hopefully be self-explanatory; you can run `hetzner-k3s releases` to see a list of the available releases from
the most recent to the oldest available. You can also filter the list using `hetzner-k3s releases --filter v1.24` and
fetch the latest tag using `hetzner-k3s releases --latest` (be careful: this can show release candidate releases too).
You can also combine them eg: `hetzner-k3s releases --filter v1.21 --latest`.

If you are using Docker, then set `kubeconfig_path` to `/cluster/kubeconfig` so that the kubeconfig is created in the
same directory where your config file is. Also set the config file path to `/cluster/<filename>`.

If you don't want to specify the Hetzner token in the config file (for example if you want to use the tool with CI),
then you can use the `HCLOUD_TOKEN` environment variable instead, which has predecence. This also works for any possible
configuration variable.

**Important**: The tool assignes the label `cluster` to each server it creates, with the cluster name you specify in the
config file, as the value. So please ensure you don't create unrelated servers in the same project having
the label `cluster=<cluster name>`, because otherwise they will be deleted if you delete the cluster. I recommend you
create a separate Hetzner project for each cluster, see note at the end of this README for more details.

The Servers which are created by the Cluster Autoscaler use a different label `hcloud/node-group`.

If you set `masters.instance_count` to 1 then the tool will create a non highly available control plane; for production
clusters you may want to set it to a number greater than 1. This number must be odd to avoid split brain issues with
etcd and the recommended number is 3.

You can specify any number of worker node pools for example to have mixed nodes with different specs for different
workloads.

At the moment Hetzner Cloud has four locations: two in Germany (`nbg1`, Nuremberg and `fsn1`, Falkenstein), one in
Finland (`hel1`, Helsinki) and two in the USA (`ash`, Ashburn, Virginia and `hil`, Hillsboro, Oregon). Please note that the Ashburn, Virginia
location has just been announced and it's limited to AMD instances for now.

For the available instance types and their specs, either check from inside a project when adding a server manually or
run the following with your Hetzner token:

```bash
curl \
	-H "Authorization: Bearer $API_TOKEN" \
	'https://api.hetzner.cloud/v1/server_types'
```

By default, the image in use is Ubuntu 20.04, but you can specify an image to use with the `image` config option. This
makes it also possible to use a snapshot that you have already created from and existing server (for example to
preinstall some tools). If you want to use a custom snapshot you'll need to specify the **ID** of the snapshot/image,
not the description you gave when you created the template server. To find the ID of your custom image/snapshot, run:

```bash
curl \
	-H "Authorization: Bearer $API_TOKEN" \
	'https://api.hetzner.cloud/v1/images'
```

Notes:

- if you use a custom image, the creation of the servers may take longer than when using the default image
- the option `verify_host_key` is by default set to `false` to disable host key verification. This is because sometimes
  when creating new servers, Hetzner may assign IP addresses that were previously used by other servers you owned in the
  past. Therefore the host key verification would fail. If you set this option to `true` and this happens, the tool
  won't be able to continue creating the cluster until you resolve the issue with one of the suggestions it will give
  you
- the setting `api_allowed_networks` allows specifying which networks can access the Kubernetes API, but this only works
  with single master clusters currently. Multi-master HA clusters require a load balancer for the API, but load
  balancers are not yet covered by Hetzner's firewalls.

Finally, to create the cluster run:

```bash
hetzner-k3s create-cluster --config-file cluster_config.yaml
```

This will take a couple of minutes or less depending on the number of masters and worker nodes.

If you are creating an HA cluster and see the following in the output you can safely ignore it - it happens when
additional masters are joining the first one:

```
Job for k3s.service failed because the control process exited with error code.
See "systemctl status k3s.service" and "journalctl -xe" for details.
```

### Idempotency

The `create-cluster` command can be run any number of times with the same configuration without causing any issue, since
the process is idempotent. This means that if for some reason the create process gets stuck or throws errors (for
example if the Hetzner API is unavailable or there are timeouts etc), you can just stop the current command, and re-run
it with the same configuration to continue from where it left.

### Adding nodes

To add one or more nodes to a node pool, just change the instance count in the configuration file for that node pool and
re-run the create command.

### Scaling down a node pool

To make a node pool smaller:

- decrease the instance count for the node pool in the configuration file so that those extra nodes are not recreated in
  the future
- delete the nodes from Kubernetes (`kubectl delete node <name>`)
- delete the instances from the cloud console (make sure you delete the correct ones :p)

In a future relese I will add some automation for the cleanup.

### Replacing a problematic node

- delete the node from Kubernetes (`kubectl delete node <name>`)
- delete the correct instance from the cloud console
- re-run the create script. This will re-create the missing node and have it join to the cluster

### Converting a non-HA cluster to HA

It's easy to convert a non-HA with a single master cluster to HA with multiple masters. Just change the masters instance
count and re-run the create command. This will create a load balancer for the API server and update the kubeconfig so
that all the API requests go through the load balancer.

## Upgrading to a new version of k3s

If it's the first time you upgrade the cluster, all you need to do to upgrade it to a newer version of k3s is run the
following command:

```bash
hetzner-k3s upgrade-cluster --config-file cluster_config.yaml
```

So you just need to specify the new k3s version as an additional parameter and the configuration file will be updated
with the new version automatically during the upgrade. To see the list of available k3s releases run the
command `hetzner-k3s releases`.

Note that the API server will briefly be unavailable during the upgrade of the controlplane.

To check the upgrade progress, run `watch kubectl get nodes -owide`. You will see the masters being upgraded one per
time, followed by the worker nodes.

### What to do if the upgrade doesn't go smoothly

If the upgrade gets stuck for some reason, or it doesn't upgrade all the nodes:

1. Clean up the existing upgrade plans and jobs, and restart the upgrade controller

```bash
kubectl -n system-upgrade delete job --all
kubectl -n system-upgrade delete plan --all

kubectl label node --all plan.upgrade.cattle.io/k3s-server- plan.upgrade.cattle.io/k3s-agent-

kubectl -n system-upgrade rollout restart deployment system-upgrade-controller
kubectl -n system-upgrade rollout status deployment system-upgrade-controller
```

I recommend running the above commands also when upgrading a cluster that has already been upgraded at least once
previously, since the upgrade leaves some stuff behind that needs to be cleaned up.

2. Re-run the `upgrade-cluster` command with an additional parameter.

I have noticed that sometimes I need to re-run the upgrade command a couple of times to complete an upgrade
successfully. Must be some bug in the system upgrade controller but I haven't investigated further.

You can also check the logs of the system upgrade controller's pod:

```bash
kubectl -n system-upgrade \
  logs -f $(kubectl -n system-upgrade get pod -l pod-template-hash -o jsonpath="{.items[0].metadata.name}")
```

A final note about upgrades is that if for some reason the upgrade gets stuck after upgrading the masters and before
upgrading the worker nodes, just cleaning up the resources as described above might not be enough. In that case also try
running the following to tell the upgrade job for the workers that the masters have already been upgraded, so the
upgrade can continue for the workers:

```bash
kubectl label node <master1> <master2> <master2> plan.upgrade.cattle.io/k3s-server=upgraded
```

## Upgrading the OS on nodes

- consider adding a temporary node during the process if you don't have enough spare capacity in the cluster
- drain one node
- update etc
- reboot
- uncordon
- proceed with the next node

## Deleting a cluster

To delete a cluster, running

```bash
hetzner-k3s delete-cluster --config-file cluster_config.yaml
```

This will delete all the resources in the Hetzner Cloud project for the cluster being deleted.

## Additional info

### Load balancers

Once the cluster is ready, you can already provision services of type LoadBalancer for your workloads (such as the Nginx
ingress controller for example) thanks to the Hetzner Cloud Controller Manager that is installed automatically.

There are some annotations that you can add to your services to configure the load balancers. I personally use the
following:

```yaml
  service:
    annotations:
      load-balancer.hetzner.cloud/hostname: <a valid fqdn>
      load-balancer.hetzner.cloud/http-redirect-https: 'false'
      load-balancer.hetzner.cloud/location: nbg1
      load-balancer.hetzner.cloud/name: <lb name>
      load-balancer.hetzner.cloud/uses-proxyprotocol: 'true'
      load-balancer.hetzner.cloud/use-private-ip: "true"
```

I set `load-balancer.hetzner.cloud/hostname` to a valid hostname that I configure (after creating the load balancer)
with the IP of the load balancer; I use this together with the
annotation `load-balancer.hetzner.cloud/uses-proxyprotocol: 'true'` to enable the proxy protocol. Reason: I enable the
proxy protocol on the load balancers so that my ingress controller and applications can "see" the real IP address of the
client. However when this is enabled, there is a problem where [cert-manager](https://cert-manager.io/docs/) fails
http01 challenges; you can find an explanation of why [here](https://github.com/compumike/hairpin-proxy) but the easy
fix provided by some providers - including Hetzner - is to configure the load balancer so that it uses a hostname
instead of an IP. Again, read the explanation for the reason but if you care about seeing the actual IP of the client
then I recommend you use these two annotations.

The annotation `load-balancer.hetzner.cloud/use-private-ip: "true"` ensures that the communication between the load
balancer and the nodes happens through the private network, so we don't have to open any ports on the nodes (other than
the port 6443 for the Kubernetes API server).

The other annotations should be self explanatory. You can find a list of the available
annotations [here](https://pkg.go.dev/github.com/hetznercloud/hcloud-cloud-controller-manager/internal/annotation).

## Persistent volumes

Once the cluster is ready you can create persistent volumes out of the box with the default storage
class `hcloud-volumes`, since the Hetzner CSI driver is installed automatically. This will use Hetzner's block storage (
based on Ceph so it's replicated and highly available) for your persistent volumes. Note that the minimum size of a
volume is 10Gi. If you specify a smaller size for a volume, the volume will be created with a capacity of 10Gi anyway.

## Keeping a project per cluster

I recommend that you create a separate Hetzner project for each cluster, because otherwise multiple clusters will
attempt to create overlapping routes. I will make the pod cidr configurable in the future to avoid this, but I still
recommend keeping clusters separated from each other. This way, if you want to delete a cluster with all the resources
created for it, you can just delete the project.

## Contributing and support

Please create a PR if you want to propose any changes, or open an issue if you are having trouble with the tool - I will
do my best to help if I can.

## License

The project is available as open source under the terms of the [MIT License](https://opensource.org/licenses/MIT).

## Code of Conduct

Everyone interacting in the hetzner-k3s project's codebases, issue trackers, chat rooms and mailing lists is expected to
follow the [code of conduct](https://github.com/janosmiko/hetzner-k3s/blob/main/CODE_OF_CONDUCT.md).

## Acknowledgement

This project was inspired by the awesome [project](https://github.com/vitobotta/hetzner-k3s)
by [Vito Botta](https://github.com/vitobotta).

## Stargazers over time

[![Stargazers over time](https://starchart.cc/janosmiko/hetzner-k3s.svg)](https://starchart.cc/janosmiko/hetzner-k3s)

