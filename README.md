# PastureStack Container Cron

Container Cron watches Docker events and runs `start`, `stop`, or `restart`
actions according to cron expressions stored in container labels.

The maintained runtime targets the PastureStack 1.6 compatibility control
plane and current Docker Engine releases.

The current public release is [`v0.6.1`](https://github.com/PastureStack/container-cron/releases/tag/v0.6.1), produced from `2cce2cc80f2590239d5ef758a21c54a3f1e0f2b9` by the [successful release gate](https://github.com/PastureStack/container-cron/actions/runs/39960497907). The published `ghcr.io/pasturestack/container-cron:v0.6.1` digest is `sha256:ea2790f2a3e32eb6e3266eb175ee0a69c4a854463c22a74604f69855da5c6829`. Later source or documentation changes do not alter that producer; component publication is not Server deployment.

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
VERSION_OVERRIDE=v0.6.1 TAG=v0.6.1 make smoke
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

`DOCKER_HOST` supports an absolute Unix socket or one exact TCP, HTTP, or HTTPS
endpoint. Network endpoints may contain only a scheme, host, and optional port.
Container Cron pins every request to that endpoint and rejects HTTP redirects.
Use verified TLS for remote Docker daemons; unencrypted TCP should be limited to
an isolated trusted network.

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

## October 2026 security maintenance

Maintained Go builds select the patched Go 1.27.3 compiler without raising the existing Go language requirement. Ubuntu build/runtime stages apply the checksum-pinned `ca-certificates 20260816ubuntu0.26.04.1` security package after their frozen APT baseline; the final trust store and installed version are verified. Release artifacts must pass the repository's existing checks before publication.

## Full-severity release policy

Release verification retains the complete runtime vulnerability report. Every
finding with an official fix blocks release, regardless of severity. Unknown
findings block unless a separately reviewed, exact not-affected statement is
proved by the actual product dependency graph. LOW/MEDIUM findings without an
official fix require the repository's exact CVE/package/version/PURL review ledger;
new or changed findings fail closed. This policy-only update does not replace any
already published immutable artifact.
