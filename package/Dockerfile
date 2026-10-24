# syntax=docker/dockerfile:1.7
ARG UBUNTU_SECURITY_BACKPORTS_IMAGE=ghcr.io/pasturestack/ubuntu-security-backports:v0.1.0@sha256:2742a3d429e4e14bc587f6dedc51ded4246cf90bbaedcd86c67d0885e32a6f2a

FROM ${UBUNTU_SECURITY_BACKPORTS_IMAGE} AS ubuntu_security_backports

FROM ubuntu:26.04@sha256:2260313b31c8c011cd2eebe728008efac1b3982be73eb71348ea2648d2c0e09b
ARG UBUNTU_SECURITY_BACKPORTS_IMAGE

ARG VERSION
ARG REVISION

COPY ubuntu-apt.lock /licenses/ubuntu-apt.lock
COPY ubuntu-security-packages.lock /licenses/ubuntu-security-packages.lock
COPY install-ubuntu-security-updates /usr/local/libexec/pasturestack/install-ubuntu-security-updates
COPY ca-certificates.deb /tmp/ca-certificates.deb

ENV DEBIAN_FRONTEND=noninteractive

ADD --checksum=sha256:5b6326f1932cfd7be57130561dc78192f09ed9f58bfbe6606ed0c50510c3b6cd https://archive.ubuntu.com/ubuntu/pool/main/c/ca-certificates/ca-certificates_20260816ubuntu0.26.04.1_all.deb /opt/pasturestack-ca-certificates.deb
RUN set -eux; \
    . /licenses/ubuntu-apt.lock; \
    rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; \
    rm -rf /tmp/ca-bootstrap; \
    mkdir -p /tmp/ca-bootstrap /etc/ssl/certs; \
    dpkg-deb --extract /tmp/ca-certificates.deb /tmp/ca-bootstrap; \
    find /tmp/ca-bootstrap/usr/share/ca-certificates -type f -name '*.crt' \
        | LC_ALL=C sort \
        | while IFS= read -r certificate; do sed -e '$a\' "${certificate}"; done \
        > /etc/ssl/certs/ca-certificates.crt; \
    test -s /etc/ssl/certs/ca-certificates.crt; \
    printf 'Types: deb\nURIs: https://snapshot.ubuntu.com/ubuntu/%s\nSuites: resolute resolute-updates resolute-backports resolute-security\nComponents: main universe restricted multiverse\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\nSnapshot: no\n' \
        "${UBUNTU_APT_SNAPSHOT}" > /etc/apt/sources.list.d/pasturestack-snapshot.sources; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "30";\nAcquire::https::Timeout "30";\nAcquire::http::Pipeline-Depth "0";\nAcquire::https::CaInfo "/etc/ssl/certs/ca-certificates.crt";\nAcquire::https::Verify-Peer "true";\nAcquire::https::Verify-Host "true";\nAcquire::AllowInsecureRepositories "false";\nAPT::Get::AllowUnauthenticated "false";\n' > /etc/apt/apt.conf.d/80pasturestack-retries; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates="${UBUNTU_APT_CA_CERTIFICATES_BOOTSTRAP_VERSION}"; \
    dpkg -i /opt/pasturestack-ca-certificates.deb && \
    update-ca-certificates --fresh && \
    test "$(dpkg-query -W -f='${Version}' ca-certificates)" = '20260816ubuntu0.26.04.1' && \
    test ! -e /usr/share/ca-certificates/mozilla/emSign_ECC_Root_CA_-_G3.crt && \
    rm -f /opt/pasturestack-ca-certificates.deb && \
    sh /usr/local/libexec/pasturestack/install-ubuntu-security-updates /licenses/ubuntu-security-packages.lock /etc/ssl/certs/ca-certificates.crt && \
    { \
        printf 'snapshot\t%s\n' "${UBUNTU_APT_SNAPSHOT}"; \
        dpkg-query -W -f='${binary:Package}\t${Version}\n' | LC_ALL=C sort; \
    } > /licenses/CONTAINER-CRON-RUNTIME-UBUNTU-APT-PACKAGES.tsv; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /usr/bin/pebble

# Keep the build-only installer and package archives outside the final image.
RUN --mount=type=bind,from=ubuntu_security_backports,source=/out,target=/opt/pasturestack/ubuntu-security-backports,readonly \
    set -eux; \
    evidence=/usr/share/pasturestack/security/ubuntu-backports; \
    mkdir -p "$evidence"; \
    /opt/pasturestack/ubuntu-security-backports/install-existing-backports --install-in-docker-build > "$evidence/install.json"; \
    /opt/pasturestack/ubuntu-security-backports/install-existing-backports --verify-installed > "$evidence/verify.json"; \
    for file in manifest.json qualification.json source-inputs.lock.json openvex.json; do \
        cp "/opt/pasturestack/ubuntu-security-backports/$file" "$evidence/$file"; \
    done; \
    printf '%s\n' "$UBUNTU_SECURITY_BACKPORTS_IMAGE" > "$evidence/bundle-image.txt"; \
    (cd "$evidence" && sha256sum manifest.json qualification.json source-inputs.lock.json openvex.json install.json verify.json bundle-image.txt > SHA256SUMS); \
    . /licenses/ubuntu-apt.lock; \
    { \
        printf 'snapshot\t%s\n' "$UBUNTU_APT_SNAPSHOT"; \
        dpkg-query -W -f='${binary:Package}\t${Version}\n' | LC_ALL=C sort; \
    } > /licenses/CONTAINER-CRON-RUNTIME-UBUNTU-APT-PACKAGES.tsv

COPY container-cron /usr/bin/container-cron
COPY LICENSE SECURITY.md UNOFFICIAL_MAINTENANCE_FORK.md ORIGIN.md COMPATIBILITY.md /licenses/
RUN chmod +x /usr/bin/container-cron && \
    ln -s container-cron /usr/bin/container-crontab

LABEL org.opencontainers.image.title="PastureStack Container Cron" \
      org.opencontainers.image.description="Container-aware cron scheduler maintained by PastureStack" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/PastureStack/container-cron" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

CMD ["container-cron"]
