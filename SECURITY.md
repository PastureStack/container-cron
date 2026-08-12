# Security

## Docker socket boundary

Container Cron can control the host Docker daemon. Access to
`/var/run/docker.sock` is equivalent to administrative access to the host. Do
not expose the socket to untrusted workloads, and do not treat the container
boundary as a security sandbox.

Network-based `DOCKER_HOST` settings are parsed once at startup. Requests use a
fixed internal URL authority while the transport connects only to that parsed
endpoint; credentials, paths, queries, fragments, invalid ports, and HTTP
redirects are rejected. Remote Docker access should use verified TLS. Plain TCP
is retained for compatibility but must remain on an isolated trusted network.

The runtime image intentionally remains root-compatible for legacy
system-service deployments. A non-root migration requires a separately tested
Docker API authorization design.

## Metadata boundary

Metadata-aware mode trusts service state returned by the configured metadata
endpoint. Keep that endpoint on a protected node-local network, and do not
point the process at an untrusted HTTP service.

## Release gates

Before release, run the race detector, unit tests, static analysis, image build,
endpoint-pinning regression tests, and `scripts/smoke`. Validate that an
unavailable Docker socket returns an error without a panic and that a closed
event stream retries with a bounded delay.

Build and runtime Ubuntu packages are resolved from the recorded snapshot in
`ubuntu-apt.lock`. Go and Docker CLI downloads are verified with architecture-
specific SHA-256 values before use. The disposable builder deliberately omits
Docker Buildx until its latest released binary no longer embeds vulnerable
Moby daemon code; this single-platform image uses Docker's compatibility
builder instead.

The gate retains the raw Trivy report for the exported builder filesystem. All
Critical and High findings must match `security/dapper.openvex.json` exactly by
vulnerability ID and package PURL, without duplicates. Both sorted match sets
and a machine-readable applicability summary are retained as evidence. The
review fails closed if a finding, version, PURL, or statement changes. This
single-scan review avoids relying on Trivy's experimental local-VEX filtering
while preserving an independently auditable OpenVEX decision. It also verifies
that `linux-libc-dev` supplies user-space headers only and that no Linux kernel
image or module exists in the disposable builder.

Release images use immutable semantic version tags. Catalog and Compose files
must use the semantic tag and must not expose an image digest in the UI.
