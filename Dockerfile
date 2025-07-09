FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o k8s-exporter main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/k8s-exporter /app/k8s-exporter
COPY kube /app/kube
EXPOSE 8080
ENTRYPOINT ["/app/k8s-exporter", "--kubeconfig", "/app/kube"]
