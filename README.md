# IVCAP - A Command-line Tool to interact with an IVCAP deployment

__IVCAP__ is helping researchers better investigate their domains and derive new insights  by collecting, processing and analysing multi-modal and multi-scale data and better facilitating data provenance and thereby fostering interdisciplinary collaboration.

__IVCAP__ has an extensive REST API which is usually called directly from applications or scientific notebooks. However, to support simple data operation from the command line, we developed this simple command-line tool. It only covers the subset of the IVCAP API, but we would be very excited to receive pull requests to extend it's functionality or fix bugs.

* [Install released binaries](#install)
* [Usage](#usage)
* [Build from source](#build)

## Install Released Binaries<a name="install"></a>

[binary-releases]: https://github.com/ivcap-works/ivcap-cli/releases/latest
[release-page]:    https://github.com/ivcap-works/ivcap-cli/releases

There are [ready to use binaries][binary-releases] for some architectures available at the repo's [release][release-page] tab.

If you use [homebrew](https://brew.sh/), you can install it by:

```
brew tap brew tap ivcap-works/ivcap
brew install ivcap
```

## Usage <a name="usage"></a>

```
% ivcap
A command line tool to to more conveniently interact with the
API exposed by a specific IVCAP deployment.

Usage:
  ivcap [command]

Available Commands:
  artifact    Create and manage artifacts
  aspect      Create and manage aspects
  collection  Create and manage collections
  completion  Generate the autocompletion script for the specified shell
  context     Manage and set access to various IVCAP deployments
  help        Help about any command
  order       Create and manage orders
  queue       Create and manage queues
  service     Create and manage services

Flags:
      --access-token string   Access token to use for authentication with API server [IVCAP_ACCESS_TOKEN]
      --context string        Context (deployment) to use
      --debug                 Set logging level to DEBUG
  -h, --help                  help for ivcap
      --no-history            Do not store history
  -o, --output string         Set format for displaying output [json, yaml]
      --silent                Do not show any progress information
      --timeout int           Max. number of seconds to wait for completion (default 30)
  -v, --version               version for ivcap

Use "ivcap [command] --help" for more information about a command.
```

### Configure context for a specific deployment

With the following command we are creating a context named `sd-dev` for the IVCAP deployment at `https://develop.ivcap.net`. Please check first the details of deployment you want to use.

```
% ivcap context create sd-dev https://develop.ivcap.net
Context 'sd-dev' created.
```

If we have multiple contexts, we can easily switch with `context set`

```
% ivcap context set sd-dev
Switched to context 'sd-dev'.
```

The following command lists all the configured contexts:

```
% ivcap context list
+---------+-----------+-----------------------------+--------------------------------+
| CURRENT | NAME      | ACCOUNTID                   | URL                            |
+---------+-----------+-----------------------------+--------------------------------+
| *       | sd-dev     | urn:ivcap:account:4c65b865  | <http://develop.ivcap.net> |
+---------+-----------+-----------------------------+--------------------------------+
```

To obtain an authorisation token, some deployments provide a username/password based identity provider.

```
% ivcap context login


    █▀▀▀▀▀█    ▀█  ▄▀▄▀▀ ▄▄▀▄ █▀▀▀▀▀█
    █ ███ █ █  █▀ ▀█▀ █  ▀▀█  █ ███ █
    █ ▀▀▀ █ █ ▀▀▄▀▀▀▀█▀ ▀█ ▀▀ █ ▀▀▀ █
    ▀▀▀▀▀▀▀ █▄█ █ █ █ █ █▄█ ▀ ▀▀▀▀▀▀▀
    █ ██▀▀▀▄▄▄ ▄ ██ ██▄█▀▄█▄█ ██▀██ ▄
    █▀▄▄ ▀▀  █ █▀█▀▀▀█▄  █  █ ▄ █▄█▀
...
To login to the IVCAP Service, please go to:  https://id-provider.com/activate?user_code=....
or scan the QR Code to be taken to the login page
Waiting for authorisation...
```

Follow this [link](./doc/ivcap_context.md) for more details about the `context` command.

### Services

To list all available services:

```
% ivcap services list --limit 2
+--------+------------------------+-------------------------------+
| ID     | NAME                   | ACCOUNT                       |
+--------+------------------------+-------------------------------+
| @1     | image-analysis-example | urn:ivcap:account:45a06508... |
| @2     | cv-pipeline-v0-ps1-gpu | urn:ivcap:account:29df453d... |
| ... @3 |                        |                               |
+--------+------------------------+-------------------------------+
```

To get more details about a specific service

```
% ivcap service get @1

          ID  urn:ivcap:service:19f9c31e...
        Name  image-analysis-example
 Description  A simple IVCAP service creating a thumbnail and reporting stats on a collection of images
      Status
  Account ID  urn:ivcap:account:45a06508...
  Parameters  ┌────────┬────────────────────────────────┬────────────┬─────────┬──────────┐
              │ NAME   │ DESCRIPTION                    │ TYPE       │ DEFAULT │ OPTIONAL │
              ├────────┼────────────────────────────────┼────────────┼─────────┼──────────┤
              │ images │ Collection of image artifacts. │ collection │         │ true     │
              ├────────┼────────────────────────────────┼────────────┼─────────┼──────────┤
              │  width │ Thumbnail width.               │ int        │ 100     │ false    │
              ├────────┼────────────────────────────────┼────────────┼─────────┼──────────┤
              │ height │ Thumbnail height.              │ int        │ 100     │ false    │
              └────────┴────────────────────────────────┴────────────┴─────────┴──────────┘
```

Follow this [link](./doc/ivcap_service.md) for more details about the `service` command.

### Orders

To place an order:

```
% ivcap orders create \
     urn:ivcap:service:d939b74d... \
     --name "Order for max" \
     msg="Hi, how are you"
Order 'urn:ivcap:order:81b204e8...' with status 'Pending' submitted.

```

To check on the status of an order:

```
% ivcap orders get urn:ivcap:order:81b204e8...

         ID  urn:ivcap:order:f169f54d-ec8d-4d6a-af17-0c1c33625379
       Name  urn:ibenthos:collection:indo_flores_0922:LB4 UQ PhotoTransect@256028
     Status  succeeded
    Ordered  6 months ago (01 Oct 23 17:26 AEDT)
    Service  image-analysis-example (@15)
 Account ID  urn:ivcap:account:45a06508-5c3a-4678-8e6d-e6399bf27538
 Parameters  ┌─────────────────────────────────────────────────┐
             │ images =  @1 (urn:ivcap:collection:508a2aba...) │
             │  width =  100                                   │
             │ height =  100                                   │
             └─────────────────────────────────────────────────┘
   Products  ┌────┬───────────────┬──────────────────┐
             │ @2 │ result.png    │ image/png        │
             │ @3 │ stats.json    │ application/json │
             │ @4 │ thumbnail.png │ image/png        │
             └────┴───────────────┴──────────────────┘
   Metadata  ┌─────┬────────────────────────────────────────┐
             │ @6  │ urn:ivcap:schema:order-uses-workflow.  │
             │ @7  │ urn:ivcap:schema:order-uses-artifact.1 │
             │ ...                                          │
             └─────┴────────────────────────────────────────┘
```

Follow this [link](./doc/ivcap_order.md) for more details about the `order` command.

### Artifacts

To check the details of the artifact created by the previously placed order:

```
% ivcap artifact get urn:ivcap:artifact:017ecae8...

         ID  urn:ivcap:artifact:017ecae8...
       Name  out.png
     Status  available
       Size  50855
  Mime-type  image/png
 Account ID  urn:ivcap:account:58d8e161...
   Metadata  ┌────┬─────────────────────────────────────────────┐
             │ @1 │ urn:ivcap:schema:artifact.1                 │
             │ @2 │ urn:ivcap:schema:artifact-usedBy-order.1    │
             │ @3 │ urn:example:schema:image-analysis:thumbnail │
             └────┴─────────────────────────────────────────────┘
```

To download the content associated with the artifact.

```
% ivcap artifact download urn:ivcap:artifact:017ecae8... -f /tmp/out.png
Successfully wrote 50855 bytes to /tmp/out.png
```

Follow this [link](./doc/ivcap_artifact.md) for more details about the `artifact` command.

### Packages
#### To upload service docker image
```
ivcap package push -f alpine:3.20.1

 Pushing alpine:3.20.1 from local, may take multiple minutes depending on the size of the image ...
 40df18f632     4.21MB already exits
 092561eea8       1.45KB uploaded
 registry.kube-system.svc.cluster.local/0f0e3f57-80f7-4899-9b69-459af2efd789/alpine:3.20.1 pushed

```

#### To upload a service docker image over 2G, which needs to push from a local registry

- Start a local registry
```
docker run -d -p 8080:5000 --name registry-2 registry:2

```
- Tag image
```
docker tag alpine:3.20.1 localhost:8080/alpine:3.20.1
```
- Push to local registry
```
docker push localhost:8080/alpine:3.20.1
```
- Then push from local registry to ivcap service
```
ivcap package push -f localhost:8080/alpine:3.20.1
 Pushing localhost:8080/alpine:3.20.1 from localhost:8080, may take multiple minutes depending on the size of the image ...
 a258b2a6b5     3.90MB uploaded
 092561eea8       1.45KB uploaded
 registry.kube-system.svc.cluster.local/0f0e3f57-80f7-4899-9b69-459af2efd789/alpine:3.20.1 pushed
```

#### List packages
```
ivcap package ls

registry.kube-system.svc.cluster.local/0f0e3f57-80f7-4899-9b69-459af2efd789/alpine:3.20.1
registry.kube-system.svc.cluster.local/0f0e3f57-80f7-4899-9b69-459af2efd789/cv_pipeline_v0_pm1:2024-03-27_16-48-57
registry.kube-system.svc.cluster.local/0f0e3f57-80f7-4899-9b69-459af2efd789/python:3.13.0
```

Follow this [link](./doc/ivcap_package.md) for more details about the `package` command.

## Build from Source <a name="build"></a>

### Prerequisites

You will need the following installed:

[golangci-lint]: https://golangci-lint.run/welcome/install/#local-installation
[gocritic]:      https://github.com/go-critic/go-critic?tab=readme-ov-file#installation
[staticcheck]:   https://staticcheck.dev/docs/getting-started/#installation
[gosec]:         https://github.com/securego/gosec?tab=readme-ov-file#install
[govulncheck]:   https://go.googlesource.com/vuln
[addlicense]:    https://github.com/nokia/addlicense?tab=readme-ov-file#install-as-a-go-program

- go version >= 1.22.5 (e.g. `snap install go --classic`)
- [golangci-lint] (e.g. `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.57.2`)
- [gocritic] (e.g. `go install -v github.com/go-critic/go-critic/cmd/gocritic@latest`; you may also need to add `~/go/bin` to your `PATH`)
- [staticcheck] (e.g. `go install honnef.co/go/tools/cmd/staticcheck@latest`)
- [gosec] (e.g. `go install github.com/securego/gosec/v2/cmd/gosec@latest`)
- [govulncheck] (e.g. `go install golang.org/x/vuln/cmd/govulncheck@latest`)
- [addlicense] (e.g. `go install github.com/nokia/addlicense@latest`)

#### Install prerequisites

The prerequisite tools can be installed by running the make target:

```shell
make install-tools
```

### Build & Install

To build and install from local source code, ensure you have the prerequisites
and run:

```shell
make build
make install
```

If your Go paths are configured correctly, you should now have the `ivcap`
command available in your shell.

To build and install without performing any code checks (implicitly done via `make
check`) run:

```shell
make build-dangerously
make install-dangerously
```
