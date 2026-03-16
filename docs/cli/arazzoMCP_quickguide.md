# MCP Server Generate & Auto-Proxy — Complete Step-by-Step Guide

This guide walks you through **every step** of using the `ap mcp-server generate` command with the `--proxy` flag to automatically generate an MCP server from an Arazzo specification and register it as a proxy on the WSO2 API Platform gateway.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
   - [2.1 Docker & Docker Compose](#21-docker--docker-compose)
   - [2.2 Go (for building the CLI)](#22-go-for-building-the-cli)
   - [2.3 Make (optional, for build convenience)](#23-make-optional-for-build-convenience)
3. [Building the CLI](#3-building-the-cli)
4. [Starting the Gateway](#4-starting-the-gateway)
   - [4.1 Start the gateway services](#41-start-the-gateway-services)
   - [4.2 Verify the gateway is running](#42-verify-the-gateway-is-running)
   - [4.3 Understanding the gateway services](#43-understanding-the-gateway-services)
5. [Configuring the CLI to Connect to the Gateway](#5-configuring-the-cli-to-connect-to-the-gateway)
   - [5.1 Add a gateway](#51-add-a-gateway)
   - [5.2 Set the active gateway](#52-set-the-active-gateway)
   - [5.3 Using environment variables (alternative)](#53-using-environment-variables-alternative)
6. [Preparing the Arazzo Spec Folder](#6-preparing-the-arazzo-spec-folder)
   - [6.1 Folder structure requirements](#61-folder-structure-requirements)
   - [6.2 Arazzo file format](#62-arazzo-file-format)
   - [6.3 OpenAPI file requirements](#63-openapi-file-requirements)
   - [6.4 Example: Petstore Arazzo folder](#64-example-petstore-arazzo-folder)
7. [Running the Command](#7-running-the-command)
   - [7.1 Basic generate (no proxy)](#71-basic-generate-no-proxy)
   - [7.2 Generate with auto-proxy](#72-generate-with-auto-proxy)
   - [7.3 Generate with custom context path](#73-generate-with-custom-context-path)
   - [7.4 Generate with custom port](#74-generate-with-custom-port)
   - [7.5 Save build artifacts](#75-save-build-artifacts)
   - [7.6 All flags reference](#76-all-flags-reference)
8. [What Happens Under the Hood](#8-what-happens-under-the-hood)
   - [8.1 Step 1: Validate input folder](#81-step-1-validate-input-folder)
   - [8.2 Step 2: Parse the Arazzo specification](#82-step-2-parse-the-arazzo-specification)
   - [8.3 Step 3: Validate source descriptions](#83-step-3-validate-source-descriptions)
   - [8.4 Step 4: Generate Python MCP server code](#84-step-4-generate-python-mcp-server-code)
   - [8.5 Step 5: Generate Dockerfile](#85-step-5-generate-dockerfile)
   - [8.6 Step 6: Build the Docker image](#86-step-6-build-the-docker-image)
   - [8.7 Step 7: Auto-proxy flow (when --proxy is used)](#87-step-7-auto-proxy-flow-when---proxy-is-used)
9. [After the Command Completes](#9-after-the-command-completes)
   - [9.1 Running the MCP server permanently](#91-running-the-mcp-server-permanently)
   - [9.2 Accessing the MCP server through the gateway](#92-accessing-the-mcp-server-through-the-gateway)
   - [9.3 Managing gateway MCP proxies](#93-managing-gateway-mcp-proxies)
   - [9.4 Connecting to Claude Desktop](#94-connecting-to-claude-desktop)
10. [Troubleshooting](#10-troubleshooting)
11. [Full End-to-End Example](#11-full-end-to-end-example)

---

## 1. Overview

The `ap mcp-server generate` command performs the following:

1. **Reads** an Arazzo specification file and its referenced OpenAPI specs from a folder.
2. **Generates** a Python MCP (Model Context Protocol) server that exposes each Arazzo workflow as an MCP tool.
3. **Builds** a Docker image containing the generated server.
4. **(With `--proxy`)** Automatically registers the MCP server as a proxy on the active WSO2 API Platform gateway so that AI agents and LLMs can discover and invoke the tools through the gateway.

**The key benefit of `--proxy`**: You go from Arazzo spec → running gateway-proxied MCP server in a single command. No manual YAML editing or gateway configuration needed.

---

## 2. Prerequisites

### 2.1 Docker & Docker Compose

Docker is required for two things:
- **Building** the MCP server Docker image
- **Running** the WSO2 API Platform gateway (which runs as Docker containers)

**Install Docker Desktop:**

- **Windows**: Download from https://www.docker.com/products/docker-desktop/ and install. Ensure WSL 2 backend is enabled.
- **macOS**: Download from https://www.docker.com/products/docker-desktop/ and install.
- **Linux**: Follow the official guide at https://docs.docker.com/engine/install/

**Verify Docker is installed and running:**

```bash
docker --version
# Expected: Docker version 24.x or later

docker compose version
# Expected: Docker Compose version v2.x or later

# Test Docker is actually running:
docker ps
# Expected: A list (possibly empty) of running containers — NOT an error
```

> ⚠️ **Important**: Docker Desktop must be **running** (not just installed). On Windows, look for the Docker whale icon in your system tray. If Docker is not running, you will get errors like "Cannot connect to the Docker daemon".

### 2.2 Go (for building the CLI)

The CLI is written in Go. You need Go installed to build it from source.

**Install Go:**

- Download from https://go.dev/dl/
- Install version **1.25.1** or later

**Verify:**

```bash
go version
# Expected: go version go1.25.1 or later
```

### 2.3 Make (optional, for build convenience)

The project includes a Makefile for convenience. If you don't have `make`, you can use `go build` directly (shown below).

- **Windows**: Install via Chocolatey (`choco install make`) or use Git Bash (which often includes make).
- **macOS/Linux**: Usually pre-installed. If not: `sudo apt install make` (Linux) or `xcode-select --install` (macOS).

---

## 3. Building the CLI

The CLI binary is called `ap`. You need to build it from source before using it.

**Navigate to the CLI source directory:**

```bash
cd api-platform/cli/src
```

**Option A: Build using Make (recommended)**

```bash
make build-skip-tests
```

This creates the binary at `cli/src/build/ap` (or `cli/src/build/ap.exe` on Windows).

**Option B: Build using Go directly**

```bash
go build -o build/ap main.go
```

On Windows:

```powershell
go build -o build/ap.exe main.go
```

**Option C: Run without building (for quick testing)**

Instead of building first, you can run the CLI directly with `go run`:

```bash
go run main.go mcp-server generate --help
```

> 💡 **Tip**: For the rest of this guide, when we write `ap`, you can substitute with:
> - `./build/ap` (if you built with make)
> - `go run main.go` (to run without building)
> - Add `build/` to your PATH to use `ap` directly from anywhere

**Verify the CLI works:**

```bash
ap --help
```

You should see a list of available commands including `gateway` and `mcp-server`.

---

## 4. Starting the Gateway

The `--proxy` flag requires a **running and healthy WSO2 API Platform gateway**. The gateway runs as a set of Docker containers via Docker Compose.

### 4.1 Start the gateway services

**Navigate to the gateway directory:**

```bash
cd api-platform/gateway
```

**Start the core gateway services:**

```bash
docker compose up -d
```

This starts three containers:

| Container | Port | Purpose |
|---|---|---|
| `gateway-controller` | **9090** | The control plane REST API — stores API/MCP configurations, serves xDS config to the runtime |
| `gateway-runtime` | **8080** (HTTP), **8443** (HTTPS) | The data plane — Envoy proxy that routes traffic to your backend services |
| `sample-backend` | 5000 | A sample Petstore backend service (used for testing) |

**Wait for the services to be fully ready** (typically 10-15 seconds):

```bash
# Watch the containers until they're all healthy
docker compose ps
```

You should see all three containers with status `Up`.

### 4.2 Verify the gateway is running

**Check the gateway controller health endpoint:**

```bash
curl http://localhost:9090/health
```

**Expected response:**

```json
{"status":"healthy"}
```

If you get this response, the gateway controller is running and ready to accept configurations.

> ⚠️ If you get "Connection refused", wait a few more seconds and try again. The controller may still be starting up.

**Check the gateway runtime:**

```bash
curl http://localhost:8080
```

This might return a 404 (no routes configured yet), which is fine — it means the runtime is running.

### 4.3 Understanding the gateway services

**Gateway Controller (port 9090):**
- This is the **management API** for the gateway.
- The CLI communicates with this service to register APIs and MCP proxies.
- It stores configurations in a SQLite database.
- It serves xDS (Envoy discovery service) configuration to the gateway runtime.
- **Authentication**: Protected with basic auth. Default credentials are `admin` / `admin` (configured in `gateway/configs/config.toml`).

**Gateway Runtime (port 8080):**
- This is the **traffic proxy** — actual API/MCP requests flow through here.
- It runs Envoy as the core proxy engine.
- It automatically picks up configuration changes from the controller via xDS.
- When the `--proxy` flag registers an MCP server, clients will access it through this service at `http://localhost:8080/<context>/mcp`.

**Default authentication credentials** (from `gateway/configs/config.toml`):

```toml
[controller.auth.basic]
enabled = true

[[controller.auth.basic.users]]
username = "admin"
password = "admin"
roles = ["admin"]
```

> 💡 These are the credentials you'll use when configuring the CLI to connect to the gateway (next section).

---

## 5. Configuring the CLI to Connect to the Gateway

Before using `--proxy`, you must tell the CLI **where the gateway is** and **how to authenticate**.

### 5.1 Add a gateway

This command saves the gateway connection details to the CLI configuration file at `~/.wso2ap/config.yaml`.

**Interactive mode (prompts for credentials):**

```bash
ap gateway add --display-name dev --server http://localhost:9090 --auth basic
```

You will be prompted to enter:
- **Username**: `admin`
- **Password**: `admin`

**Non-interactive mode (pass credentials as flags):**

```bash
ap gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive --username admin --password admin
```

**With `go run` (if you haven't built the binary):**

```bash
cd api-platform/cli/src
go run main.go gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive --username admin --password admin
```

**What each flag means:**

| Flag | Value | Description |
|---|---|---|
| `--display-name` or `-n` | `dev` | A friendly name for this gateway configuration. You can name it anything (e.g., `local`, `dev`, `prod`). |
| `--server` or `-s` | `http://localhost:9090` | The URL of the gateway **controller** REST API. This is port 9090, NOT 8080. |
| `--auth` | `basic` | Authentication type. Options: `none`, `basic`, `bearer`. The default gateway config uses basic auth. |
| `--username` | `admin` | Basic auth username (only with `--auth basic`). |
| `--password` | `admin` | Basic auth password (only with `--auth basic`). |
| `--no-interactive` or `-y` | — | Skip interactive prompts; credentials must be provided via flags. |

> 💡 **Note**: If this is the **first gateway** you add, it is **automatically set as the active gateway**. No need to run `ap gateway use` separately.

### 5.2 Set the active gateway

If you have multiple gateways configured, set which one is active:

```bash
ap gateway use --display-name dev
```

The active gateway is the one that `--proxy` will deploy to.

**List all configured gateways:**

```bash
ap gateway list
```

### 5.3 Using environment variables (alternative)

Instead of storing credentials in the config file, you can use environment variables. The CLI checks these first:

| Environment Variable | Description |
|---|---|
| `WSO2AP_GW_USERNAME` | Basic auth username |
| `WSO2AP_GW_PASSWORD` | Basic auth password |
| `WSO2AP_GW_TOKEN` | Bearer token |

**Example (PowerShell):**

```powershell
$env:WSO2AP_GW_USERNAME = "admin"
$env:WSO2AP_GW_PASSWORD = "admin"
ap gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive
```

**Example (Bash):**

```bash
export WSO2AP_GW_USERNAME=admin
export WSO2AP_GW_PASSWORD=admin
ap gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive
```

---

## 6. Preparing the Arazzo Spec Folder

The `ap mcp-server generate` command requires a **folder** containing the Arazzo specification and all referenced OpenAPI files.

### 6.1 Folder structure requirements

```
my-arazzo-folder/
├── my-arazzo-spec.yaml      ← Exactly ONE Arazzo file (must have top-level "arazzo" key)
├── openapi-spec-1.yaml      ← OpenAPI spec referenced in sourceDescriptions
├── openapi-spec-2.yaml      ← (optional) Additional OpenAPI specs if referenced
└── ...                      ← Any other referenced spec files
```

**Rules:**
1. The folder must contain **exactly one** Arazzo specification file (`.yaml` or `.yml`).
2. The Arazzo file is identified by having a top-level `arazzo` key (not just any YAML file).
3. **All** OpenAPI files referenced in the Arazzo `sourceDescriptions` must be present in the folder.
4. The file paths in `sourceDescriptions[].url` are **relative to the folder** (e.g., `./openapi_v2.yaml`).

### 6.2 Arazzo file format

An Arazzo file defines **workflows** that orchestrate API operations. Here's the structure the command expects:

```yaml
# Required: Must be present — this is how the CLI identifies it as an Arazzo file
arazzo: 1.0.1

# Required: Must contain title and version
info:
  title: My API Workflows        # Used to derive Docker image name and gateway context
  version: 1.0.0
  description: Optional description of the workflows

# Required: References to OpenAPI specs
sourceDescriptions:
  - name: myApi                   # Reference name used in workflows
    url: ./my-openapi.yaml        # Relative path to the OpenAPI file in the same folder
    type: openapi                 # Must be "openapi"

# Required: At least one workflow
workflows:
  - workflowId: myWorkflow        # Unique ID — becomes the MCP tool name
    summary: What this workflow does
    inputs:                       # Parameters the MCP tool will accept
      type: object
      properties:
        param1: { type: string }
        param2: { type: integer }
    steps:
      - stepId: step1
        operationId: getItem      # References an operationId from the OpenAPI spec
        parameters:
          - name: itemId
            in: path
            value: $inputs.param1
        successCriteria:
          - condition: $statusCode == 200
```

**Key fields used by the command:**

| Field | How it's used |
|---|---|
| `info.title` | → Docker image name (e.g., "My API Workflows" → `my-api-workflows-mcp-server`) |
| `info.title` | → Default gateway context path (e.g., "My API Workflows" → `/my-api-workflows`) |
| `sourceDescriptions[].url` | → Validated that the referenced OpenAPI file exists in the folder |
| `workflows[].workflowId` | → Each workflow becomes an MCP tool with this name |
| `workflows[].summary` | → MCP tool description |
| `workflows[].inputs.properties` | → MCP tool input parameters |

### 6.3 OpenAPI file requirements

The OpenAPI files must:
- Be valid OpenAPI 3.0 or 3.1 specs
- Contain the `operationId` values referenced in the Arazzo workflow steps
- Have `servers[].url` pointing to the actual backend API (the generated MCP server uses this to make real API calls)

### 6.4 Example: Petstore Arazzo folder

The repository includes a working example at `example/`:

```
example/
├── multi_workflow_indep.yaml   ← Arazzo file (3 workflows: lookupPet, createNewPet, updatePetInfo)
└── openapi_v2.yaml             ← Petstore OpenAPI spec (getPetById, addPet, updatePet)
```

**Arazzo file** (`multi_workflow_indep.yaml`):

```yaml
arazzo: 1.0.1
info:
  title: Independent Pet Workflows
  version: 1.0.0
  description: Three independent workflows for pet lookup, creation, and update.

sourceDescriptions:
  - name: petApi
    url: ./openapi_v2.yaml
    type: openapi

workflows:
  - workflowId: lookupPet
    summary: Look up a pet by its ID and return its details.
    inputs:
      type: object
      properties:
        petId: { type: integer }
    steps:
      - stepId: fetchPet
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId
        successCriteria:
          - condition: $statusCode == 200
        outputs:
          pet: $response.body

  - workflowId: createNewPet
    summary: Create a new pet and then verify it was persisted.
    inputs:
      type: object
      properties:
        petId: { type: integer }
        petName: { type: string }
    steps:
      - stepId: addStep
        operationId: addPet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.petName
            photoUrls: ["https://example.com/pet.jpg"]
            status: available
        successCriteria:
          - condition: $statusCode == 200
      - stepId: verifyStep
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId
        successCriteria:
          - condition: $statusCode == 200

  - workflowId: updatePetInfo
    summary: Check that a pet exists, then update its name.
    inputs:
      type: object
      properties:
        petId: { type: integer }
        newName: { type: string }
    steps:
      - stepId: checkExists
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId
        successCriteria:
          - condition: $statusCode == 200
      - stepId: doUpdate
        operationId: updatePet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            photoUrls: ["https://example.com/pet.jpg"]
            status: available
        successCriteria:
          - condition: $statusCode == 200
```

**OpenAPI file** (`openapi_v2.yaml`) — standard Petstore API with three operations: `getPetById`, `addPet`, `updatePet`, pointing to `https://petstore.swagger.io/v2`.

---

## 7. Running the Command

### 7.1 Basic generate (no proxy)

This builds the Docker image only — does NOT register with the gateway:

```bash
ap mcp-server generate -d ./example
```

**What happens:**
1. Reads `example/multi_workflow_indep.yaml`
2. Validates `example/openapi_v2.yaml` exists
3. Generates `mcp_server.py` (Python MCP server with 3 tools: `lookup_pet`, `create_new_pet`, `update_pet_info`)
4. Generates a `Dockerfile`
5. Builds Docker image: `independent-pet-workflows-mcp-server`
6. Prints run command

**Output:**

```
Validating input folder...
Found Arazzo spec: Independent Pet Workflows with 3 workflow(s)
Generating MCP server code...
Building Docker image...
[Docker build output...]

┌──────────────────────────────────────────────────────────────────┐
│ ✅ MCP Server image built successfully!                        │
│                                                                │
│ Image:  independent-pet-workflows-mcp-server                   │
│ Run:    docker run -p 5000:5000 independent-pet-workflows-mcp-server │
│ URL:    http://localhost:5000                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 7.2 Generate with auto-proxy

This builds the image AND registers it with the active gateway:

```bash
ap mcp-server generate -d ./example --proxy
```

**What happens (in addition to the basic generate):**
1. Checks the active gateway is configured and healthy
2. Starts a temporary Docker container from the built image
3. Waits for the MCP server to be ready (up to 30 seconds)
4. Introspects the MCP server via JSON-RPC (tools, prompts, resources)
5. Generates gateway-compatible YAML configuration
6. Applies the YAML to the gateway controller (creates or updates the MCP proxy)
7. Stops and removes the temporary container
8. Prints the gateway MCP endpoint URL

**Output:**

```
Validating input folder...
Found Arazzo spec: Independent Pet Workflows with 3 workflow(s)
Generating MCP server code...
Building Docker image...
[Docker build output...]

┌──────────────────────────────────────────────────────────────────┐
│ ✅ MCP Server image built successfully!                        │
│ ...                                                            │
└──────────────────────────────────────────────────────────────────┘

Setting up gateway proxy...
Using default context: /independent-pet-workflows (override with --context)
Checking gateway connection...
Gateway is healthy ✓
Starting temporary container 'mcp-proxy-temp-1740000000000'...
Container started: a1b2c3d4e5f6
Waiting for MCP server to be ready...
MCP server is ready ✓
Introspecting MCP server...
Applying MCP proxy config to gateway...
Gateway: MCP proxy created successfully
Cleaning up temporary container...

┌──────────────────────────────────────────────────────────────────┐
│ ✅ MCP proxy configured on gateway!                            │
│                                                                │
│ Gateway MCP endpoint: http://localhost:9090/independent-pet-workflows/mcp │
│                                                                │
│ Important: The MCP server container must be running for the    │
│ proxy to work.                                                 │
│ Start it with: docker run -p 5000:5000 independent-pet-workflows-mcp-server │
└──────────────────────────────────────────────────────────────────┘
```

### 7.3 Generate with custom context path

Override the auto-derived gateway context path:

```bash
ap mcp-server generate -d ./example --proxy --context /petstore-mcp
```

**Without `--context`:** Context is auto-derived from the Arazzo title:
- "Independent Pet Workflows" → `/independent-pet-workflows`
- "My API" → `/my-api`
- "Payment Service v2" → `/payment-service-v2`

**With `--context`:** Your specified value is used as-is. The leading `/` is added automatically if missing.

### 7.4 Generate with custom port

Change the port the MCP server listens on:

```bash
ap mcp-server generate -d ./example -p 8080 --proxy
```

**Default port**: `5000`

> ⚠️ Make sure the port is not already in use. If port 5000 is occupied by another process (e.g., a previously running MCP server), you'll get a Docker port binding error.

### 7.5 Save build artifacts

Save the generated files (Dockerfile, mcp_server.py, spec copies) to a directory:

```bash
ap mcp-server generate -d ./example --output-dir ./my-output --proxy
```

This saves the build artifacts to `./my-output/` so you can inspect or manually edit them. Without `--output-dir`, a temporary directory is used and cleaned up automatically.

**Generated files saved:**

```
my-output/
├── Dockerfile          ← The generated Dockerfile
├── mcp_server.py       ← The generated Python MCP server code
└── arazzo/             ← Copy of all spec files from the input folder
    ├── multi_workflow_indep.yaml
    └── openapi_v2.yaml
```

### 7.6 All flags reference

| Flag | Short | Type | Default | Required | Description |
|---|---|---|---|---|---|
| `--folder` | `-d` | string | — | ✅ Yes | Path to folder containing the Arazzo and OpenAPI spec files |
| `--port` | `-p` | int | `5000` | No | Port the MCP server listens on inside the container and mapped to localhost |
| `--output-dir` | — | string | — | No | Directory to save generated build artifacts. If not set, temp dir is used and cleaned up. |
| `--proxy` | — | bool | `false` | No | Auto-proxy the MCP server through the active gateway after building the image |
| `--context` | — | string | — | No | Gateway context path (e.g., `/petstore`). Only used with `--proxy`. If not set, derived from Arazzo title. |

---

## 8. What Happens Under the Hood

Here's a detailed breakdown of every step the command performs internally.

### 8.1 Step 1: Validate input folder

- Resolves the `-d` folder path to an absolute path
- Verifies the folder exists and is a directory
- Scans all `.yaml` and `.yml` files in the folder
- Identifies the Arazzo file by checking for a top-level `arazzo` key
- **Error** if no Arazzo file found: `no Arazzo specification file found in folder`
- **Error** if multiple Arazzo files found: `multiple Arazzo specification files found in folder`

### 8.2 Step 2: Parse the Arazzo specification

- Reads and parses the YAML file into a structured representation
- Validates required fields:
  - `arazzo` key must be present (version string like `1.0.1`)
  - `info.title` must be non-empty
  - At least one workflow must be defined
- Extracts `sourceDescriptions` (references to OpenAPI files)
- Extracts all `workflows` with their inputs, steps, and outputs

### 8.3 Step 3: Validate source descriptions

- For each `sourceDescriptions` entry, checks that the referenced file exists in the folder
- Resolves relative paths (e.g., `./openapi_v2.yaml`) against the folder path
- **Error** if any referenced file is missing: `referenced source file not found`

### 8.4 Step 4: Generate Python MCP server code

Generates a Python file (`mcp_server.py`) that:

- Uses **fastmcp** (`from fastmcp import FastMCP`) as the MCP framework
- Uses **arazzo-runner** to execute workflows against real APIs
- Creates one `@mcp.tool()` function per workflow:
  - `workflowId: lookupPet` → Python function `lookup_pet(pet_id: int)`
  - `workflowId: createNewPet` → Python function `create_new_pet(pet_id: int, pet_name: str)`
  - `workflowId: updatePetInfo` → Python function `update_pet_info(pet_id: int, new_name: str)`
- Maps Arazzo input types to Python types: `integer` → `int`, `string` → `str`, `boolean` → `bool`, `number` → `float`
- Converts camelCase names to snake_case for Python convention
- Configures the server to run on `http://0.0.0.0:<port>` in stateless HTTP mode

### 8.5 Step 5: Generate Dockerfile

Creates a Dockerfile:

```dockerfile
FROM python:3.11-slim
WORKDIR /app
RUN pip install --no-cache-dir fastmcp arazzo-runner
COPY arazzo/ ./arazzo/
COPY mcp_server.py .
EXPOSE 5000
CMD ["python", "mcp_server.py"]
```

### 8.6 Step 6: Build the Docker image

- Copies spec files and generated code into a build context directory
- If `--output-dir` is set: uses that directory (files persist)
- If `--output-dir` is not set: uses a temp directory under `~/.wso2ap/.tmp/` (cleaned up after build)
- Derives the image name from the Arazzo title:
  - "Independent Pet Workflows" → `independent-pet-workflows-mcp-server`
  - Lowercases, replaces non-alphanumeric chars with hyphens, appends `-mcp-server`
- Runs `docker build -t <image-name> .`
- Prints the success summary with run command and URL

### 8.7 Step 7: Auto-proxy flow (when --proxy is used)

This is the Phase 4 auto-proxy logic. Here's every sub-step:

#### 8.7.1 Determine context path

- If `--context` was provided: use the user's value (e.g., `/petstore-mcp`)
- If `--context` was NOT provided: auto-derive from `info.title`:
  - "Independent Pet Workflows" → `/independent-pet-workflows`
  - Prints: `Using default context: /independent-pet-workflows (override with --context)`

#### 8.7.2 Check gateway prerequisites

- Creates a gateway client using `NewClientForActive()`:
  - Reads `~/.wso2ap/config.yaml` to find the active gateway
  - Loads the server URL, auth type, and credentials
- Sends `GET /health` to the gateway controller (port 9090)
- **Error** if no active gateway: `cannot connect to gateway — Please configure a gateway first`
- **Error** if health check fails: `gateway is not healthy — Please ensure the gateway is running and accessible`

#### 8.7.2b Detect the gateway Docker network

- Inspects running gateway-controller containers to find the Docker network they are on
- Looks for a network name ending with `gateway-network` (e.g., `gateway_gateway-network`)
- This is critical: the MCP server container must join the **same Docker network** as the gateway so the gateway runtime can route traffic to it
- **Error** if no matching network found: `could not find the gateway Docker network`

#### 8.7.3 Start a temporary Docker container

- Generates a unique container name: `mcp-proxy-temp-<unix-timestamp-millis>`
- Runs: `docker run -d -p <port>:<port> --network <gateway-network> --name <container-name> <image-name>`
- The `--network` flag joins the container to the gateway Docker network so the gateway runtime can reach it
- The `-d` flag starts it in detached (background) mode
- **Error** if port is in use: provides helpful message to stop conflicting process or use a different port
- Registers a **deferred cleanup**: when the function returns (success or error), it will `docker stop` and `docker rm` the container

#### 8.7.4 Wait for the MCP server to be ready

- Polls `http://localhost:<port>/mcp` with HTTP POST requests
- Timeout: **30 seconds**, retry interval: **1 second**, HTTP timeout per request: **2 seconds**
- Any response (even 4xx/5xx) counts as "server is up"
- **Error** if server doesn't respond within 30 seconds: `MCP server did not become ready — Check container logs: docker logs <container-name>`

#### 8.7.5 Introspect the MCP server

Uses the existing `internal/mcp/generator.go` `Generate()` function which:

1. Sends JSON-RPC `initialize` request to `http://localhost:<port>/mcp`:
   ```json
   {
     "jsonrpc": "2.0",
     "id": 1,
     "method": "initialize",
     "params": {
       "protocolVersion": "2025-06-18",
       "capabilities": {},
       "clientInfo": {"name": "ap-cli", "version": "1.0.0"}
     }
   }
   ```
2. Sends `notifications/initialized` notification
3. Sends `tools/list` — retrieves all MCP tools (name, description, input schema)
4. Sends `prompts/list` — retrieves all MCP prompts
5. Sends `resources/list` — retrieves all MCP resources
6. Writes a gateway-compatible YAML config file to the output directory

The generated YAML looks like:

```yaml
apiVersion: gateway.apiplatform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: Generated-MCP-v1.0
spec:
  displayName: Generated-MCP
  version: v1.0
  context: /generated
  specVersion: "2025-06-18"
  upstream:
    url: http://independent-pet-workflows-mcp-server:5000
  tools:
    - name: lookup_pet
      description: Look up a pet by its ID and return its details.
      inputSchema: { ... }
    - name: create_new_pet
      description: Create a new pet and then verify it was persisted.
      inputSchema: { ... }
    - name: update_pet_info
      description: Check that a pet exists, then update its name.
      inputSchema: { ... }
  resources: []
  prompts: []
```

> **Note:** The upstream URL uses the **Docker container name** (the image name), not `localhost`. This is because the gateway runtime runs inside a Docker container and resolves the MCP server address via Docker DNS on the shared network.

#### 8.7.6 Patch the context path and upstream URL

- If `--context` was provided or a default was derived, replaces `context: /generated` in the YAML with the correct context (e.g., `context: /independent-pet-workflows`)
- Patches the `upstream.url` from `http://localhost:<port>` to `http://<image-name>:<port>` — this ensures the gateway runtime (which runs in Docker) can reach the MCP server via Docker DNS on the shared network
- Uses line-based replacement to preserve YAML formatting and indentation

#### 8.7.7 Apply to gateway

- Gets the MCP resource handler (`kind: Mcp`) which knows the gateway API endpoints
- Extracts `metadata.name` from the generated YAML (e.g., `Generated-MCP-v1.0`)
- Checks if this MCP proxy already exists on the gateway:
  - `GET /mcp-proxies/Generated-MCP-v1.0` → 200 means exists, 404 means new
- If **new**: `POST /mcp-proxies` with the YAML body (Content-Type: `application/x-yaml`)
- If **exists**: `PUT /mcp-proxies/Generated-MCP-v1.0` with the YAML body (updates it)
- Prints the gateway response message

#### 8.7.8 Cleanup

- Stops the temporary container: `docker stop mcp-proxy-temp-<timestamp>`
- Removes the temporary container: `docker rm mcp-proxy-temp-<timestamp>`
- If a temporary output directory was used (no `--output-dir`), it's also deleted

#### 8.7.9 Print summary

Displays a boxed summary with:
- The gateway MCP endpoint URL
- A reminder to start the MCP server container for the proxy to work
- The exact `docker run` command to start the container

---

## 9. After the Command Completes

### 9.1 Running the MCP server permanently

The `--proxy` flag only registers the MCP configuration with the gateway — it does **not** leave the MCP server running. You need to start the container yourself **on the gateway Docker network**:

```bash
docker run -d \
  --name independent-pet-workflows-mcp-server \
  --network gateway_gateway-network \
  -p 5000:5000 \
  independent-pet-workflows-mcp-server
```

Flags:
- `-d` — Run in detached (background) mode
- `--name` — Must match the image name (the gateway upstream URL references this container name)
- `--network gateway_gateway-network` — Join the gateway's Docker network so the gateway runtime can reach it
- `-p 5000:5000` — Map container port to host port (for direct access and testing)

> ⚠️ **Critical**: The `--name` and `--network` flags are required. Without them, the gateway runtime cannot route traffic to the MCP server. The exact network name is printed by the `--proxy` command.

**Verify it's running:**

```bash
# Check the container is up
docker ps

# Test direct access
curl -X POST http://localhost:5000/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'

# Test through the gateway proxy
curl -X POST http://localhost:8080/independent-pet-workflows/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

### 9.2 Accessing the MCP server through the gateway

Once the MCP server container is running and the proxy is registered:

**Direct access (bypassing gateway):**
```
http://localhost:5000/mcp
```

**Through the gateway (proxied):**
```
http://localhost:8080/<context>/mcp
```

For the Petstore example with default context:
```
http://localhost:8080/independent-pet-workflows/mcp
```

With a custom context `--context /petstore-mcp`:
```
http://localhost:8080/petstore-mcp/mcp
```

**Configure your AI agent / LLM / MCP client to use the gateway URL** as the MCP server endpoint.

### 9.3 Managing gateway MCP proxies

The CLI provides built-in commands to list, inspect, and delete MCP proxies on the gateway:

```bash
# List all MCP proxies registered on the gateway
ap gateway mcp list

# Get details of a specific MCP proxy
ap gateway mcp get --id Generated-MCP-v1.0

# Delete an MCP proxy from the gateway
ap gateway mcp delete --id Generated-MCP-v1.0
```

> 💡 **Tip**: When re-running `--proxy`, the command automatically **updates** the existing proxy if one with the same ID already exists (`Generated-MCP-v1.0`). You don't need to delete it first.

### 9.4 Connecting to Claude Desktop

To use the MCP server with Claude Desktop, you need a bridge that converts the HTTP Streamable transport to stdio (which Claude Desktop expects). Use the `supergateway` npm package:

**Edit your Claude Desktop config** (`%APPDATA%\Claude\claude_desktop_config.json` on Windows, `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "toolshop-server": {
      "command": "npx",
      "args": [
        "-y",
        "supergateway",
        "--streamableHttp",
        "http://localhost:8080/apim-toolshop-brand-workflows/mcp"
      ]
    }
  }
}
```

**How it works:**
- Claude Desktop launches `supergateway` as a stdio child process
- `supergateway` connects to the MCP server's Streamable HTTP endpoint
- It bridges stdio ↔ HTTP, translating JSON-RPC messages between the two

**Important notes:**
- You can connect directly (`http://localhost:7000/mcp`) or through the gateway (`http://localhost:8080/<context>/mcp`)
- The MCP server container must be running before you start Claude Desktop
- Restart Claude Desktop after editing the config file
- The `npx -y` ensures `supergateway` is auto-installed on first run

---

## 10. Troubleshooting

### "Docker is not available or not running"

- Make sure Docker Desktop is running (check system tray for the whale icon)
- Run `docker ps` to verify connectivity
- On Linux, make sure your user is in the `docker` group: `sudo usermod -aG docker $USER`

### "cannot connect to gateway" / "no active gateway configured"

- Did you add a gateway? Run: `ap gateway add --display-name dev --server http://localhost:9090 --auth basic`
- Did you set it as active? Run: `ap gateway use --display-name dev`
- Check your config: `cat ~/.wso2ap/config.yaml`

### "gateway is not healthy"

- Is the gateway running? Run: `docker compose ps` in the `gateway/` directory
- Start it: `cd gateway && docker compose up -d`
- Check health manually: `curl http://localhost:9090/health`
- Check controller logs: `docker compose logs gateway-controller`

### "failed to start container" / port already in use

- Another process is using the port. Find it:
  - **Windows**: `netstat -aon | findstr :5000`
  - **Linux/macOS**: `lsof -i :5000`
- Stop the conflicting process, or use a different port: `ap mcp-server generate -d ./folder -p 8080 --proxy`

### "MCP server did not become ready in 30 seconds"

- The Python server might have crashed on startup. Check the container logs:
  ```bash
  docker logs mcp-proxy-temp-<timestamp>
  ```
  (The container name is printed in the output)
- Possible causes:
  - Missing Python dependencies in the Dockerfile
  - Invalid Arazzo spec causing code generation errors
  - Port conflicts inside the container

### "no Arazzo specification file found in folder"

- Make sure your YAML file has the top-level `arazzo` key:
  ```yaml
  arazzo: 1.0.1    ← This line MUST be present
  info:
    title: ...
  ```
- Make sure the file extension is `.yaml` or `.yml`

### "multiple Arazzo specification files found in folder"

- The folder must contain **exactly one** Arazzo file
- Move extra Arazzo files out of the folder

### "referenced source file not found"

- Check that all files in `sourceDescriptions[].url` exist in the folder
- Paths are relative to the folder (e.g., `./openapi_v2.yaml`)

### Authentication errors (401/403)

- Verify credentials: default is `admin` / `admin`
- Try re-adding the gateway: `ap gateway add --display-name dev --server http://localhost:9090 --auth basic`
- Check if environment variables are overriding: `echo $WSO2AP_GW_USERNAME`

---

## 11. Full End-to-End Example

Here's the complete flow from scratch, assuming a fresh system with Docker and Go installed.

### Step 1: Clone or navigate to the repository

```bash
cd api-platform
```

### Step 2: Build the CLI

```bash
cd api-platform/cli/src
make build-skip-tests
# Or: go build -o build/ap.exe main.go   (Windows)
# Or: go build -o build/ap main.go       (Linux/macOS)
```

### Step 3: Start the gateway

Open a **new terminal**:

```bash
cd api-platform/gateway
docker compose up -d
```

Wait 10-15 seconds, then verify:

```bash
curl http://localhost:9090/health
# → {"status":"healthy"}
```

### Step 4: Configure the CLI

Back in the CLI terminal:

```bash
# From cli/src directory:
./build/ap gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive --username admin --password admin
```

Or with `go run`:

```bash
go run main.go gateway add --display-name dev --server http://localhost:9090 --auth basic --no-interactive --username admin --password admin
```

### Step 5: Prepare the Arazzo folder

The repository includes a ready-to-use example at `../../example/` (relative to `cli/src`).

If you want to use it:

```
example/
├── multi_workflow_indep.yaml   ← Arazzo spec (3 pet workflows)
└── openapi_v2.yaml             ← Petstore OpenAPI spec
```

### Step 6: Generate and auto-proxy

```bash
# From cli/src directory:
./build/ap mcp-server generate -d ../../example --proxy
```

Or with `go run`:

```bash
go run main.go mcp-server generate -d ../../example --proxy
```

**Expected output:**

```
Validating input folder...
Found Arazzo spec: Independent Pet Workflows with 3 workflow(s)
Generating MCP server code...
Building Docker image...
[Docker build layers...]

┌──────────────────────────────────────────────────────────────────────────────────┐
│ ✅ MCP Server image built successfully!                                        │
│ ...                                                                            │
└──────────────────────────────────────────────────────────────────────────────────┘

Setting up gateway proxy...
Using default context: /independent-pet-workflows (override with --context)
Checking gateway connection...
Gateway is healthy ✓
Found gateway network: gateway_gateway-network
Starting temporary container 'mcp-proxy-temp-1740000000000'...
Container started: a1b2c3d4e5f6
Waiting for MCP server to be ready...
MCP server is ready ✓
Introspecting MCP server...
Applying MCP proxy config to gateway...
MCP proxy created successfully
Cleaning up temporary container...

┌──────────────────────────────────────────────────────────────────────────────────┐
│ ✅ MCP proxy configured on gateway!                                            │
│                                                                                │
│ Gateway MCP endpoint: http://localhost:8080/independent-pet-workflows/mcp      │
│                                                                                │
│ Important: The MCP server container must be running on the gateway             │
│ network for the proxy to work.                                                 │
│ Start it with:                                                                 │
│   docker run -d --name independent-pet-workflows-mcp-server                    │
│     --network gateway_gateway-network -p 5000:5000                             │
│     independent-pet-workflows-mcp-server                                       │
│                                                                                │
│ To remove this proxy from the gateway:                                         │
│   ap gateway mcp delete --id Generated-MCP-v1.0                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### Step 7: Start the MCP server container

The container **must** be started on the gateway Docker network with a matching container name:

```bash
docker run -d \
  --name independent-pet-workflows-mcp-server \
  --network gateway_gateway-network \
  -p 5000:5000 \
  independent-pet-workflows-mcp-server
```

### Step 8: Test the gateway proxy

```bash
curl -X POST http://localhost:8080/independent-pet-workflows/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
  }'
```

You should see a JSON-RPC response listing the three MCP tools: `lookup_pet`, `create_new_pet`, and `update_pet_info`.

### Step 9: Manage gateway MCP proxies

```bash
# List all registered MCP proxies
ap gateway mcp list

# Inspect a specific proxy
ap gateway mcp get --id Generated-MCP-v1.0

# Delete a proxy (when you no longer need it)
ap gateway mcp delete --id Generated-MCP-v1.0
```

### Step 10: Connect Claude Desktop

Edit your Claude Desktop config file and add the MCP server:

**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "my-mcp-server": {
      "command": "npx",
      "args": [
        "-y",
        "supergateway",
        "--streamableHttp",
        "http://localhost:8080/independent-pet-workflows/mcp"
      ]
    }
  }
}
```

Restart Claude Desktop. It will launch `supergateway` which bridges stdio ↔ Streamable HTTP, allowing Claude to discover and invoke your MCP tools.

---

