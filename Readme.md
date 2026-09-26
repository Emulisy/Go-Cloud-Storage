# GoCloudStorage

A full-stack cloud storage application built with Go, MySQL, Redis, and Cloudflare R2. It supports content-addressable storage, resumable multipart uploads, integrity verification, and automated deployment to an Azure VM.

**[Live demo](https://emulisygocloud.southeastasia.cloudapp.azure.com)** · **[Architecture and API](architecture.md)**

## Features

- **Content-addressable storage (CAS)** — SHA-256 identifies stored content independently of filenames. Multiple user file records can reference one R2 object, avoiding duplicate storage.
- **Fast upload and deduplication** — when matching content already exists, the application creates a user reference without uploading the bytes again.
- **Resumable multipart uploads** — files are split into 5 MiB chunks. Redis tracks uploaded R2 parts so interrupted transfers can continue from the missing chunk.
- **End-to-end integrity checks** — completed multipart objects are read back from R2 and verified against the expected SHA-256 digest and file size before metadata is committed.
- **Reference-aware deletion** — removing one user's file preserves shared content until its final reference is deleted.
- **Authenticated file management** — bcrypt password hashing, signed JWT cookies, user-scoped CRUD operations, pagination, renaming, upload progress, and drag-and-drop file selection.
- **Production deployment** — Docker Compose runs Caddy, the Go application, MySQL, and Redis. Caddy provides automatic HTTPS, and GitHub Actions deploys pushes to `main` over SSH.

## Technology

| Area | Stack |
| --- | --- |
| Backend | Go, `net/http`, AWS SDK for Go v2 |
| Frontend | HTML, CSS, JavaScript |
| Metadata | MySQL 8.4 |
| Upload state | Redis 7 |
| Object storage | Cloudflare R2 |
| Infrastructure | Docker Compose, Caddy, Ubuntu, Azure VM |
| Delivery | GitHub Actions, SSH, rsync |

## Deploy to an Ubuntu Azure VM

### 1. Prepare Azure and external services

Create or configure:

- An Ubuntu Azure VM with at least 2 GiB RAM; 4 GiB is recommended when building the image on the VM.
- The DNS name `emulisygocloud.southeastasia.cloudapp.azure.com` pointing to the VM.
- Azure Network Security Group rules allowing TCP ports `22`, `80`, and `443`. Do not expose `8080`, `3306`, or `6379` publicly.
- A Cloudflare R2 bucket and API credentials.

### 2. Bootstrap the VM

```sh
git clone https://github.com/Emulisy/Go-Cloud-Storage.git
cd Go-Cloud-Storage
sudo bash deploy/setup-server.sh
```

The script installs Docker Engine and the Compose plugin, copies the project to `/opt/gocloudstorage`, creates a protected `.env`, and grants the SSH user Docker access. Log out and reconnect after it completes.

If Ubuntu already has the conflicting `docker-compose-v2` package, remove it and rerun setup:

```sh
sudo apt-get remove -y docker-compose-v2
sudo apt-get --fix-broken install -y
sudo bash deploy/setup-server.sh
```

### 3. Configure production secrets

Edit `/opt/gocloudstorage/.env`:

```dotenv
MYSQL_ROOT_PASSWORD=replace-with-a-strong-root-password
MYSQL_DATABASE=gocloudstorage
MYSQL_USER=gocloudstorage
MYSQL_PASSWORD=replace-with-a-strong-app-password

REDIS_DB=0
REDIS_PASSWORD=

JWT_SECRET=replace-with-at-least-32-random-bytes

R2_ACCESS_KEY_ID=replace-with-your-access-key-id
R2_SECRET_ACCESS_KEY=replace-with-your-secret-access-key
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=replace-with-your-bucket-name
R2_REGION=auto
```

Protect and validate the file:

```sh
chmod 600 /opt/gocloudstorage/.env
cd /opt/gocloudstorage
docker compose config --quiet
```

The R2 endpoint must be an HTTPS service URL without a bucket path. The supplied Redis container does not require a password, so `REDIS_PASSWORD` remains empty.

### 4. Configure GitHub Actions

Add these repository secrets under **Settings → Secrets and variables → Actions**:

| Secret | Value |
| --- | --- |
| `AZURE_VM_HOST` | `emulisygocloud.southeastasia.cloudapp.azure.com` |
| `AZURE_VM_USER` | VM SSH username |
| `AZURE_VM_SSH_PRIVATE_KEY` | Complete private deployment key |
| `AZURE_VM_SSH_KNOWN_HOSTS` | Verified SSH host-key entry for the VM |

The workflow in [`.github/workflows/deploy.yml`](.github/workflows/deploy.yml) runs on every push to `main` and can also be started manually. It:

1. Synchronizes the repository to `/opt/gocloudstorage` while preserving `.env`.
2. Validates the Compose and Caddy configuration.
3. Pulls service images and rebuilds the Go application.
4. Starts the stack and waits for service health checks.

### 5. Verify deployment

```sh
cd /opt/gocloudstorage
docker compose ps
docker compose logs --tail=100 caddy app
curl -I http://127.0.0.1 -H 'Host: emulisygocloud.southeastasia.cloudapp.azure.com'
```

Then open the [live application](https://emulisygocloud.southeastasia.cloudapp.azure.com).

## Run locally

### Prerequisites

- Go `1.26.4`, as declared in `go.mod`
- Docker with Docker Compose
- A Cloudflare R2 bucket and credentials

Create `.env` using the variables from the deployment section. For host-based development, optionally set:

```dotenv
REDIS_ADDR=127.0.0.1:6379
```

Start MySQL and Redis, then run the Go application:

```sh
docker compose up -d mysql redis
go mod download
go run .
```

Open [http://localhost:8080](http://localhost:8080). MySQL initializes new data volumes from [`doc/table.sql`](doc/table.sql).

To run the complete containerized stack instead, use `docker compose up -d --build`. Caddy publishes ports 80 and 443; the application remains private inside the Compose network.

## Documentation

- [Architecture, CAS data model, upload protocol, and API reference](architecture.md)
- [Database schema](doc/table.sql)
- [Deployment workflow](.github/workflows/deploy.yml)

## Project structure

```text
.
|-- auth/                  # JWT authentication and middleware
|-- cache/                 # Redis client and upload state
|-- db/                    # MySQL access and metadata operations
|-- deploy/                # Caddy configuration and VM setup
|-- doc/table.sql          # Database schema
|-- handler/               # HTTP handlers
|-- static/                # Browser interface
|-- storage/               # Cloudflare R2 integration
|-- architecture.md        # Detailed design and API reference
|-- compose.yaml           # Production service topology
|-- dockerfile             # Go application image
`-- main.go                # Application entry point and routes
```

## Current limitations

- Fast upload reuses content by hash and size; a hardened multi-tenant deployment should additionally verify authorization to reference matching content.
- MySQL and R2 changes are not atomic, so uncertain failures may require object reconciliation.
- Expired Redis sessions require cleanup of abandoned R2 multipart uploads.
