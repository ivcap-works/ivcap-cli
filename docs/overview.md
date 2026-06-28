# Overview

**`ivcap`** is the official command-line interface for interacting with an
[IVCAP](https://github.com/ivcap-works) deployment. It can be used directly
by humans at a terminal, and is also used by AI agents via its built-in
[MCP server](#agent-mcp-usage).

---

## What is IVCAP?

IVCAP (_Integrated Virtual Collaborative Analysis Platform_) is a managed,
provenance-aware platform for running, building, and orchestrating analytic
services and AI agents. It is designed to help researchers investigate their
domains and derive new insights by:

- **collecting, processing and analysing** multi-modal and multi-scale data,
- **tracking provenance** automatically — recording which service processed
  which inputs to produce which outputs, and
- **fostering collaboration** across disciplines by making data lineage
  transparent and reproducible.

IVCAP exposes a REST API. This CLI wraps that API into convenient shell
commands and also acts as a local MCP server so that AI agents can interact
with the platform programmatically.

---

## Core concepts

| Concept | Description |
|---------|-------------|
| **Service** | A packaged, executable capability (think "function endpoint with configuration"). Services are registered by developers and can be invoked by any authorised user. |
| **Job** | A single execution of a service with a specific set of inputs. Jobs progress through states (`pending → executing → succeeded/failed`) and emit events. |
| **Artifact** | Immutable blob-like data referenced by URN (images, JSON files, tar'd datasets, …). Artifacts are produced and consumed by jobs. |
| **Data Fabric** | A metadata graph that stores _aspects_ — structured records attached to any entity (artifact, job, service, …). Used for provenance, annotation, and discovery. |
| **Aspect** | A single structured record in the Data Fabric, described by a schema URN. IVCAP writes provenance aspects automatically; users can write their own. |
| **Queue** | An ordered message channel for asynchronous communication between services. |
| **Secret** | A named credential stored securely in the platform for use by services at runtime. |
| **Package** | A Docker container image stored in the platform's registry — the execution unit behind a service. |
| **Collection** | A named group of artifacts or other resources, useful for organising experiment outputs. |

---

## What you can do with this CLI

### Manage the platform from the command line

```bash
# Set up a deployment context
ivcap context create my-deployment https://my.ivcap.net
ivcap context login

# Discover services
ivcap service list
ivcap service get urn:ivcap:service:…

# Submit and monitor jobs
ivcap job create urn:ivcap:service:… -f params.json --watch

# Work with artifacts
ivcap artifact list
ivcap artifact get urn:ivcap:artifact:…
ivcap artifact download urn:ivcap:artifact:… -f output.bin

# Query the Data Fabric
ivcap datafabric query --entity urn:ivcap:artifact:…
```

### Run Nextflow pipelines

The `nextflow` subcommand provides higher-level wrappers for deploying and
running [Nextflow](https://www.nextflow.io/)-based pipeline services:

```bash
ivcap nextflow create ./my-pipeline.tar.gz
ivcap nextflow run urn:ivcap:service:… -f params.json --watch
ivcap nextflow job-view urn:ivcap:job:…   # open execution report in browser
```

### Agent / MCP usage

`ivcap mcp` starts a local
[Model Context Protocol](https://modelcontextprotocol.io/) server that exposes
all platform operations as MCP tools. AI agents (Claude, GPT, LangChain, …)
connect to this server and can submit jobs, download results, query provenance,
and build multi-step pipelines — all without writing custom API code.

```bash
# Start the MCP server (STDIO mode — used by most agent frameworks)
ivcap mcp

# Start the MCP server in SSE mode
ivcap mcp --port 8088
```

The server also exposes embedded _skill playbooks_ as MCP Resources so that
agents can discover best practices and workflow patterns at runtime:

```bash
ivcap skills list
ivcap skills show ivcap-job-create
```

---

## Installation

Pre-built binaries are available for macOS, Linux, and Windows at the
[GitHub releases page](https://github.com/ivcap-works/ivcap-cli/releases).

**macOS (Homebrew):**
```bash
brew tap ivcap-works/ivcap
brew install ivcap
```

**From source:**
```bash
git clone https://github.com/ivcap-works/ivcap-cli.git
cd ivcap-cli
make build
make install
```

---

## Getting started

```bash
# 1. Create a context for your IVCAP deployment
ivcap context create mydeployment https://your.ivcap.net

# 2. Log in
ivcap context login

# 3. List available services
ivcap service list --limit 10

# 4. Submit a job
ivcap job create urn:ivcap:service:<uuid> -f job-params.json --watch
```

See the [Command Reference](index.md) for the full list of commands.
