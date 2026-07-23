#!/bin/bash
set -euo pipefail

REPO="rodrigoffreir3/GreenToken"
PREFIX="/usr/local/bin"
VARIANT="stub"
VERSION=""

# Parse de argumentos
while [[ $# -gt 0 ]]; do
  case "$1" in
    --gpu)
      VARIANT="gpu"
      shift
      ;;
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    *)
      echo "Opção desconhecida: $1"
      echo "Uso: $0 [--gpu] [--prefix /usr/local/bin] [--version v0.1.0]"
      exit 1
      ;;
  esac
done

# Detecção de OS e Arquitetura
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

if [ "$OS" != "linux" ]; then
    echo "[ERRO] GreenToken é suportado nativamente apenas em Linux (detectado: $OS)."
    exit 1
fi

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "[ERRO] Arquitetura $ARCH não suportada."
        exit 1
        ;;
esac

# Tratamento da variante GPU (somente amd64)
VARIANT_SUFFIX=""
if [ "$VARIANT" = "gpu" ]; then
    if [ "$ARCH" != "amd64" ]; then
        echo "[AVISO] Suporte a GPU (-gpu) disponível apenas para linux/amd64 nesta versão. Usando variante stub."
    else
        VARIANT_SUFFIX="-gpu"
    fi
fi

# Determina versão a baixar
if [ -z "$VERSION" ]; then
    echo "[GreenToken] Buscando última versão lançada..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "v0.1.0")
fi

TARBALL="greentoken_${VERSION}_${OS}_${ARCH}${VARIANT_SUFFIX}.tar.gz"
SHA256FILE="${TARBALL}.sha256"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "[GreenToken] Baixando GreenToken $VERSION ($OS/$ARCH$VARIANT_SUFFIX)..."
curl -fsSL "${DOWNLOAD_URL}/${TARBALL}" -o "${TMPDIR}/${TARBALL}"
curl -fsSL "${DOWNLOAD_URL}/${SHA256FILE}" -o "${TMPDIR}/${SHA256FILE}"

echo "[GreenToken] Validando integridade SHA256 (Zero-Trust)..."
cd "$TMPDIR"
if ! sha256sum -c "${SHA256FILE}"; then
    echo "[ERRO CRÍTICO] Checksum SHA256 não confere!"
    echo "O arquivo baixado pode ter sido corrompido ou adulterado em trânsito. Instalação abortada."
    exit 1
fi
echo "[GreenToken] ✅ Checksum SHA256 verificado com sucesso."

# Extrai e instala
echo "[GreenToken] Instalando binários em $PREFIX..."
tar -xzf "${TARBALL}" -C "$TMPDIR"

mkdir -p "$PREFIX"
for bin in greentoken greentoken-agent greentoken-collector; do
    # Procura o binário extraído que pode ter sufixos de OS/ARCH
    FOUND=$(find "$TMPDIR" -maxdepth 2 -type f -name "${bin}*" ! -name "*.tar.gz*" ! -name "*.sha256" | head -n 1)
    if [ -n "$FOUND" ]; then
        install -m 755 "$FOUND" "${PREFIX}/${bin}"
        echo "  - Instalado: ${PREFIX}/${bin}"
    fi
done

echo ""
echo "[GreenToken] 🎉 Instalação concluída com sucesso!"
echo "[GreenToken] Para diagnosticar o ambiente e validar permissões/recursos, rode:"
echo "  greentoken doctor"
