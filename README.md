# surisink

[![Go Version](https://img.shields.io/github/go-mod/go-version/luhtaf/surisink)](go.mod)
[![License](https://img.shields.io/github/license/luhtaf/surisink)](LICENSE)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](Dockerfile)
[![Go Report Card](https://goreportcard.com/badge/github.com/luhtaf/surisink)](https://goreportcard.com/report/github.com/luhtaf/surisink)

**Suricata → Object Storage, simplified.**

`surisink` takes every file extracted by Suricata and reliably stores it into **S3-compatible object storage** (MinIO, Ceph, Wasabi, AWS S3, …).  
It acts as a **bridge** between network-level detection and long-term evidence storage — so you don’t lose critical files during incident response.

Together with [**corator**](https://github.com/luhtaf/corator), which operates at the **application layer**, `surisink` focuses on the **network layer**.  
Both projects share the same goal: making sure important files are captured and preserved securely, each from a different vantage point.  
Deployed on a mirror / tap / span port, `surisink` passively collects files without affecting production traffic — complementing `corator` at the application edge.

Because it works at the **network level**, `surisink` can also capture files transferred over lower-layer protocols such as **FTP, SCP, SMB, SMTP attachments**, or even **HTTP traffic outside the protected application layer** — transactions that would not be visible to `corator`.

This approach ensures that files are safeguarded across multiple layers without forcing changes to existing system architectures.  
Downstream tools — e.g., [s3nitor](https://github.com/luhtaf/s3nitor) — can then scan and analyze these stored files.

---

## Why surisink?
- **Evidence preservation**: every extracted file ends up in object storage, with proper metadata.
- **Seamless integration**: works directly with Suricata’s filestore.
- **Efficient**: lightweight tailer instead of filesystem polling (reads Suricata’s `eve.json` for `fileinfo` events).
- **Pluggable ecosystem**: pairs naturally with scanning pipelines.

---

## Features
- Tails Suricata extraction events (`fileinfo: stored=true`) and uploads only stored files.
- Flexible filestore path resolution:
  - `absolute`: use full path if available.
  - `file_id`: reconstruct from file ID and naming pattern (e.g., `file.%d`).
  - Optional date-based subdirectories.
- Uploads files to S3 with metadata and tags:
  - SHA-256, MIME type, timestamp, flow ID, source/destination IP.
- Deduplication by SHA-256:
  - In-memory (default).
  - Persistent SQLite (production-ready).
- Configurable worker pool, retry, and backoff.
- Structured JSON logging for easy forwarding to Elasticsearch, Loki, etc.

---

## Quick Start

```bash
git clone https://github.com/luhtaf/surisink.git
cd surisink
make build
CONFIG_PATH=./configs/config.example.yaml ./bin/surisink
```

### Docker
```bash
docker build -t surisink:latest .
docker run --rm   -v $(pwd)/configs/config.example.yaml:/app/config.yaml   -v /var/log/suricata:/var/log/suricata   -v /var/lib/suricata/filestore:/var/lib/suricata/filestore   -e CONFIG_PATH=/app/config.yaml   surisink:latest
```

### Docker Compose (example)
```yaml
version: "3.8"
services:
  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports: ["9000:9000", "9001:9001"]
    volumes: ["./.data/minio:/data"]

  suricata:
    image: jasonish/suricata:latest
    network_mode: host
    cap_add: ["NET_ADMIN", "NET_RAW"]
    volumes:
      - ./suricata:/etc/suricata
      - ./suricata/log:/var/log/suricata
      - ./suricata/filestore:/var/lib/suricata/filestore

  surisink:
    build: .
    environment:
      CONFIG_PATH: /app/config.yaml
    volumes:
      - ./configs/config.example.yaml:/app/config.yaml
      - ./suricata/log:/var/log/suricata
      - ./suricata/filestore:/var/lib/suricata/filestore
    depends_on: [minio]
```

---

## Configuration

See [`configs/config.example.yaml`](configs/config.example.yaml) for all options.

Key sections:
- **suricata**: `eve.json` path, filestore dir, path strategy.
- **uploader**: workers, retries, prefix.
- **s3**: endpoint, credentials, bucket.
- **dedupe**: enable/disable, SQLite path.
- **logging**: level and format (json/console).

---

## Logging

All events are structured JSON logs. Example events:

- `eve_received`: Suricata signaled a new file extraction.
- `upload_success`: file uploaded with metadata.
- `upload_retry`: temporary failure, retrying.
- `upload_failed`: permanent failure after retries.
- `skip_duplicate`: file already known, skipped.

Each log line includes fields such as `sha256`, `flow_id`, `src`, `dst`, `size`, `mime`, and S3 key.

---

## Deduplication

To avoid uploading the same file multiple times:
- **In-memory dedupe** by default (cleared on restart).
- **SQLite dedupe** for persistence across restarts:
  ```yaml
  dedupe:
    enabled: true
    sqlite_path: "./data/surisink.db"
  ```

---

## Diagram

```
Suricata → surisink → S3
   (extracts)   (stores evidence)
```

---

## License
MIT
