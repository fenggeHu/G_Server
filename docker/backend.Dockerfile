FROM golang:1.25.6 AS build
WORKDIR /src
COPY Backend/go.mod Backend/go.sum ./
RUN go mod download
COPY Backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/backend .

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/backend /app/backend
COPY Backend/migrations /app/migrations
RUN addgroup -S app && adduser -S -G app app
USER app
ENTRYPOINT ["/app/backend"]
