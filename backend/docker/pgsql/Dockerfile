FROM postgres:18beta3
ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    apt-get update; \
    # Runtime libs we keep
    apt-get install -y --no-install-recommends \
        libcld2-0 \
        pgxnclient; \
    # Build deps (PGXS + toolchain + CLD2 headers)
    apt-get install -y --no-install-recommends \
        postgresql-server-dev-18 \
        build-essential \
        libcld2-dev; \
    # Build & install the extension
    pgxn install pg_cld2; \
    # Remove only the build deps; let --auto-remove pick their leaf deps
    apt-get purge -y --auto-remove \
        postgresql-server-dev-18 \
        build-essential \
        libcld2-dev; \
    rm -rf /var/lib/apt/lists/* /root/.pgxn
