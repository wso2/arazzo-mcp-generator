# arazzo-mcp-gen

`arazzo-mcp-gen` is a CLI tool that turns an [Arazzo specification](https://spec.openapis.org/arazzo/latest.html) and its referenced OpenAPI files into a fully Dockerized Python MCP (Model Context Protocol) server. Each Arazzo workflow becomes an MCP tool that any AI agent can call.

---

## Table of Contents

1. [What It Does](#what-it-does)
2. [Prerequisites](#prerequisites)
3. [Installation](#installation)
4. [Commands](#commands)
   - [validate](#validate)
   - [inspect](#inspect)
   - [visualize](#visualize)
   - [mcp-server generate](#mcp-server-generate)
5. [User Scenario: End-to-End Walkthrough](#user-scenario-end-to-end-walkthrough)
6. [Generated Artifacts](#generated-artifacts)
7. [Sample Arazzo File](#sample-arazzo-file)
8. [License](#license)

---

## What It Does

Given a folder containing:
- one Arazzo `.yaml` file (describes multi-step API workflows)
- referenced OpenAPI `.yaml` files (describe individual API operations)

…the CLI will:

| Step | What happens |
|------|-------------|
| Validate | Checks the Arazzo file for correctness (requires Spectral or uses built-in checks) |
| Inspect | Shows a human-readable summary of workflows and steps |
| Visualize | Renders a Mermaid flowchart of the workflow logic |
| Generate | Emits `mcp_server.py` + `Dockerfile`, then builds a Docker image |
| Run | `docker run` the image — any MCP client can connect |

---

## Prerequisites

| Tool | Why | Install |
|------|-----|---------|
| **Docker** | Build and run the generated image | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/) |
| **Node.js + npx** *(optional)* | Enables the Spectral validator for in-depth Arazzo checks | [nodejs.org](https://nodejs.org) |

---

## Installation

Download the latest version for your operating system from the [Releases](https://github.com/wso2/arazzo-mcp-generator/releases) page, or use the quick install commands below.

### macOS / Linux

```bash
# For macOS (Apple Silicon)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/arazzo-mcp-gen_Darwin_arm64.tar.gz -o arazzo-mcp-gen.tar.gz
tar -xzf arazzo-mcp-gen.tar.gz
sudo mv arazzo-mcp-gen-darwin-arm64 /usr/local/bin/arazzo-mcp-gen
rm arazzo-mcp-gen.tar.gz

# For macOS (Intel)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/arazzo-mcp-gen_Darwin_x86_64.tar.gz -o arazzo-mcp-gen.tar.gz
tar -xzf arazzo-mcp-gen.tar.gz
sudo mv arazzo-mcp-gen-darwin-amd64 /usr/local/bin/arazzo-mcp-gen
rm arazzo-mcp-gen.tar.gz

# For Linux (x86_64)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/arazzo-mcp-gen_Linux_x86_64.tar.gz -o arazzo-mcp-gen.tar.gz
tar -xzf arazzo-mcp-gen.tar.gz
sudo mv arazzo-mcp-gen-linux-amd64 /usr/local/bin/arazzo-mcp-gen
rm arazzo-mcp-gen.tar.gz

# For Linux (ARM64)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/arazzo-mcp-gen_Linux_arm64.tar.gz -o arazzo-mcp-gen.tar.gz
tar -xzf arazzo-mcp-gen.tar.gz
sudo mv arazzo-mcp-gen-linux-arm64 /usr/local/bin/arazzo-mcp-gen
rm arazzo-mcp-gen.tar.gz
```

### Windows

```powershell
# Download and extract (PowerShell)
Invoke-WebRequest -Uri https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/arazzo-mcp-gen_Windows_x86_64.zip -OutFile arazzo-mcp-gen.zip
Expand-Archive -Path arazzo-mcp-gen.zip -DestinationPath .
# Move arazzo-mcp-gen.exe to a directory in your PATH, or run it directly
```

Verify the installation:

```bash
arazzo-mcp-gen --version
```

---

<!-- ## Quick Start

If you don't have an Arazzo spec yet, let the CLI create a sample one:

```bash
arazzo-mcp-gen sample my-project
cd my-project
```

Then validate, inspect, and generate in three commands:

```bash
arazzo-mcp-gen validate -f .
arazzo-mcp-gen inspect  -f .
arazzo-mcp-gen mcp-server generate -f . -p 5000
```

Once Docker finishes building, run it:

```bash
docker run -p 5000:5000 <image-name-from-output>
```

--- -->

## Commands

### `validate`

Validates an Arazzo specification for correctness and completeness.

Uses **Spectral** (via `npx @stoplight/spectral-cli`) with the official `spectral:arazzo` ruleset as the primary validator when available. Falls back to the built-in Go validator when Node.js is not installed, showing install instructions.

```bash
arazzo-mcp-gen validate -f <file-or-folder>
```

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) | — |
| `--check-remote` | | Also probe remote source URLs for accessibility | `false` |
| `--strict` | | Treat warnings as errors (exits with code 1 on warnings) | `false` |

**Examples**

```bash
# Validate a folder (auto-detects the Arazzo file)
arazzo-mcp-gen validate -f ./my-arazzo-folder

# Validate a single file
arazzo-mcp-gen validate -f ./workflow.yaml

# Validate and also check that remote OpenAPI URLs are reachable
arazzo-mcp-gen validate -f ./my-arazzo-folder --check-remote

# Strict mode: fail if there are any warnings
arazzo-mcp-gen validate -f ./my-arazzo-folder --strict
```

**What it checks (Spectral ruleset)**
- Full JSON Schema validation against the Arazzo 1.0.x spec
- Unique `workflowId` and `stepId` values
- Step targets (`operationId`, `operationPath`, `workflowId`) are present and valid
- Parameter `name`, `in`, and `value` fields
- Success criteria condition syntax
- Unique `onSuccess` / `onFailure` action names
- Output expression syntax
- `dependsOn` cross-references

**Additional built-in checks (always run)**
- Local source file existence
- Remote URL accessibility (only with `--check-remote`)
- Multiple `$statusCode` criteria that are AND-ed together (a common mistake)

**Exit codes**

| Code | Meaning |
|------|---------|
| `0` | Passed (no errors) |
| `1` | Errors found, or warnings in `--strict` mode |

---

### `inspect`

Parses and prints a detailed, colour-coded overview of an Arazzo spec — without generating anything. Use this to understand a spec or debug step-flow routing before generating an MCP server.

```bash
arazzo-mcp-gen inspect -f <file-or-folder>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) |

**Examples**

```bash
# Inspect a folder (auto-detects the Arazzo file)
arazzo-mcp-gen inspect -f ./my-arazzo-folder

# Inspect a specific file
arazzo-mcp-gen inspect -f ./workflow.yaml
```

**Output includes**
- Spec metadata: title, version, Arazzo version
- All source descriptions with types and URLs
- For each workflow:
  - Input schema with types
  - Each step: operation target, parameter bindings, success criteria
  - `onSuccess` / `onFailure` routing with conditions (GOTO, END, RETRY)
  - Step outputs and their expressions
  - Workflow-level outputs

---

### `visualize`

Generates a Mermaid flowchart diagram of the Arazzo spec's workflow logic. By default opens the rendered diagram in your browser (no extra tools needed). Can also save to a file.

```bash
arazzo-mcp-gen visualize -f <file-or-folder> [-o <output-file>]
```

Alias: `viz`

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) |
| `--output` | `-o` | Output file path. `.md` → Mermaid in fenced code block; `.mmd` → raw Mermaid syntax |

**Examples**

```bash
# Open diagram in browser (default)
arazzo-mcp-gen visualize -f ./my-arazzo-folder

# Save to GitHub-renderable Markdown
arazzo-mcp-gen visualize -f ./workflow.yaml -o diagram.md

# Save raw Mermaid source
arazzo-mcp-gen visualize -f ./my-arazzo-folder -o flow.mmd

# Short alias
arazzo-mcp-gen viz -f ./my-arazzo-folder
```

**Diagram shows**
- Start and end nodes for each workflow
- Steps with operation targets
- `onSuccess` / `onFailure` branches labelled with conditions
- Implicit sequential flow and fallthrough paths (dashed arrows)
- Cross-workflow `goto` references

> Paste any `.mmd` file into [mermaid.live](https://mermaid.live) for a shareable interactive link.

---

### `mcp-server generate`

The main command. Reads your Arazzo + OpenAPI files, generates a Python MCP server, and builds a Docker image.

```bash
arazzo-mcp-gen mcp-server generate -f <file-or-folder> [flags]
```

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if folder; uses parent directory for referenced OpenAPI files if file) | — |
| `--port` | `-p` | Port the MCP server listens on inside the container and on your host | `5000` |
| `--output` | `-o` | Save generated artifacts (`mcp_server.py`, `Dockerfile`, `arazzo/` folder) to this path for inspection. If omitted a temp directory is used and cleaned up automatically | — |

**Examples**

```bash
# From a folder (auto-detects the Arazzo file)
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder

# From a single Arazzo file directly
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder/workflow.arazzo.yaml

# Custom port
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder -p 8080

# Inspect generated files after build
arazzo-mcp-gen mcp-server generate -f ./workflow.arazzo.yaml -p 8080 -o ./artifacts
```

**Input requirements**
- When `-f` is a folder: it must contain exactly one `.yaml`/`.yml` file with a top-level `arazzo:` key
- When `-f` is a file: point directly to the Arazzo file; the folder can contain multiple Arazzo files
- All OpenAPI files referenced in `sourceDescriptions[].url` must be in the same folder as the Arazzo file
- The Arazzo file must have `info.title`, `info.version`, and at least one workflow

**What it does**
1. Finds and validates the Arazzo file in the folder
2. Generates `mcp_server.py` — each workflow becomes a `@mcp.tool()` function with typed parameters
3. Generates a `Dockerfile` using `python:3.11-slim`
4. Runs `docker build` to produce a tagged image
5. Prints the `docker run` command to start the server

**Running the generated server**

```bash
docker run -p 5000:5000 <image-name>
```

If your workflow uses HTTPS endpoints with self-signed or otherwise invalid TLS certificates, run the image with the following environment variable to disable certificate verification:

```bash
docker run -e ARAZZO_DISABLE_TLS_VERIFY=1 -p 5000:5000 <image-name>
```

The MCP endpoint is available at `http://localhost:5000/mcp`.

---

## User Scenario: End-to-End Walkthrough

> **Scenario:** You have an OpenAPI spec for a pet store API and want to expose a "check if a pet exists, then create or update it" workflow as an MCP tool for an AI agent.

### Step 1 — Prepare your project folder

Create a folder containing your Arazzo specification and its referenced OpenAPI files:

1. Create a folder named `pet-project`.
2. Save your Arazzo file (e.g., `petstore_workflow.yaml`) inside it.
3. Ensure all OpenAPI `.yaml` files referenced in the Arazzo spec are also in this folder.

```text
pet-project/
├── petstore_workflow.yaml   ← Your Arazzo spec
└── petstore_openapi.yaml    ← Your OpenAPI spec
```

### Step 2 — Validate the spec

```bash
arazzo-mcp-gen validate -f .
```

**Expected output (Spectral available):**
```text
Validating: /path/to/pet-project/petstore_workflow.yaml
────────────────────────────────────────────────────────────
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Validation Result: PASSED
  ✓ All arazzo rules passed
  ─ Validated using Spectral (spectral:arazzo ruleset)
```

Fix any errors reported before continuing. Warnings are informational; use `--strict` to treat them as errors in CI.

### Step 3 — Inspect the spec

```bash
arazzo-mcp-gen inspect -f .
```

Review the printed summary to confirm:
- The correct source descriptions (your OpenAPI file/URL)
- Every step has an `operationId` that matches your OpenAPI spec
- Input schema, success criteria, and routing look correct

### Step 4 — Visualize the flow

```bash
arazzo-mcp-gen visualize -f .
```

Your browser opens with an interactive Mermaid flowchart. Check the branching logic visually — this is especially useful for multi-step workflows with `onSuccess` / `onFailure` routing.

To save it:

```bash
# As a Markdown file (renders on GitHub)
arazzo-mcp-gen visualize -f . -o flow.md
```

### Step 5 — Generate the MCP server

Make sure Docker is running, then:

```bash
arazzo-mcp-gen mcp-server generate -f . -p 5000 -o ./artifacts
```

**Expected output:**
```text
Validating input folder...
Found Arazzo spec: Pet Upsert Workflow (V3) with 1 workflow(s)
Generating MCP server code...
Building Docker image...
[+] Building 12.3s (10/10) FINISHED
╔════════════════════════════════════════════════════════════════════════╗
║ ✅ MCP Server image built successfully!                                ║
║                                                                        ║
║ Image:  pet-upsert-workflow-v3-mcp-server                              ║
║ Run:    docker run -p 5000:5000 pet-upsert-workflow-v3-mcp-server      ║
║ URL:    http://localhost:5000                                          ║
║                                                                        ║
║ If TLS verification must be disabled for self-signed HTTPS endpoints,  ║
║ run the image with: -e ARAZZO_DISABLE_TLS_VERIFY=1                     ║
║                                                                        ║
║ Build artifacts saved to: /path/to/pet-project/artifacts               ║
╚════════════════════════════════════════════════════════════════════════╝
```

### Step 6 — Run the server

Copy the `docker run` command from the output and run it:

```bash
docker run -p 5000:5000 pet-upsert-workflow-v3-mcp-server
```

### Step 7 — Connect an MCP client

The server is now live at `http://localhost:5000/mcp` in stateless HTTP mode. To connect it to an MCP client like **Claude Desktop**, you can use `supergateway` to bridge the HTTP endpoint:

```json
{
  "mcpServers": {
    "my-mcp-server": {
      "command": "npx",
      "args": [
        "-y",
        "supergateway",
        "--streamableHttp",
        "http://localhost:5000/mcp"
      ]
    }
  }
}
```

> **Note:** Replace `http://localhost:5000/mcp` with the endpoint shown in your terminal if you used a different port.

The AI agent can now call your Arazzo workflows as tools. The tool executes the full multi-step logic internally and returns the final result.

---

## Generated Artifacts

Inspect with `--output` / `-o ./artifacts`:

```text
artifacts/
├── mcp_server.py     ← FastMCP server; each workflow = @mcp.tool()
├── Dockerfile        ← python:3.11-slim image; EXPOSEs your port
└── arazzo/
    ├── petstore_workflow.yaml   ← copy of your Arazzo spec
    └── openapi.yaml             ← copy of referenced OpenAPI spec(s)
```

| File | What it is |
|------|------------|
| `mcp_server.py` | Python server using `fastmcp` and `arazzo-runner`. Workflow inputs become typed function parameters; docstrings come from workflow summaries/descriptions. |
| `Dockerfile` | Standard slim Python container. Installs dependencies, copies the `arazzo/` folder, and runs `mcp_server.py`. |
| `arazzo/` | All spec files the container needs to resolve `$ref` and `sourceDescriptions` at runtime. |

---

## Sample Arazzo File

To get started, create a folder for your project and save the following as `petstore_workflow.yaml` inside it. This is a ready-to-use Arazzo spec targeting the public [Petstore v3 API](https://petstore3.swagger.io) — it checks whether a pet exists by ID, updates its name if found, or creates it if not.

```yaml
arazzo: "1.0.0"
info:
  title: Pet Upsert Workflow (V3)
  summary: A sample workflow that conditionally creates or updates a pet using Petstore V3
  description: Workflow targeting Petstore V3 API. Takes an id and name - renames the pet if it exists, creates it if not.
  version: 1.0.0

sourceDescriptions:
  - name: petstoreApiV3
    url: https://petstore3.swagger.io/api/v3/openapi.json
    type: openapi

workflows:
  - workflowId: ensurePetExistsV3
    summary: Check if a pet exists by ID; update its name if found, create it if not.
    description: This workflow demonstrates conditional logic based on API responses. It first checks if a pet with the given ID exists. If it does, it updates the pet's name. If it doesn't, it creates a new pet with the provided ID and name.
    inputs:
      type: object
      properties:
        petId: { type: integer }
        newName: { type: string }

    steps:
      - stepId: checkStep
        description: Check if the pet exists and route accordingly.
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId

        successCriteria:
          - condition: $statusCode == 200

        # Branch based on which status code was returned
        onSuccess:
          - name: petFoundRouteToUpdate
            criteria:
              - condition: $statusCode == 200
            type: goto
            stepId: updateStep

        # Retry on true server errors
        onFailure:
          - name: retryOnServerError
            criteria:
              - condition: $statusCode >= 500
            type: retry
            retryAfter: 5

      - stepId: createStep
        description: Pet not found - create it with the given id and name.
        operationId: addPet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls:
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterCreation
            type: end

      - stepId: updateStep
        description: Pet found - rename it using a full PUT update.
        operationId: updatePet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls:
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterUpdate
            type: end
```

Once you have this file saved, follow the [End-to-End Walkthrough](#user-scenario-end-to-end-walkthrough) to validate, inspect, and generate an MCP server from it.

---

## License

Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).