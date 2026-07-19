FROM ubuntu:26.04@sha256:3131b4cc82a783df6c9df078f86e01819a13594b865c2cad47bd1bca2b7063bb

ARG UBUNTU_MIRROR=http://archive.ubuntu.com/ubuntu
ARG VERSION
ARG REVISION
ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    find /etc/apt -type f \( -name '*.list' -o -name '*.sources' \) -exec sed -i \
        -e "s|http://security.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g" \
        -e "s|http://archive.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g" {} +; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "30";\nAcquire::https::Timeout "30";\nAcquire::http::Pipeline-Depth "0";\n' > /etc/apt/apt.conf.d/80pasturestack-retries; \
    apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    rm -f /usr/bin/pebble

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
