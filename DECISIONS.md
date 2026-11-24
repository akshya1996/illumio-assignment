# Design Decisions

## 1. A record belongs to the time window that contains its start time

**Decision:** For `top-talkers`, a record is counted in `[from, to)` if `from <= start < to`. All of its bytes are counted, even if the flow ends after `to`.

**Rejected alternative:** Splitting a record's bytes across windows in proportion to how much of `[start, end]` overlaps each one.

**Why:** Using only the start time puts each record in exactly one window. Its bytes are never counted twice or lost, and adjacent windows add up to the total. Splitting by overlap would be closer to the truth for long flows, but it assumes traffic is spread evenly across the flow, which the log doesn't tell us. It also produces fractional byte counts. The end-exclusive bound is what keeps adjacent windows from sharing a record.

**Cost:** A long flow puts all its bytes in the window where it started.

## 2. Conversations are directional

**Decision:** A conversation is the 5-tuple (source IP, source port, destination IP, destination port, protocol) as written in the record. A→B and B→A are counted separately.

**Rejected alternative:** Treating both directions as one conversation, by putting the two endpoints in a fixed order before grouping.

**Why:** VPC Flow Logs already record each direction as its own flow, so directional counts match the data as written. Keeping directions apart also shows when one side of a TCP conversation is missing. Merging them would need a rule for which side is the "client", and the log doesn't reliably say.

**Related:** A missing port (`-`) is kept separate from port `0`, so those records are never grouped together.

## 3. A bad line is skipped with a warning; a bad header stops the run

**Decision:** If a line has the wrong number of fields, an invalid value or an unknown `log-status`, it is reported to stderr with its line number and skipped. Parsing continues. A header problem (a missing required column or a duplicate column) stops the run before any records are read.

**Rejected alternative:** Stopping at the first bad line.

**Why:** Real flow logs are large and sometimes contain a corrupt line. Throwing away the whole file because of one line is worse for the user than skipping it and saying so. The header is different: if it's wrong, every line would be read into the wrong fields, so continuing would give answers that look valid but are wrong.

## How odd records are handled

| Case | Handling | Why |
|---|---|---|
| `NODATA` / `SKIPDATA` | Skipped without a warning | They describe no traffic (no data in the interval, or records dropped during capture). Counting them would add empty records. |
| `-` in an IP or port field | Accepted; stored as missing | It's normal VPC output, for example ICMP has no ports. The record still has useful byte and packet counts. |
| `-` in any other field | Line is rejected as broken | Fields like `bytes` and `protocol` are needed by every query. Guessing a value would change the results. |
| IPv6 addresses | Accepted | VPCs can carry IPv6 traffic. `net/netip` handles both. |
| Unknown `log-status` | Line is rejected as broken | It might be a new status with a different meaning, so it's safer to report it than to guess. |

## Other choices

- Filters apply to every query, not just `filter`, so for example "top talkers among REJECTed traffic" works. Multiple filters are combined with AND.
- A record with no value for a filtered field does not match. An IP filter never matches a record whose IP is `-`.
- `--action` accepts any case and is converted to uppercase to match the log.
- Top talkers with equal byte totals are sorted by IP, so the output is the same on every run.
- IPv4 and IPv4-mapped IPv6 addresses (`10.0.0.1` and `::ffff:10.0.0.1`) are treated as different addresses.



## 4. Data-quality checks use simple, explainable heuristics

**Decision:** Track B implements five data-quality checks: broken fields, impossible values, duplicates, one-sided TCP flows and traffic spikes.

**Rejected alternative:** Building a more elaborate anomaly-detection system using statistical models or machine learning.

**Why:** The assignment asks for a small tool that identifies bad data and measures how well it detects known issues. Simple checks are easier to understand, test and explain. They also make it clear why a record was flagged.

### Individual checks

| Check | Decision | Trade-off |
|---|---|---|
| Broken fields | Reuse parser errors to identify malformed records. | Malformed records are skipped and cannot be checked by record-level detectors. |
| Impossible values | Flag negative packet or byte counts, and records where `start > end`. | These checks identify obvious invalid values but cannot detect every semantically incorrect record. |
| Duplicates | Compare all fields in a flow record and flag subsequent exact matches. | Similar but non-identical records are not considered duplicates. |
| One-sided TCP | Flag TCP records without a matching reverse-direction flow. | A missing reverse flow is not necessarily invalid; asymmetric routing or incomplete visibility can explain it. |
| Traffic spikes | Group bytes by source IP and 60-second time buckets. Calculate the median bucket volume per source IP and flag buckets exceeding 3 times the median. | The threshold is heuristic and may flag legitimate traffic bursts or miss smaller anomalies. |

### Evaluation approach

**Decision:** Use a deterministic synthetic dataset with injected anomalies and a ground-truth CSV containing the line numbers and categories of those anomalies.

**Rejected alternative:** Evaluating detection accuracy using unlabeled data or manually inspecting detector output.

**Why:** Known anomaly labels make it possible to calculate true positives, false positives, false negatives and catch rate for each category. A fixed random seed makes the dataset reproducible.

**Cost:** The evaluation measures detection against the injected test cases, not real-world production accuracy. The synthetic traffic distribution is deliberately controlled, so results should not be interpreted as a guarantee of production performance.
