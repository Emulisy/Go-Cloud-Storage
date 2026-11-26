# <img src="static/assets/favicon.svg" width="34" height="34" alt=""> GoCloudStorage

GoCloudStorage is a personal file storage app built with Go. Upload, download, rename, and delete files in a browser, pause and resume large transfers, and reuse matching files without uploading them again. MySQL stores account and file metadata, Redis tracks multipart uploads, and MinIO stores file contents.

[Azure deployment](https://emulisygocloud.southeastasia.cloudapp.azure.com) · [Run locally with MinIO](#run-locally-with-minio) · [Deploy to Azure](#deploy-to-azure) · [Architecture and API](architecture.md)

## Features

- **Personal workspace.** Manage files and account details from a browser. Passwords are hashed with bcrypt, and signed JWT cookies authenticate requests.
- **Resumable transfers.** The browser uploads new files over 5 MiB in 5 MiB parts, with a smaller final part when needed. Redis records completed parts so you can reselect the same file and resume. Each successful part extends the session for 24 hours.
- **Deduplication by content.** The app checks the SHA-256 digest and size to reuse matching content. MySQL keeps filenames and ownership separate from the shared object, so deleting a file preserves the object while another reference exists.
- **Multipart integrity verification.** The Go server streams the completed object from storage and verifies its SHA-256 digest and size before saving its metadata.

## Run locally with MinIO

You need Docker with Compose and either an existing MinIO server or an AIStor license for the bundled service. For a fresh checkout, create `.env` from the [configuration template](.env.example).

```sh
git clone https://github.com/Emulisy/Go-Cloud-Storage.git
cd Go-Cloud-Storage
cp .env.example .env
```

Set these values in `.env`.

| Settings | What to enter |
| --- | --- |
| `MYSQL_ROOT_PASSWORD`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD` | Set the root and app passwords and use the database and app username from the template or choose your own. |
| `JWT_SECRET` | A random secret of at least 32 bytes. |
| `MINIO_ENDPOINT`, `MINIO_BUCKET` | Use `http://minio:9000` for the bundled service. For an existing server, enter an endpoint reachable from the app container. Enter the bucket name separately. |
| `MINIO_ACCESS_KEY_ID`, `MINIO_SECRET_ACCESS_KEY`, `MINIO_REGION` | Use credentials scoped to the app bucket and set the region if it differs from `us-east-1`. |

Keep `REDIS_PASSWORD` empty for the bundled Redis service. If you have an existing MinIO bucket and credentials, skip to [Start the app](#start-the-app).

### Bundled MinIO service

1. Place your AIStor license at `$MINIO_CONFIG_DIR/minio.license`. The Compose default directory is `/etc/gocloudstorage/minio`. Set `MINIO_ROOT_USER` and `MINIO_ROOT_PASSWORD` in `.env`. [MinIO AIStor container setup](https://docs.min.io/aistor/installation/container/install/)
2. Run `docker compose --profile minio up -d minio`. Create the bucket named by `MINIO_BUCKET` and provision app credentials that can read, write, delete, and manage multipart uploads in that bucket. You can run `mc` inside the MinIO container using `docker compose --profile minio exec minio mc` and connect it to `http://127.0.0.1:9000`. [MinIO access control](https://docs.min.io/aistor/administration/iam/access/)
3. Set `MINIO_ACCESS_KEY_ID` and `MINIO_SECRET_ACCESS_KEY` in `.env` to the app credentials.

### Start the app

Start MySQL and Redis, then build and run the app container with port 8080 published on localhost.

```sh
docker compose up -d --wait mysql redis
docker compose run --rm --build --no-deps --publish 127.0.0.1:8080:8080 app
```

Open [http://localhost:8080](http://localhost:8080), create an account, and upload a file. Upload the same file again to try deduplication. To try resuming, pause an upload over 5 MiB, then select **Resume upload**.

> [!NOTE]
> MySQL loads [the schema](doc/table.sql) when it initializes a new volume. For a volume created before the file-list indexes were added, run [the migration](doc/migrations/001_file_list_indexes.sql) from the repository root in Bash. The migration can be rerun.

```bash
docker compose exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql --user=root --database="$MYSQL_DATABASE"' \
  < doc/migrations/001_file_list_indexes.sql
```

> [!IMPORTANT]
> The app uses `s3://<object-key>` addresses for files in the configured MinIO bucket. For a fresh MinIO installation, use a new database volume. To carry over existing remote files, copy them to MinIO under the same keys, verify their size and SHA-256 digest, and update their stored addresses before starting the app.

## Architecture

The browser uses HTML, CSS, and native JavaScript modules served by Go's `net/http` server. The server handles uploads and downloads and connects to object storage through the AWS SDK for Go v2.

| Component | Responsibility |
| --- | --- |
| [`static/`](static/) | Browser pages, view rendering, controllers, and API requests. |
| [`main.go`](main.go), [`handler/`](handler/), [`auth/`](auth/) | HTTP routes, account and file operations, and session authentication. |
| [`db/`](db/), [`cache/`](cache/) | MySQL account and file metadata, plus Redis upload sessions and completed parts. |
| [`storage/`](storage/) | MinIO object operations through the S3 API. |
| [`compose.yaml`](compose.yaml), [`deploy/`](deploy/) | Services, the Caddy reverse proxy, and Ubuntu VM setup. |

See the [architecture and HTTP API reference](architecture.md) for the data model, upload sequence, endpoint details, and deployment considerations.

## Deploy to Azure

The included deployment uses Docker Compose on an Ubuntu Azure VM. Caddy terminates HTTPS and forwards requests to the Go container. MySQL and Redis publish ports on VM loopback for diagnostics.

1. Point a DNS hostname at the VM and allow inbound HTTP and HTTPS on ports 80 and 443 and SSH from your deployment runner. For a new hostname, update the site address in [`deploy/Caddyfile`](deploy/Caddyfile) and `APP_URL` in the [deployment workflow](.github/workflows/deploy.yml).
2. Clone the repository on the VM and run `sudo bash deploy/setup-server.sh` from that checkout. The script installs Docker and Compose, copies the checkout to `/opt/gocloudstorage`, and creates a protected `.env` there. Reconnect over SSH after setup so Docker group membership takes effect.
3. Fill in `/opt/gocloudstorage/.env` using [`.env.example`](.env.example). If you're using the bundled MinIO service, [start MinIO and provision its bucket](#bundled-minio-service) on the VM, then set `COMPOSE_PROFILES=minio` in `.env`. From `/opt/gocloudstorage`, run `docker compose config --quiet` to validate the configuration.
4. Configure these GitHub Actions secrets for the `production` environment, then run **Deploy to Azure VM** manually or push to `main`.

   | Secret | Value |
   | --- | --- |
   | `AZURE_VM_HOST` | VM hostname used for SSH. |
   | `AZURE_VM_USER` | VM user with access to Docker and `/opt/gocloudstorage`. |
   | `AZURE_VM_SSH_PRIVATE_KEY` | Private key for the deployment user. |
   | `AZURE_VM_SSH_KNOWN_HOSTS` | Verified SSH host-key entry for the VM. |

The workflow syncs the source while preserving the VM's `.env`, validates Caddy, rebuilds the app, waits for the services to become healthy, and applies the file-list index migration. It checks that UI files in the running container and responses from selected public routes match the commit. For deployment logs, run `docker compose logs --tail=100 caddy app` from `/opt/gocloudstorage`.

## Tests

With Go 1.26.4 or later installed, run the tests from the repository root. The storage tests cover backend configuration and how the app resolves stored object addresses.

```sh
go test ./...
```
