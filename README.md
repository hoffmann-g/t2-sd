# Consistency, latency and throughput in replicated storage

Assignment 2 of the Distributed Systems course (4647B-04, PUCRS, 2026/2). The lab measures the cost of strong consistency (consensus) against eventual consistency, in normal operation, with high network latency and under faults, on two key-value stores with 3 nodes each:

- **etcd 3.5** (Raft, linearizable reads by default): the strong class.
- **Redis 7.4** with two asynchronous replicas (writes on the primary, reads on the replicas): the eventual class.

Nothing is implemented in the stores. The project is the environment (Docker Compose), the load generator, the fault injection scripts, the analysis and the report.

Authors: George Rother, Guilherme Hoffmann, João Sbardelotto.

## Status

Done:

- load generator (`cmd/loadgen`), closed loop, tested against both clusters;
- stale-read coordinator of E3 (`cmd/stale`);
- convergence check for E5 (`cmd/check`);
- Compose files, fault scripts and the scripts of E1 to E6 (`lab/`);
- analysis of every experiment (`cmd/analyze`) and the figures (`plots/`);
- web visualization (`cmd/viz`).

Not done: the measurements themselves (nothing under `results/` is final) and the report.

## Layout

```
cmd/loadgen        load generator (runs inside the gerador container)
cmd/stale          E3 coordinator: PUT on one node, GET on another, counts stale reads
cmd/check          E5: compares the nodes with the confirmed writes after the partition
cmd/analyze        combines the repetitions of an experiment into tables (Markdown and CSV)
plots/             figures of the report, from the CSV files of analyze (Python, uv)
figures/           output of plots/
cmd/viz            web page that drives a live execution (cluster, load, faults)
internal/kv        key-value interface and the etcd and Redis adapters
internal/load      closed loop, samples, statistics
internal/results   loading of results/ for the analysis
lab/etcd.yml       etcd cluster (3 nodes) + generator
lab/redis.yml      Redis primary + 2 replicas + generator
lab/.env           subnets, images and CPU pinning
lab/scripts/       up/down, latency, partition, node failure, E1 to E6
results/           output of the experiments (one directory per run)
enunciado/         assignment text, appendix and the handout files
```

## Requirements

Linux with Docker Engine and Compose v2, the `sch_netem` kernel module (for `tc netem`) and Go 1.26 to run `analyze` and `viz` on the host. The generator binaries are built inside the `lab-gerador` image, so Go is not needed for the experiments themselves.

## Environment

Every node has two networks, as in the handout: `lab_cluster` for traffic between nodes (replication, consensus) and `lab_clientes` for the generator. Latency and partition are applied only to the first one. The subnets are `172.30.0.0/24` and `172.31.0.0/24` (changed from the handout, whose ranges were in use on the measurement host); they are set in [lab/.env](lab/.env), together with the images and the CPU sets of each container (one physical core per node, the rest for the generator).

```sh
lab/scripts/up.sh etcd        # clean etcd cluster + generator, waits until ready
lab/scripts/up.sh redis       # same for Redis (removes the etcd cluster first)
lab/scripts/down.sh           # stops whatever is up and deletes the data
```

The `gerador` container runs the Go tools and mounts `results/` at `/results`.

## Load generator

```sh
docker exec gerador loadgen -threads 16 -duration 2m -warmup 30s -out /results/demo
```

N clients repeat, with no pause: draw an operation (80% GET, 20% PUT) and a key (uniform among 10,000), send it to a node, wait for the answer or the timeout (2 s), record a sample. The draws depend only on the seed and on the client number. A request that times out or fails counts as an error and has no latency. Values have 256 bytes and start with a stamp `client:seq:` that identifies the write (used by `stale` and `check`).

By default the clients rotate over the nodes they can use (every node in etcd; the primary for writes and the replicas for reads in Redis) and skip a node for one second after a failed request. With `-pinned`, client i only uses node i mod 3, which shows what the clients of each node observe (E5).

The output directory receives `summary.json` (throughput, p50/p95/p99 per operation and per node over the measured window, generator CPU use, error messages), `timeline.csv` (completions, errors and latency per second, per node and operation) and, with `-samples`, `samples.csv.gz` with every PUT and every failed request (`-samples-all` writes every request).

The retries built into both client libraries are disabled, so every failure shows up in the samples.

## Experiments

Each script starts from a clean cluster for every repetition, records the CPU of every container (`cpu.csv`), the state of the nodes at the end (`server-n*.txt`) and the host description (`environment.txt`). A run whose output exists is skipped, so an interrupted series resumes.

```sh
lab/scripts/e1.sh etcd          # saturation: N = 1, 2, 4, ..., 512, 2 min each, 3 reps
lab/scripts/e1.sh redis
go run ./cmd/analyze e1         # table with median and range, knee and plateau; writes results/e1/e1.csv
go run ./cmd/analyze all        # every experiment that has results; tables as Markdown and CSV

lab/scripts/e2.sh etcd 32       # baseline with the optimal N of E1, 3 min
lab/scripts/e3.sh etcd          # stale reads, 10,000 pairs, normal network and 50 ms delay
lab/scripts/e4.sh etcd 32       # same as E2 with 50 ms per direction between nodes (RTT 100 ms)
lab/scripts/e5.sh etcd 32       # partition of n1 for 60 s with pinned clients, then convergence check
lab/scripts/e6.sh etcd 32       # kill the leader/primary, then a follower/replica; restart after 60 s
```

Fault scripts, usable on their own:

```sh
lab/scripts/latencia_on.sh 50ms n1 n2 n3    # tc netem on the cluster interface of each node
lab/scripts/rtt.sh n1 n2                    # ping between nodes on the cluster network
lab/scripts/latencia_off.sh n1 n2 n3
lab/scripts/particao.sh                     # disconnects n1 from lab_cluster
lab/scripts/cura.sh
lab/scripts/leader.sh                       # prints the etcd leader or the Redis primary
docker kill n2; docker start n2
```

E1 alone takes about 70 minutes per system. Run the experiments on mains power, with the CPU governor on `performance` and nothing else heavy on the machine.

## Figures

```sh
cd plots && uv run plots        # writes figures/*.png from the CSV files of cmd/analyze
```

`plots/` is a small Python project (matplotlib, pandas) managed with `uv`. It draws the saturation curve of E1, the percentiles of E2 and E4 side by side, the stale-read rate of E3 and, for E5 and E6, the successful requests per second by node and the errors per second, aligned on the instant of the fault, one panel per repetition.

## Visualization

```sh
go run ./cmd/viz        # http://localhost:8080
```

The page starts a cluster, runs the generator or the E3 coordinator inside the `gerador` container and injects the faults with the scripts above. It shows, per second, the successful requests by node, the errors, the latency percentiles and the stale reads, with the faults marked on the timeline, plus the role of each node (leader, follower, primary, replica, down). It must run on the host, from the project root.

## Tests

```sh
go test ./...
```

They cover the closed loop, the routing (skipping and probing of failed nodes, pinned mode), the sample encoding, the statistics and the timeline, with an in-memory store.
