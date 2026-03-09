# wait-jobs

`wait-jobs` is a standalone CLI utility that waits for a group of Kubernetes Jobs in parallel and exits when each job reaches a terminal state.

It solves the `kubectl wait` gap for workflows where you need:
- tracking multiple jobs at once,
- clear status output during execution,
- logs printed when jobs finish (all or only failed).

Related Kubernetes issue: [kubernetes/kubectl#1629](https://github.com/kubernetes/kubectl/issues/1629).

## Why this exists

`kubectl wait` can wait for one condition (`complete` or `failed`) and is awkward for CI/CD scenarios with multiple jobs and informative output.

`wait-jobs` provides:
- parallel tracking of many jobs,
- live status table in TTY and CI-friendly plain text in non-TTY,
- optional log collection after each job finishes.

## Installation

### From source

```bash
go install github.com/chapsuk/wait-jobs@latest
```

### Local build

```bash
make build
./bin/wait-jobs --help
```

## Quick start

Wait by label selector:

```bash
wait-jobs -n staging -l app=migrations --timeout 10m --logs failed
```

Wait by explicit job names:

```bash
wait-jobs -n default migration-a migration-b migration-c --logs all
```

## Configuration source (auto-detect)

`wait-jobs` automatically chooses cluster config:
1. explicit `--kubeconfig`,
2. in-cluster ServiceAccount (`rest.InClusterConfig`) when running inside Kubernetes,
3. fallback to `$KUBECONFIG` or `~/.kube/config`.

This makes one binary usable both locally and inside Kubernetes jobs/pods.

## Flags

- `-n, --namespace` namespace (default: resolved from config source)
- `-l, --selector` label selector for jobs
- `--timeout` max wait duration (default: `5m`)
- `--logs` log mode: `all`, `failed`, `none` (default: `none`)
- `--kubeconfig` path to kubeconfig
- `--context` kubeconfig context
- `--no-color` disable ANSI colors

## Exit codes

- `0` all tracked jobs are successful
- `1` one or more jobs failed
- `2` timeout reached
- `3` runtime/configuration error

## Typical output

```text
Watching 3 jobs in namespace "staging" (timeout: 10m0s)
  JOB                 STATUS      AGE
  data-migration      Complete    1m23s
  schema-update       Failed      0m45s
  seed-data           Running     2m10s

--- Logs: schema-update (Failed) ---
...

Summary: failed=1 deleted=0 total=3
```

## Development

```bash
make tidy
make test
```

## Contributing

Pull requests are welcome. Please include tests for behavior changes.

## License

MIT, see `LICENSE`.
