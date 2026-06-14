# --- build the static Go binary (web assets are embedded) ---
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bourse ./cmd/bourse

# --- runtime: needs node + the Claude CLI so subscription mode can generate ---
FROM node:22-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && npm install -g @anthropic-ai/claude-code
# reuse the image's built-in node user (uid 1000) to match the host data volume
USER node
WORKDIR /app
COPY --from=build /bourse /usr/local/bin/bourse
ENV BOURSE_PORT=8080 BOURSE_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["bourse"]
