# Nextflow skills

Skills for building and packaging Nextflow pipelines for IVCAP.

## Core Skills (Start Here)

- **Pipeline Basics**: `skills://nextflow-pipeline-basics/SKILL.md`
  - Core Nextflow DSL2 fundamentals: project structure, ivcap.yaml, main.nf, config
  - Container requirements and dependency management
  - Standard workflow patterns and channel operations
  - **Use when:** Starting a new pipeline or learning Nextflow basics

- **Pipeline Deployment**: `skills://nextflow-pipeline-deployment/SKILL.md`
  - Validation, packaging into tar.gz, and deployment to IVCAP
  - Service ID generation and management
  - Job submission and parameter handling
  - **Use when:** Ready to package and deploy a working pipeline

## Advanced Skills

- **MCP Tools Usage**: `skills://nextflow-mcp-tools/SKILL.md`
  - Using MCP tools (nextflow_create, nextflow_run, artifact_build)
  - Handling large pipelines with artifact_build pattern
  - Source types, size limits, and job polling
  - **Use when:** Deploying pipelines programmatically via MCP

- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
  - Container requirements and verification
  - Script invocation best practices
  - Samples schemas and provenance tracking
  - **Use when:** Building production-ready, reusable pipelines

- **Examples**: `skills://nextflow-examples/SKILL.md`
  - Complete real-world pipeline examples (genome assembly, image analysis)
  - Detailed ivcap.yaml configurations with comprehensive documentation
  - **Use when:** Need a template or reference for similar workflows

## Debugging Skills

- **General Debugging**: `skills://nextflow-debugging/SKILL.md`
  - Service ID reuse and iterative development
  - Accessing Nextflow logs from failed runs
  - Common debugging patterns and log analysis
  - **Use when:** Pipeline execution fails or needs debugging

- **MCP Debugging**: `skills://nextflow-mcp-debugging/SKILL.md`
  - Diagnosing MCP tool failures
  - Actionable feedback format and tool improvement reporting
  - Pattern recognition for common tool issues
  - **Use when:** MCP tool calls fail unexpectedly

## Legacy Skills (Archived)

The following large files have been split into focused, modular skills above:
- `nextflow-pipeline.SKILL.md.OLD` (was 2011 lines) → Now split into basics, deployment, mcp-tools, mcp-debugging
- `nextflow-build.SKILL.md.OLD` (was 2006 lines) → Now split into debugging, production-practices, examples

## Reference material

- Config template: `skills://file/nextflow/references/config.md`
- Troubleshooting: `skills://file/nextflow/references/troubleshooting.md`
