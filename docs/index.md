# IVCAP CLI

**`ivcap`** is the official command-line interface for interacting with an
[IVCAP](https://github.com/ivcap-works) deployment. It can be used directly
by humans at a terminal, and is also used by AI agents via its built-in
[MCP server](ivcap_mcp.md).

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

Each row links to the CLI command that manages that resource.

| Concept | Description | Command |
|---------|-------------|---------|
| **Service** | A packaged, executable capability. Services are registered by developers and invoked by any authorised user. | [service](ivcap_service.md) |
| **Job** | A single execution of a service. Jobs progress through states (`pending → executing → succeeded/failed`) and emit events. | [job](ivcap_job.md) |
| **Artifact** | Immutable blob-like data (images, JSON, tar'd datasets) produced and consumed by jobs. | [artifact](ivcap_artifact.md) |
| **Collection** | A named group of artifacts or resources, useful for organising experiment outputs. | [collection](ivcap_collection.md) |
| **Data Fabric** | A metadata graph storing _aspects_ — structured records attached to any entity (job, artifact, service, …). Used for provenance and discovery. | [datafabric](ivcap_datafabric.md) |
| **Queue** | An ordered message channel for asynchronous communication between services. | [queue](ivcap_queue.md) |
| **Secret** | A named credential stored securely in the platform for use by services at runtime. | [secret](ivcap_secret.md) |
| **Package** | A container image in the platform's registry — the execution unit behind a service. | [package](ivcap_package.md) |
| **Context** | A named deployment endpoint and credentials for the CLI to connect to an IVCAP installation. | [context](ivcap_context.md) |
| **Nextflow pipeline** | A Nextflow DSL2 pipeline deployed as an IVCAP service for scalable scientific workflows. | [nextflow](ivcap_nextflow.md) |

### Agent support

| Concept | Description | Command |
|---------|-------------|---------|
| **MCP server** | Exposes all platform operations as [Model Context Protocol](https://modelcontextprotocol.io/) tools for AI agents. | [mcp](ivcap_mcp.md) |
| **Skill playbook** | Embedded best-practice workflow guides that agents can read at runtime. | [skills](ivcap_skills.md) |
| **Agent context** | Embedded operational guidance (safe defaults, URN conventions, etc.) for AI agents. | [agent-context](ivcap_agent-context.md) |

---

**New to IVCAP?** See the [Getting Started](getting-started.md) guide for
installation, context setup, and your first job.
