#!/bin/bash
set -e

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/greentoken"

echo "[GreenToken] Compilando o agente..."
make build-agent

echo "[GreenToken] Instalando o binário do agente em $INSTALL_DIR..."
install -m 755 ./bin/greentoken-agent $INSTALL_DIR/greentoken-agent

mkdir -p $CONFIG_DIR
chmod 755 $CONFIG_DIR

if [ ! -f "$CONFIG_DIR/agent.env" ]; then
    echo "[GreenToken] Criando arquivo de ambiente padrão em $CONFIG_DIR/agent.env..."
    cat <<EOF > "$CONFIG_DIR/agent.env"
GT_COLLECTOR_URL=localhost:50051
GT_AGENT_ID=host-agent-01
GT_WORKLOAD_NAME=vllm
GT_MODEL_NAME=llama3
GT_CPU_COUNT=4
# GT_LOG_FILE=/path/to/inference.log
EOF
    chmod 600 "$CONFIG_DIR/agent.env"
fi

echo "[GreenToken] Copiando serviço systemd..."
cp ./deploy/greentoken-agent.service /etc/systemd/system/

echo "[GreenToken] Recarregando daemon e habilitando o serviço..."
systemctl daemon-reload
systemctl enable greentoken-agent

echo "[GreenToken] Instalação concluída com sucesso."
echo "[GreenToken] Ajuste o arquivo $CONFIG_DIR/agent.env se necessário e inicie o serviço com: systemctl start greentoken-agent"
