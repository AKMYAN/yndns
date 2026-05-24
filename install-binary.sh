#!/usr/bin/env bash
set -e

VERSION="${VERSION:-v0.1.0}"
ARCH="${ARCH:-Amd64}"
BIN_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/yndns"
RELEASE="https://github.com/AKMYAN/yndns/releases/download/${VERSION}/yndns-${VERSION}-Linux-${ARCH}"

echo "==> Downloading yndns ${VERSION} (linux/${ARCH})..."
curl -fsSLo /tmp/yndns "${RELEASE}"

echo "==> Installing to $BIN_DIR..."
mkdir -p "$BIN_DIR"
cp /tmp/yndns "$BIN_DIR/yndns"
chmod +x "$BIN_DIR/yndns"
rm -f /tmp/yndns

echo "==> Setting up config dir $CONFIG_DIR..."
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    echo 'token: ""' > "$CONFIG_DIR/config.yaml"
    echo "    Created default config (add your token)."
fi

if ! echo "$PATH" | grep -q "$BIN_DIR"; then
    if ! grep -q "$BIN_DIR" "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        echo "    Added $BIN_DIR to PATH in ~/.bashrc"
    fi
fi

echo "==> Done. Run 'source ~/.bashrc' or open a new terminal."
echo "    Usage: yndns 8.8.8.8"
