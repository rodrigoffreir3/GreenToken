# Estágio 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copia os módulos go.mod e go.sum de pb e agent
COPY pb/go.mod ./pb/
COPY agent/go.mod agent/go.sum ./agent/

WORKDIR /app/agent
RUN go mod download

# Copia o código-fonte inteiro
WORKDIR /app
COPY pb/ ./pb/
COPY agent/ ./agent/

WORKDIR /app/agent
# Compila o agente de forma estática (build stub default)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o greentoken-agent .

# Estágio 2: Final
FROM alpine:latest

WORKDIR /app

# Copia apenas o executável compilado do builder
COPY --from=builder /app/agent/greentoken-agent .

# Define o entrypoint
ENTRYPOINT ["./greentoken-agent"]
