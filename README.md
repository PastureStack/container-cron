# PastureStack Container Cron

Container Cron watches Docker events and runs `start`, `stop`, or `restart`
actions according to cron expressions stored in container labels.

The maintained runtime targets the PastureStack 1.6 compatibility control
plane and current Docker Engine releases.

## Build and test

The supported development workflow uses the repository's disposable Dapper
image:

```sh
make ci
```

The resulting executable is `bin/container-cron`. The build also creates the
legacy `bin/container-crontab` executable alias for staged upgrades.

Run the isolated container lifecycle test after packaging:

```sh
VERSION_OVERRIDE=v0.6.0 TAG=v0.6.0 make smoke
```

The smoke test creates only uniquely named disposable containers and verifies
that scheduled `start` and `stop` actions both reach the Docker daemon.

## Run

Standard Docker mode:

```sh
./bin/container-cron
```

Metadata-aware mode:

```sh
./bin/container-cron --metadata-mode \
  --metadata-url http://169.254.169.250/2016-07-29
```

The process needs access to the Docker API. Mounting `/var/run/docker.sock`
grants host-level control and must only be done on trusted nodes.

## Labels

- `cron.schedule`: required cron expression.
- `cron.action`: optional `start`, `stop`, or `restart`; defaults to `start`.
- `cron.restart_timeout`: optional stop/restart timeout in seconds; defaults to
  10 seconds.
- `cron.leader`: retained for compatibility with existing workload metadata.

Cron expressions use the six-field format supported by `robfig/cron.v2`.

## Metrics

Pass `--metrics` to expose Prometheus metrics on port `9191`. The current job
count is reported as:

```text
pasturestack_container_cron_jobs{hostname="...",state="active|inactive"}
```

## Compatibility

Runtime aliases and legacy metadata contracts are documented in
[COMPATIBILITY.md](COMPATIBILITY.md). New deployments should use only the
PastureStack names shown in this README.

## Origin and independence

PastureStack is an independent community effort to preserve, audit, and modernize the Rancher 1.6 ecosystem. It is not affiliated with or endorsed by Rancher Labs or SUSE.

**Upstream:** [`rancher/container-crontab`](https://github.com/rancher/container-crontab). This GitHub fork retains the upstream Git history, authorship, dates, and license notices unchanged; PastureStack maintenance is consolidated into one commit after the preserved upstream boundary.

Past authorship and project origin are documented in [ORIGIN.md](ORIGIN.md) and
remain available in the unmodified Git history. PastureStack does not claim
exclusive authorship of the original work.

## License

This repository retains its existing Apache License 2.0 terms. See [LICENSE](LICENSE).
