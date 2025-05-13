FROM python:3.11-slim

ARG REPL_TAG=v4.19.0
ARG GO_VERSION=1.24.3
ENV ELAN_HOME="/elan-cache"
ENV PATH="/elan-cache/bin:${PATH}"
ENV REPL_PATH="/app/repl"

RUN apt-get update && apt-get install -y \
    git \
    wget \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 -b ${REPL_TAG} https://github.com/leanprover-community/repl.git /app/repl

RUN wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
ENV PATH="${PATH}:/usr/local/go/bin"

WORKDIR /app
COPY go.mod app /app


# Assume the elan-cache directory is mounted
CMD ["go", "run", "server.go"]
