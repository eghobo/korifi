> **Warning**
> Make sure you are using the correct version of these instructions by using the link in the release notes for the version you're trying to install. If you're not sure, check our [latest release](https://github.com/cloudfoundry/korifi/releases/latest).

# Install Korifi on kind

This document integrates our [install instructions](./INSTALL.md) with specific tips to install Korifi locally using [kind](https://kind.sigs.k8s.io/).

## Cluster creation

In order to access the Korifi API, we'll need to [expose the cluster ingress locally](https://kind.sigs.k8s.io/docs/user/ingress/). To do it, create your kind cluster using a command like this:

```sh
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF
```

## Initial setup

Export the following environment variables:

```sh
export ROOT_NAMESPACE="cf"
export ADMIN_USERNAME="kubernetes-admin"
export BASE_DOMAIN="vcap.me"
```

`vcap.me` will conveniently [resolve to `127.0.0.1`](https://www.nslookup.io/domains/vcap.me/dns-records), which is exactly what we need.

## Container registry

We recommend you use [DockerHub](https://hub.docker.com/) as your container registry.

## Dependencies

No changes here, follow the instructions.

## DNS

You can skip this section.

## Deploy Korifi

No changes here. If using DockerHub as recommended above, set the following values:

* `api.packageRegistry`: `index.docker.io/<username>`
* `kpack-image-builder.builderRepository`: `index.docker.io/<username>/kpack-builder`
* `kpack-image-builder.packageRegistry`: `index.docker.io/<username>`

## Post-install Configuration

For the container registry credentials `Secret`, we recommend you [create an access token](https://hub.docker.com/settings/security?generateToken=true) on DockerHub. No changes otherwise.

## Test Korifi

No changes here, follow the instructions. When running `cf login`, make sure you select the entry associated to your kind cluster (`kind-kind` by default).