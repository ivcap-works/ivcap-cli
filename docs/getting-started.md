# Getting Started

This guide walks you through installing the IVCAP CLI, configuring a deployment
context, and verifying your setup.

---

## 1. Install

### macOS – Homebrew (recommended)

```bash
brew tap ivcap-works/ivcap
brew install ivcap
```

### macOS / Linux – Download a pre-built binary

1. Visit the [Releases page](https://github.com/ivcap-works/ivcap-cli/releases)
   and download the archive for your platform (e.g. `ivcap_darwin_arm64.tar.gz`).
2. Extract and move the binary onto your `$PATH`:

```bash
tar xzf ivcap_*.tar.gz
sudo mv ivcap /usr/local/bin/
```

### Windows

Download the `.zip` for `windows_amd64` from the
[Releases page](https://github.com/ivcap-works/ivcap-cli/releases), extract
`ivcap.exe`, and add its directory to your `PATH`.

### Build from source

Requires [Go](https://go.dev/) ≥ 1.21.

```bash
git clone https://github.com/ivcap-works/ivcap-cli.git
cd ivcap-cli
make build          # produces ./ivcap
make install        # installs to $GOPATH/bin
```

### Verify the installation

```bash
ivcap --version
```

---

## 2. Configure a deployment context

A _context_ stores the URL of an IVCAP deployment and your credentials so you
don't have to repeat them on every command.

### Create a context

```bash
ivcap context create <name> <deployment-url>
```

Example:
```bash
ivcap context create myivcap https://develop.ivcap.net
```

!!! tip
    You can have multiple contexts (e.g. `develop`, `staging`, `prod`) and
    switch between them with `ivcap context set <name>`.

### List and switch contexts

```bash
ivcap context list          # show all configured contexts
ivcap context set myivcap   # make 'myivcap' the active context
ivcap context get           # show details of the active context
```

---

## 3. Log in

Once a context is active, authenticate to obtain an access token:

```bash
ivcap context login
```

This opens a browser window for your identity provider's login flow. After
successful authentication the token is stored locally and refreshed
automatically.

### Headless / non-interactive login (CI, agents)

If you already have a token (e.g. from a CI secret), skip the browser flow:

```bash
export IVCAP_ACCESS_TOKEN="<your-token>"
```

or pass it inline:

```bash
ivcap --access-token "<your-token>" service list
```

---

## 4. Verify your connection

```bash
ivcap service list --limit 5 --output json
```

A successful response returns a JSON array of available services. If you see
an authentication error, re-run `ivcap context login`.

---

## 5. Submit your first job

```bash
# Find a service to run
ivcap service list --limit 10

# Inspect its parameters
ivcap service get urn:ivcap:service:<uuid>

# Create a job (params can be JSON inline or from a file)
ivcap job create urn:ivcap:service:<uuid> \
  --parameter key=value \
  --watch        # stream events until the job finishes

# Retrieve the job result
ivcap job get urn:ivcap:job:<uuid>
```

---

## Next steps

- Browse the [Core concepts & commands](index.md#core-concepts) for a full list of commands.
- Set up the [MCP server](ivcap_mcp.md) to let AI agents interact with IVCAP.
- Read the [agent skills](ivcap_skills.md) for best-practice workflow patterns.
