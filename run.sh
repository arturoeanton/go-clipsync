#!/bin/bash
set -e

# ═══════════════════════════════════════════════════
#  ClipSync — Instalar como servicio (macOS / Linux)
# ═══════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$HOME/bin"
BIN_PATH="$BIN_DIR/clipsync-server"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

OS="$(uname -s)"

echo ""
echo -e "${CYAN}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  📋 ClipSync — Instalación de servicio        ║${NC}"
echo -e "${CYAN}╚═══════════════════════════════════════════════╝${NC}"
echo ""

# ── 1. Verificar Go ──────────────────────────────
if ! command -v go &>/dev/null; then
    echo -e "${RED}[✗] Go no está instalado.${NC}"
    echo "    Instalá Go desde https://go.dev/dl/ o con tu package manager"
    exit 1
fi
echo -e "${GREEN}[✓]${NC} Go $(go version | awk '{print $3}')"

# ── 2. Verificar dependencias de clipboard (Linux) ─
if [ "$OS" = "Linux" ]; then
    if ! command -v xclip &>/dev/null && ! command -v xsel &>/dev/null; then
        echo -e "${YELLOW}[!] Se recomienda instalar xclip o xsel para soporte de clipboard${NC}"
        echo "    sudo apt install xclip   (Debian/Ubuntu)"
        echo "    sudo pacman -S xclip     (Arch)"
        echo "    sudo dnf install xclip   (Fedora)"
    else
        CLIP_TOOL=""
        command -v xclip &>/dev/null && CLIP_TOOL="xclip"
        command -v xsel &>/dev/null && CLIP_TOOL="xsel"
        echo -e "${GREEN}[✓]${NC} Clipboard tool: $CLIP_TOOL"
    fi
fi

# ── 3. Compilar ──────────────────────────────────
echo -e "${YELLOW}[…]${NC} Compilando clipsync-server..."
cd "$SCRIPT_DIR/cmd"
go build -o "$SCRIPT_DIR/clipsync-server" .
echo -e "${GREEN}[✓]${NC} Binario compilado"

# ── 4. Instalar binario ─────────────────────────
mkdir -p "$BIN_DIR"
cp "$SCRIPT_DIR/clipsync-server" "$BIN_PATH"
chmod +x "$BIN_PATH"
echo -e "${GREEN}[✓]${NC} Binario instalado en $BIN_PATH"

# ══════════════════════════════════════════════════
#  Instalación específica por OS
# ══════════════════════════════════════════════════

if [ "$OS" = "Darwin" ]; then
    # ── macOS: LaunchAgent ───────────────────────
    SERVICE_LABEL="com.clipsync.server"
    PLIST_PATH="$HOME/Library/LaunchAgents/${SERVICE_LABEL}.plist"
    LOG_DIR="$HOME/Library/Logs"

    # Descargar servicio anterior si existe
    if launchctl list 2>/dev/null | grep -q "$SERVICE_LABEL"; then
        echo -e "${YELLOW}[…]${NC} Deteniendo servicio anterior..."
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
    fi

    # Crear LaunchAgent plist
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

    # Cargar servicio
    launchctl load "$PLIST_PATH"
    echo -e "${GREEN}[✓]${NC} Servicio cargado y arrancado"

    # Verificar
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
        echo ""
    else
        echo -e "${RED}[✗] El servicio no parece estar corriendo.${NC}"
        echo "    Revisá los logs: cat $LOG_DIR/clipsync-server.err.log"
        exit 1
    fi

elif [ "$OS" = "Linux" ]; then
    # ── Linux: systemd user service ──────────────
    SYSTEMD_DIR="$HOME/.config/systemd/user"
    SERVICE_FILE="$SYSTEMD_DIR/clipsync.service"
    LOG_DIR="$HOME/.local/share/clipsync"

    mkdir -p "$SYSTEMD_DIR"
    mkdir -p "$LOG_DIR"

    # Detener servicio anterior si existe
    if systemctl --user is-active clipsync.service &>/dev/null; then
        echo -e "${YELLOW}[…]${NC} Deteniendo servicio anterior..."
        systemctl --user stop clipsync.service 2>/dev/null || true
    fi

    # Crear unit file
    cat > "$SERVICE_FILE" <<UNIT
[Unit]
Description=ClipSync — Universal Clipboard Sync
After=graphical-session.target bluetooth.target

[Service]
Type=simple
ExecStart=${BIN_PATH}
WorkingDirectory=${BIN_DIR}
Restart=on-failure
RestartSec=5
Environment=DISPLAY=:0
Environment=PATH=/usr/local/bin:/usr/bin:/bin

StandardOutput=append:${LOG_DIR}/clipsync.out.log
StandardError=append:${LOG_DIR}/clipsync.err.log

[Install]
WantedBy=default.target
UNIT

    echo -e "${GREEN}[✓]${NC} Systemd unit creado en $SERVICE_FILE"

    # Recargar, habilitar y arrancar
    systemctl --user daemon-reload
    systemctl --user enable clipsync.service
    systemctl --user start clipsync.service
    echo -e "${GREEN}[✓]${NC} Servicio habilitado y arrancado"

    sleep 1
    if systemctl --user is-active clipsync.service &>/dev/null; then
        echo ""
        echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
        echo -e "${GREEN}  ✅ ClipSync corriendo como servicio de Linux${NC}"
        echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
        echo ""
        echo -e "  Dashboard:  ${CYAN}http://localhost:8066${NC}"
        echo -e "  Logs out:   $LOG_DIR/clipsync.out.log"
        echo -e "  Logs err:   $LOG_DIR/clipsync.err.log"
        echo ""
        echo -e "  ${YELLOW}Comandos útiles:${NC}"
        echo "    Detener:    systemctl --user stop clipsync"
        echo "    Arrancar:   systemctl --user start clipsync"
        echo "    Estado:     systemctl --user status clipsync"
        echo "    Ver logs:   journalctl --user -u clipsync -f"
        echo "    Desinstalar: systemctl --user disable --now clipsync"
        echo ""
    else
        echo -e "${RED}[✗] El servicio no parece estar corriendo.${NC}"
        echo "    Revisá los logs: cat $LOG_DIR/clipsync.err.log"
        echo "    O usá: systemctl --user status clipsync"
        exit 1
    fi
else
    echo -e "${RED}[✗] OS no soportado: $OS${NC}"
    echo "    ClipSync soporta macOS y Linux."
    exit 1
fi
