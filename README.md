# Flowline

Flowline is a command-line tool written in Go for analyzing AWS VPC Flow Logs.

It parses flow log records using the field order defined in each log file's header and supports filtering traffic, counting conversations, and identifying top talkers by bytes transferred.

## Features

- Header-driven parsing that supports different field orders and optional fields.
- IPv4 and IPv6 address support.
- Filtering by source IP, destination IP, source port, destination port, protocol, and action.
- Counting records by directional five-tuple (source IP, source port, destination IP, destination port, protocol).
- Identifying top talkers by total bytes within a specified time window.
- Skipping `NODATA` and `SKIPDATA` records and reporting malformed records.

## Requirements

- Go 1.27.3 or later.

## Build

```bash
go build -o flowline ./cmd/flowline
```

## Run tests

```bash
go test ./...
go vet ./...
```

## Usage

```bash
./flowline --input <log-file> --query <query-name> [options]
```

### Query types

| Query | Description |
|---|---|
| `filter` | Print records matching the supplied filters |
| `conversations` | Count records grouped by directional five-tuple |
| `top-talkers` | Rank source IPs by total bytes in a time window |

### Available flags

| Flag | Description | Default |
|---|---|---|
| `--input` | Path to the flow log file (required) | — |
| `--query` | Query to execute: `filter`, `conversations`, or `top-talkers` | `filter` |
| `--src-ip` | Filter by source IP | — |
| `--dst-ip` | Filter by destination IP | — |
| `--src-port` | Filter by source port | — |
| `--dst-port` | Filter by destination port | — |
| `--protocol` | Filter by protocol number | — |
| `--action` | Filter by action (`ACCEPT` or `REJECT`, any case) | — |
| `--from` | Inclusive start of the `top-talkers` window, in Unix seconds | `0` |
| `--to` | Exclusive end of the `top-talkers` window, in Unix seconds | `0` |
| `--n` | Maximum number of top talkers to return | `10` |

All supplied filters are combined using AND semantics. Filters are applied before executing the selected query.

For `top-talkers`, a record is counted if its `start` time is in `[--from, --to)`.

Malformed lines are printed to stderr as warnings and skipped; the rest of the file is still processed.

## Examples

Build the CLI first:

```bash
go build -o flowline ./cmd/flowline
```

### 1. Filter records

Find records where the source IP is `10.0.0.1`:

```bash
./flowline --input samples/flow_v2.log --query filter --src-ip 10.0.0.1
```

Output:

```text
Parsed 3 valid records
Matched 2 records:
10.0.0.1:50000 -> 10.0.0.2:443 protocol=6 packets=10 bytes=500 start=100 end=150 action=ACCEPT
10.0.0.1:50001 -> 10.0.0.3:443 protocol=6 packets=5 bytes=300 start=110 end=160 action=ACCEPT
```

Missing IPs or ports (`-` in the log) are printed as `-`, and IPv6 addresses are shown in brackets, e.g. `[2001:db8::1]:443`.

### 2. Count conversations

Count records grouped by directional five-tuple:

```bash
./flowline --input samples/flow_v2.log --query conversations
```

Output:

```text
Parsed 3 valid records
Found 3 conversations:
10.0.0.1:50000 -> 10.0.0.2:443 protocol=6: 1 record
10.0.0.1:50001 -> 10.0.0.3:443 protocol=6: 1 record
10.0.0.2:443 -> 10.0.0.1:50000 protocol=6: 1 record
```

Conversations are directional, so the reply from `10.0.0.2` is counted separately from the request to it.

### 3. Find top talkers

Find the top two source IPs by bytes transferred between timestamps 100 and 200:

```bash
./flowline --input samples/flow_v2.log --query top-talkers --from 100 --to 200 --n 2
```

Output:

```text
Parsed 3 valid records
Top talkers:
10.0.0.1: 800 bytes
10.0.0.2: 400 bytes
```

### 4. Parse a different header layout

`samples/flow_v3.log` uses a different field order. The same queries work without changing the parser:

```bash
./flowline --input samples/flow_v3.log --query filter --src-ip 10.0.1.1
./flowline --input samples/flow_v3.log --query conversations
./flowline --input samples/flow_v3.log --query top-talkers --from 200 --to 300 --n 2
```
## Generating sample data

Flowline includes a deterministic sample-data generator that produces 100 synthetic VPC flow log records.

The generator uses a fixed random seed (`42`), so repeated runs produce identical output.

Run it from the project root:

```bash
go run ./cmd/samplegen
```

This creates:

```text
samples/generated_flow.log
```

The generated file can be used with any Flowline query:

```bash
go run ./cmd/flowline --input samples/generated_flow.log --query conversations
go run ./cmd/flowline --input samples/generated_flow.log --query top-talkers --from 1000 --to 4600 --n 5
```

## Use of AI tools

I used Claude Code (an AI coding assistant) for the following:

- **Code comments:** Added doc and inline comments to the Go source and test files (`internal/model`, `internal/parser`, `internal/query`, `cmd/flowline`). Code behavior was not changed.
- **Checking the README:** Ran every command in this README to make sure it builds, passes tests and gives the documented results.
- **CLI output fixes:** Fixed `filter` output, which printed ports as memory addresses instead of numbers. Made `conversations` output easier to read, fixed plurals ("1 record"), and sorted conversations by IP and port as numbers instead of as text. Added tests for these changes in `cmd/flowline/main_test.go`.
- **README updates:** Added example output and clarified the descriptions of `--from`, `--to` and `--action`.
- **Review:** Reviewed the README, DECISIONS.md and the sample generator against the assignment requirements.
- **DECISIONS.md:** Rewrote it into the three-decision format, with rejected alternatives, based on how the code already behaves.

I reviewed all AI-generated changes and checked them by running `go test ./...` and `go vet ./...`.
