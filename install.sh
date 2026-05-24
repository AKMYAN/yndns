#!/usr/bin/env bash
set -e

BIN_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/yndns"

echo "==> Building yndns..."
go build -o yndns .

echo "==> Installing to $BIN_DIR..."
mkdir -p "$BIN_DIR"
cp yndns "$BIN_DIR/yndns"
chmod +x "$BIN_DIR/yndns"

echo "==> Setting up config dir $CONFIG_DIR..."
mkdir -p "$CONFIG_DIR"
if [ -f config.yaml ]; then
    cp config.yaml "$CONFIG_DIR/config.yaml"
    echo "    config.yaml copied."
else
    echo "    No config.yaml found, skipping."
fi

if ! echo "$PATH" | grep -q "$BIN_DIR"; then
    if ! grep -q "$BIN_DIR" "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        echo "    Added $BIN_DIR to PATH in ~/.bashrc"
    fi
fi

echo "==> Done. Run 'source ~/.bashrc' or open a new terminal."
echo "    Usage: yndns 8.8.8.8"
