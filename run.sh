#!/bin/bash
set -e

# ═══════════════════════════════════════════════════
#  ClipSync — Instalar como servicio macOS (LaunchAgent)
# ═══════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVICE_LABEL="com.clipsync.server"
PLIST_PATH="$HOME/Library/LaunchAgents/${SERVICE_LABEL}.plist"
BIN_DIR="$HOME/bin"
BIN_PATH="$BIN_DIR/clipsync-server"
LOG_DIR="$HOME/Library/Logs"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${CYAN}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  📋 ClipSync — Instalación de servicio macOS  ║${NC}"
echo -e "${CYAN}╚═══════════════════════════════════════════════╝${NC}"
echo ""

# ── 1. Verificar Go ──────────────────────────────
if ! command -v go &>/dev/null; then
    echo -e "${RED}[✗] Go no está instalado.${NC}"
    echo "    Instalá Go desde https://go.dev/dl/ o con: brew install go"
    exit 1
fi
echo -e "${GREEN}[✓]${NC} Go $(go version | awk '{print $3}')"

# ── 2. Compilar ──────────────────────────────────
echo -e "${YELLOW}[…]${NC} Compilando clipsync-server..."
cd "$SCRIPT_DIR/cmd"
go build -o "$SCRIPT_DIR/clipsync-server" .
echo -e "${GREEN}[✓]${NC} Binario compilado"

# ── 3. Instalar binario ─────────────────────────
mkdir -p "$BIN_DIR"
cp "$SCRIPT_DIR/clipsync-server" "$BIN_PATH"
chmod +x "$BIN_PATH"
echo -e "${GREEN}[✓]${NC} Binario instalado en $BIN_PATH"

# ── 4. Descargar servicio anterior si existe ─────
if launchctl list 2>/dev/null | grep -q "$SERVICE_LABEL"; then
    echo -e "${YELLOW}[…]${NC} Deteniendo servicio anterior..."
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
fi

# ── 5. Crear LaunchAgent plist ───────────────────
cat > "$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${BIN_PATH}</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${BIN_DIR}</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>${LOG_DIR}/clipsync-server.out.log</string>

    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/clipsync-server.err.log</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>

    <!-- Acceso al clipboard y Bluetooth (sesión GUI) -->
    <key>LimitLoadToSessionType</key>
    <string>Aqua</string>

    <key>ProcessType</key>
    <string>Interactive</string>
</dict>
</plist>
PLIST

echo -e "${GREEN}[✓]${NC} LaunchAgent creado en $PLIST_PATH"

# ── 6. Cargar servicio ───────────────────────────
launchctl load "$PLIST_PATH"
echo -e "${GREEN}[✓]${NC} Servicio cargado y arrancado"

# ── 7. Verificar ─────────────────────────────────
sleep 1
if launchctl list 2>/dev/null | grep -q "$SERVICE_LABEL"; then
    echo ""
    echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✅ ClipSync corriendo como servicio de macOS${NC}"
    echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  Dashboard:  ${CYAN}http://localhost:8066${NC}"
    echo -e "  Logs out:   $LOG_DIR/clipsync-server.out.log"
    echo -e "  Logs err:   $LOG_DIR/clipsync-server.err.log"
    echo ""
    echo -e "  ${YELLOW}Comandos útiles:${NC}"
    echo "    Detener:    launchctl unload $PLIST_PATH"
    echo "    Arrancar:   launchctl load $PLIST_PATH"
    echo "    Ver logs:   tail -f $LOG_DIR/clipsync-server.out.log"
    echo "    Desinstalar: bash $(basename "$0") --uninstall"
    echo ""
else
    echo -e "${RED}[✗] El servicio no parece estar corriendo.${NC}"
    echo "    Revisá los logs: cat $LOG_DIR/clipsync-server.err.log"
    exit 1
fi
