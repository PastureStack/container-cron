# Security

## Docker socket boundary

Container Cron can control the host Docker daemon. Access to
`/var/run/docker.sock` is equivalent to administrative access to the host. Do
not expose the socket to untrusted workloads, and do not treat the container
boundary as a security sandbox.

The runtime image intentionally remains root-compatible for legacy
system-service deployments. A non-root migration requires a separately tested
Docker API authorization design.

## Metadata boundary

Metadata-aware mode trusts service state returned by the configured metadata
endpoint. Keep that endpoint on a protected node-local network, and do not
point the process at an untrusted HTTP service.

## Release gates

Before release, run the race detector, unit tests, static analysis, image build,
and `scripts/smoke`. Validate that an unavailable Docker socket returns an error
without a panic and that a closed event stream retries with a bounded delay.

Release images use immutable semantic version tags. Catalog and Compose files
must use the semantic tag and must not expose an image digest in the UI.
