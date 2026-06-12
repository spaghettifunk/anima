# Anima

Anima collects telemetry, indexes observed metrics, enriches them with PromQL usage evidence, and serves metric usage reports.

More details live in [docs/](docs/).

## Run Locally

Start the local dependencies:

```sh
docker compose up -d
```

If you run Anima from your host with `go run`, make sure `config.toml` points to the exposed local services:

```toml
[producer]
brokers = "localhost:9092"

[consumer]
brokers = "localhost:9092"

[store]
address = "localhost"
```

The root command validates the full config, so keep the other required values populated too. For local Grafana auth, prefer:

```sh
export GRAFANA_TOKEN="..."
```

Run one service per terminal:

```sh
mage collector
mage indexer
mage entityResolver
mage enricher
mage reporter
mage queryAuditProxy
```

Useful local endpoints:

```sh
curl http://localhost:8500/healthz
curl http://localhost:8600/healthz
curl "http://localhost:8600/api/v1/reports/summary?tenant=default-tenant"
curl "http://localhost:8600/api/v1/entities?type=service"
```

Stop dependencies when done:

```sh
docker compose down
```

## Tests

Run the full test suite:

```sh
mage test
```

Run one package:

```sh
go test ./reporter
```

## Collector Profiling

Collector profiling uses [Mage](https://magefile.org/) for local tasks and
[Vegeta](https://github.com/tsenart/vegeta) for HTTP load generation. Install
both before running the profiling workflow:

```sh
go install github.com/magefile/mage@latest
go install github.com/google/pprof@latest
```

Vegeta must also be available in `PATH`.

Start local dependencies and run the collector in a separate terminal:

```sh
docker compose up -d
mage collector
```

If you run Anima from your host, make sure `config.toml` points
Kafka to the exposed local broker:

```toml
[producer]
brokers = "localhost:9092"
```

The collector starts a pprof server on `127.0.0.1:6060`. It is intentionally
bound to localhost and is not mounted on the public Gin router.

Run the end-to-end collector profile:

```sh
mage profile:collector
```

The target generates `profiling/metrics.json`, drives
`POST /telemetry/v1/metrics` with Vegeta, captures pprof data, and writes
artifacts under `profiling/out/<timestamp>/`:

```text
cpu.pprof
heap.pprof
alloc_space.pprof
goroutine.pprof
vegeta.bin
vegeta.txt
vegeta.json
```

Inspect a profile with:

```sh
go tool pprof -http=:10000 profiling/out/<timestamp>/cpu.pprof
go tool pprof -http=:10000 profiling/out/<timestamp>/heap.pprof
go tool pprof -http=:10000 -sample_index=alloc_space profiling/out/<timestamp>/alloc_space.pprof
go tool pprof -http=:10000 profiling/out/<timestamp>/goroutine.pprof
```

Useful overrides:

```sh
PROFILE_DURATION=60s PROFILE_RATE=100/s mage profile:collector
PROFILE_COLLECTOR_URL=http://localhost:8500 PROFILE_PPROF_URL=http://127.0.0.1:6060 mage profile:collector
```

Clean generated profile outputs:

```sh
mage profile:clean
```
