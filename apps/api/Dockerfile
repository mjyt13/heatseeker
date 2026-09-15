# Single image for both `api` and `worker` — pick the command at run time.
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X heatseeker/api/internal/transport/http.Version=${VERSION}" \
      -o /out/heatseeker ./cmd/heatseeker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/heatseeker /heatseeker
EXPOSE 8080
ENTRYPOINT ["/heatseeker"]
CMD ["api"]
