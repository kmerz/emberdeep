# syntax=docker/dockerfile:1
# 1) build the client
FROM node:22 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# the build script copies dist into ../internal/web/dist (an empty dir is fine here)
RUN mkdir -p /src/internal/web && npm run build

# 2) build the server with the client embedded
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/emberdeep ./cmd/emberdeep
RUN mkdir -p /out/data/saves

# 3) tiny runtime image
FROM gcr.io/distroless/static
COPY --from=build /out/emberdeep /emberdeep
# saved games: mount a volume on /data to keep them across container restarts
COPY --from=build --chown=65532:65532 /out/data /data
ENV ADDR=:8080 MAX_ROOMS=200 SAVE_DIR=/data/saves
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/emberdeep"]
