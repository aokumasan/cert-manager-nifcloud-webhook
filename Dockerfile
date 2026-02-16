FROM golang:1.25-alpine AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace

COPY go.mod go.sum ./
COPY third_party ./third_party

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM gcr.io/distroless/static:nonroot

COPY --from=build /workspace/webhook /bin/webhook

ENTRYPOINT ["/bin/webhook"]
