# Intro

This repo contains 'cmd-admission-webhook-k8s' an application thar registers the Network Service Mesh
`Mutatingwebhookconfigurations` and responds to mutation requests.

# Usage

## Environment config

`cmd-admission-webhook-k8s` accept following environment variables:

| Variable | Type | Default | Description |
| -------- | ---- | ------- | ----------- |
| NSM_NAME | string | `admission-webhook-k8s` | Name of current admission webhook instance |
| NSM_SERVICE_NAME | string | `default` | Name of service that related to this admission webhook instance |
| NSM_NAMESPACE | string | `default` | Namespace where admission webhook is deployed |
| NSM_ANNOTATION | string | `networkservicemesh.io` | Name of annotation that means that the resource can be handled by admission-webhook |
| NSM_LABELS | map[string]string | | Map of labels and their values to be appended for each deployment that has `NSM_ANNOTATION` |
| NSM_NSURL_ENV_NAME | string | `NSM_NETWORK_SERVICES` | Name of environment variable that contains NSURL in initContainers/Containers |
| NSM_INIT_CONTAINER_IMAGES | []string | | List of init containers to be appended for each deployment that has `NSM_ANNOTATION` |
| NSM_CONTAINER_IMAGES | []string | | List of containers to be appended for each deployment that has `NSM_ANNOTATION` |
| NSM_ENVS | []string | | Additional environment variables to be appended to each appeded container and init container |
| NSM_CERT_FILE_PATH | string | | Path to certificate |
| NSM_KEY_FILE_PATH | string | | Path to RSA/Ed25519 related to `NSM_CERT_FILE_PATH` |
| NSM_CA_BUNDLE_FILE_PATH | string | | Path to cabundle file related to `NSM_CERT_FILE_PATH` |
| NSM_OPEN_TELEMETRY_ENDPOINT | string | `otel-collector.observability.svc.cluster.local:4317` | OpenTelemetry Collector Endpoint |

# Build

## Build cmd binary locally

You can build the locally by executing

```bash
go build ./...
```

## Build Docker container

You can build the docker container by running:

```bash
docker build .
```

# Testing

## Testing Docker container

Testing is run via a Docker container.  To run testing run:

```bash
docker run --privileged --rm $(docker build -q --target test .)
```

# Debugging

## Debugging the tests
If you wish to debug the test code itself, that can be acheived by running:

```bash
docker run --privileged --rm -p 40000:40000 $(docker build -q --target debug .)
```

This will result in the tests running under dlv.  Connecting your debugger to localhost:40000 will allow you to debug.

```bash
-p 40000:40000
```
forwards port 40000 in the container to localhost:40000 where you can attach with your debugger.

```bash
--target debug
```

Runs the debug target, which is just like the test target, but starts tests with dlv listening on port 40000 inside the container.

## Debugging the cmd

When you run 'cmd' you will see an early line of output that tells you:

```Setting env variable DLV_LISTEN_FORWARDER to a valid dlv '--listen' value will cause the dlv debugger to execute this binary and listen as directed.```

If you follow those instructions when running the Docker container:
```bash
docker run --privileged -e DLV_LISTEN_FORWARDER=:50000 -p 50000:50000 --rm $(docker build -q --target test .)
```

```-e DLV_LISTEN_FORWARDER=:50000``` tells docker to set the environment variable DLV_LISTEN_FORWARDER to :50000 telling
dlv to listen on port 50000.

```-p 50000:50000``` tells docker to forward port 50000 in the container to port 50000 in the host.  From there, you can
just connect dlv using your favorite IDE and debug cmd.

## Debugging the tests and the cmd

```bash
docker run --privileged -e DLV_LISTEN_FORWARDER=:50000 -p 40000:40000 -p 50000:50000 --rm $(docker build -q --target debug .)
```

Please note, the tests **start** the cmd, so until you connect to port 40000 with your debugger and walk the tests
through to the point of running cmd, you will not be able to attach a debugger on port 50000 to the cmd.