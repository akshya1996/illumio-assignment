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
- `make` (only for the one-command demo).

## Quick start

One command builds the CLI, runs `go vet` and the tests, regenerates the sample data, and runs every query on the sample files:

```bash
make demo
```

It runs these steps in order and stops at the first one that fails, so the queries only run if the build and tests pass:

1. `build`: builds `./flowline`.
2. `test`: runs `go vet ./...` and `go test ./...`.
3. `generate`: rewrites `samples/generated_flow.log`. The seed is fixed, so the file is identical every time.
4. Runs the queries, each under an `===` heading, with the command printed above its output:
   - filter, conversations and top talkers on `samples/flow_v2.log`
   - the same three queries on `samples/flow_v3.log`, which has a different column order
   - top talkers among `REJECT` traffic in the generated data, to show that filters also apply to the other queries

Other targets: `make build`, `make test`, `make generate`, `make clean` (removes `./flowline`).


The data-quality evaluation (Track B) can be run separately:

```bash
go run ./cmd/qualitygen
go run ./cmd/qualityeval
```

This generates a deterministic dataset with labeled anomalies and evaluates the data-quality checks against the ground-truth CSV.



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

Output:

```text
Parsed 3 valid records
Matched 2 records:
10.0.1.1:52000 -> 10.0.1.2:443 protocol=6 packets=12 bytes=1200 start=200 end=250 action=ACCEPT
10.0.1.1:52001 -> 10.0.1.3:443 protocol=6 packets=6 bytes=600 start=210 end=260 action=REJECT
Parsed 3 valid records
Found 3 conversations:
10.0.1.1:52000 -> 10.0.1.2:443 protocol=6: 1 record
10.0.1.1:52001 -> 10.0.1.3:443 protocol=6: 1 record
10.0.1.2:443 -> 10.0.1.1:52000 protocol=6: 1 record
Parsed 3 valid records
Top talkers:
10.0.1.1: 1800 bytes
10.0.1.2: 800 bytes
```

The v3 file lists `dstaddr` before `srcaddr` (and swaps several other columns), yet the results are read into the right fields: `10.0.1.1` is correctly the source.

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
go run ./cmd/flowline --input samples/generated_flow.log --query top-talkers --from 1000 --to 4600 --n 3
```

## Track B: Data Quality — Trust but Verify

Flowline includes a data-quality evaluation tool that detects anomalies in VPC flow logs.

### Detected anomalies

The implementation checks for five types of data-quality issues:

1. **Broken fields:** Records that cannot be parsed, such as invalid IP addresses or malformed fields.
2. **Impossible values:** Negative packet or byte counts, and records where the start time is after the end time.
3. **Duplicates:** Exact duplicate flow records.
4. **One-sided TCP:** TCP flows without a corresponding reverse-direction flow.
5. **Traffic spikes:** Unusually high byte volumes for a source IP within a 60-second time bucket, compared with its median bucket volume.

### Generate test data

The quality data generator uses a fixed random seed and injects labeled anomalies into the generated dataset.

```bash
go run ./cmd/qualitygen
```

This generates:
- `samples/quality_flow.log`
- `samples/quality_ground_truth.csv`

### Run the evaluation

```bash
go run ./cmd/qualityeval
```

The evaluator compares detected anomalies against the ground-truth labels and reports:
- Expected anomalies
- Correctly detected anomalies
- False positives
- Missed anomalies
- Catch rate per category
- Overall catch rate

### Evaluation results

On the generated test dataset, all five anomaly categories were detected successfully, with a 100% catch rate and zero false positives.

These results demonstrate correctness against the injected test cases. They do not establish production detection accuracy. The traffic-spike threshold is heuristic and may require tuning against real traffic patterns.

## Design

Flowline is a pipeline of three steps: **parse → filter → query**.


| Package | Role |
|---|---|
| `internal/model` | `FlowRecord`, the parsed form of one log line. |
| `internal/parser` | Reads the header, maps each column name to its position, then parses each line by name. Bad lines are collected as errors instead of stopping the run. |
| `internal/query` | `FilterRecords`, `CountConversations` and `TopTalkers`. Each is a pure function over a slice of records, so they are easy to test on their own. |
| `internal/quality` | Implements five data-quality checks: broken fields, impossible values, duplicates, one-sided TCP flows and traffic spikes. |
| `cmd/flowline` | The CLI. It checks all flags before opening the file, then parses, filters, and runs the selected query. |
| `cmd/samplegen` | Writes `samples/generated_flow.log` from a fixed seed. |
| `cmd/qualitygen` | Generates synthetic flow records with labeled anomalies and writes the ground-truth CSV. |
| `cmd/qualityeval` | Compares detected anomalies against ground-truth labels and calculates detection metrics, including catch rate and false positives. |


Fields that can be missing in the log (IPs and ports) are stored as pointers, so "missing" (`nil`) is never confused with a real `0`. Conversations are map keys, so they hold plain values plus a `...Present` flag for each optional field.

The main choices and the alternatives I rejected are in [DECISIONS.md](DECISIONS.md).

## Assumptions

- The first line of each file is a header naming the columns, using AWS VPC Flow Log field names (`srcaddr`, `dstport`, `log-status`, and so on). Values are separated by whitespace.
- `start` and `end` are Unix timestamps in seconds.
- `protocol` is an IANA protocol number (6 = TCP, 17 = UDP), not a name.
- `action` is `ACCEPT` or `REJECT`, written in uppercase as AWS does.
- If there is no `log-status` column, every line is treated as a normal traffic record.

## Trade-offs

- **Time windows use the start time only.** Each record falls in exactly one window, so nothing is counted twice, but a long flow puts all its bytes in the window where it started.
- **Conversations are directional.** A→B and B→A are counted separately, which matches how VPC logs record flows, but answering "how much did A and B talk in total" means adding two lines.
- **Bad lines are skipped with a warning.** One corrupt line doesn't lose the rest of the file, but results can be based on incomplete data, so warnings should be checked.
- **Everything is in memory.** All records are loaded before querying. This keeps the code simple and lets one parse serve any query, but limits file size to available memory.

## Limits

- One input file per run, processed on a single thread. Very large files are limited by memory.
- A line longer than 1 MB stops reading at that point, and the rest of the file is not processed. The error is reported.
- Filters are exact matches only: no IP ranges (CIDR), port ranges, or OR conditions.
- Duplicate lines are counted every time they appear.
- `10.0.0.1` and its IPv4-mapped IPv6 form `::ffff:10.0.0.1` are treated as different addresses.
- Output is plain text only (no JSON or CSV).
- The generated sample data uses only the v2 layout and contains no odd records (`NODATA`, `-`, IPv6 or broken lines). Those cases are covered by the unit tests in `internal/parser`.
- Data-quality checks use simple heuristics. One-sided TCP flows are not necessarily invalid, and traffic-spike detection may need tuning for real traffic distributions.

## Use of AI tools

I used Claude Code (an AI coding assistant) for the following:

- **Code comments:** Added doc and inline comments to the Go source and test files (`internal/model`, `internal/parser`, `internal/query`, `cmd/flowline`). Code behavior was not changed.
- **Checking the README:** Ran every command in this README to make sure it builds, passes tests and gives the documented results.
- **CLI output fixes:** Fixed `filter` output, which printed ports as memory addresses instead of numbers. Made `conversations` output easier to read, fixed plurals ("1 record"), and sorted conversations by IP and port as numbers instead of as text. Added tests for these changes in `cmd/flowline/main_test.go`.
- **README updates:** Added example output and clarified the descriptions of `--from`, `--to` and `--action`.
- **Review:** Reviewed the README, DECISIONS.md and the sample generator against the assignment requirements.
- **DECISIONS.md:** Rewrote it into the three-decision format, with rejected alternatives, based on how the code already behaves.
- **Long lines:** Raised the parser's maximum line length from 64 KB to 1 MB and added tests for it.
- **Demo and write-up:** Added the `Makefile` (`make demo`) and the Design, Assumptions, Trade-offs and Limits sections of this README.

I reviewed all AI-generated changes and checked them by running `go test ./...` and `go vet ./...`.
