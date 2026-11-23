FROM golang:alpine AS go-builder

ARG VERSION=0.0.7
ENV GOTOOLCHAIN=auto

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
  -ldflags="-s -w -X main.version=${VERSION}" \
  -o /out/invoicer-linux ./cmd/invoicer

FROM node:22-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build && npm prune --omit=dev

FROM node:22-alpine AS runner

LABEL org.opencontainers.image.source="https://github.com/scptric/invoicer"
LABEL org.opencontainers.image.description="Invoicer — PDF E-Invoice Extraction Engine & Accounting CSV Workbench by Scptric"
LABEL org.opencontainers.image.licenses="Apache-2.0"

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app/web

COPY --from=go-builder --chown=app:app /out/invoicer-linux /app/bin/invoicer-linux
COPY --chown=app:app config/symbols.json /app/config/symbols.json
COPY --from=web-builder --chown=app:app /src/web/package.json /app/web/package.json
COPY --from=web-builder --chown=app:app /src/web/package-lock.json /app/web/package-lock.json
COPY --from=web-builder --chown=app:app /src/web/next.config.ts /app/web/next.config.ts
COPY --from=web-builder --chown=app:app /src/web/.next /app/web/.next
COPY --from=web-builder --chown=app:app /src/web/node_modules /app/web/node_modules

ENV NODE_ENV=production \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    INVOICER_ROOT_DIR=/app \
    INVOICER_BIN_PATH=/app/bin/invoicer-linux

USER app
EXPOSE 3000

CMD ["npm", "run", "start"]
