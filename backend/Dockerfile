# --- build ---
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server && \
    CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/server /bin/server
COPY --from=build /bin/worker /bin/worker
EXPOSE 8080
ENTRYPOINT ["/bin/server"]
