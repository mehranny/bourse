# --- build the Go binary (web assets are embedded) ---
# CGO is required: onnxruntime_go (optional FinBERT sentiment) wraps the ORT C
# API via cgo. The golang:1.26-bookworm image ships gcc; the runtime stage is
# glibc-based (Debian) so the dynamically-linked binary runs there. The ORT
# shared lib itself is dlopen'd lazily, so the binary still runs without it
# unless sentiment is enabled.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bourse ./cmd/bourse

# --- runtime: needs node + the Claude CLI so subscription mode can generate ---
FROM node:22-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && npm install -g @anthropic-ai/claude-code

# ONNX Runtime shared lib for in-process FinBERT (sentiment is opt-in at runtime)
ARG ORT_VERSION=1.20.1
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
 && curl -fsSL -o /tmp/ort.tgz \
    https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-x64-${ORT_VERSION}.tgz \
 && tar -xzf /tmp/ort.tgz -C /tmp \
 && cp /tmp/onnxruntime-linux-x64-${ORT_VERSION}/lib/libonnxruntime.so* /usr/local/lib/ \
 && ldconfig && rm -rf /tmp/ort.tgz /tmp/onnxruntime-linux-x64-${ORT_VERSION} \
 && apt-get purge -y curl && rm -rf /var/lib/apt/lists/*
ENV BOURSE_ORT_LIB=/usr/local/lib/libonnxruntime.so

# reuse the image's built-in node user (uid 1000) to match the host data volume
USER node
WORKDIR /app
COPY --from=build /bourse /usr/local/bin/bourse
ENV BOURSE_PORT=8080 BOURSE_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["bourse"]
