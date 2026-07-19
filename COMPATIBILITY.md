# Compatibility contracts

PastureStack uses `container-cron` for the repository, executable, image, Go
module, and metrics namespace. The POC intentionally exposes only neutral
interfaces:

- `container-crontab` is an executable alias for `container-cron`.
- `--metadata-mode` enables service-state checks.
- `io.pasturestack.stack.name` identifies the workload group.
- `io.pasturestack.project_service.name` identifies the service.
- The audited metadata client is maintained as an internal package so no
  upstream-branded import path becomes part of the public API.

Monitoring must use `pasturestack_container_cron_jobs` before switching
executables.

Historical identifiers are documented only in the private migration knowledge
base. They are not accepted by this POC because doing so would reintroduce
retired brand contracts into new public code.
