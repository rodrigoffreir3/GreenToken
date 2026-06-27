# Estágio 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copia os módulos go.mod e go.sum de pb e collector
COPY pb/go.mod ./pb/
COPY collector/go.mod collector/go.sum ./collector/

WORKDIR /app/collector
RUN go mod download

# Copia o código-fonte inteiro
WORKDIR /app
COPY pb/ ./pb/
COPY collector/ ./collector/

WORKDIR /app/collector
# Compila o collector de forma estática
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o greentoken-collector .

# Estágio 2: Final
FROM alpine:latest

WORKDIR /app

# Copia o executável do estágio de build
COPY --from=builder /app/collector/greentoken-collector .

# Expõe as portas gRPC e Prometheus
EXPOSE 50051
EXPOSE 2112

# O comando para rodar o executável
CMD ["./greentoken-collector"]
