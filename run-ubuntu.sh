#!/bin/bash
set -e

# ═══════════════════════════════════════════════════
#  ClipSync — Instalar como servicio en Ubuntu/Linux
# ═══════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$HOME/bin"
BIN_PATH="$BIN_DIR/clipsync-server"
SYSTEMD_DIR="$HOME/.config/systemd/user"
SERVICE_FILE="$SYSTEMD_DIR/clipsync.service"
LOG_DIR="$HOME/.local/share/clipsync"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${CYAN}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  ClipSync — Instalación de servicio Ubuntu     ║${NC}"
echo -e "${CYAN}╚═══════════════════════════════════════════════╝${NC}"
echo ""

# ── 1. Instalar dependencias del sistema ─────────
echo -e "${YELLOW}[…]${NC} Verificando dependencias del sistema..."

PACKAGES_TO_INSTALL=""

dpkg -s bluez &>/dev/null          || PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL bluez"
dpkg -s libbluetooth-dev &>/dev/null || PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL libbluetooth-dev"
dpkg -s libdbus-1-dev &>/dev/null  || PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL libdbus-1-dev"
dpkg -s xclip &>/dev/null          || PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL xclip"
dpkg -s pkg-config &>/dev/null     || PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL pkg-config"

if [ -n "$PACKAGES_TO_INSTALL" ]; then
    echo -e "${YELLOW}[…]${NC} Instalando paquetes:${PACKAGES_TO_INSTALL}"
    sudo apt update -qq
    sudo apt install -y $PACKAGES_TO_INSTALL
    echo -e "${GREEN}[✓]${NC} Paquetes instalados"
else
    echo -e "${GREEN}[✓]${NC} Todas las dependencias ya están instaladas"
fi

# ── 2. Verificar Bluetooth activo ────────────────
if ! systemctl is-active bluetooth &>/dev/null; then
    echo -e "${YELLOW}[…]${NC} Activando servicio Bluetooth..."
    sudo systemctl enable --now bluetooth
fi
echo -e "${GREEN}[✓]${NC} Bluetooth activo"

# ── 3. Verificar Go ──────────────────────────────
if ! command -v go &>/dev/null; then
    echo -e "${RED}[✗] Go no está instalado.${NC}"
    echo "    Instalá Go desde https://go.dev/dl/ o con:"
    echo "    sudo snap install go --classic"
    exit 1
fi
echo -e "${GREEN}[✓]${NC} Go $(go version | awk '{print $3}')"

# ── 4. Compilar ──────────────────────────────────
echo -e "${YELLOW}[…]${NC} Compilando clipsync-server..."
cd "$SCRIPT_DIR/cmd"
CGO_ENABLED=1 go build -o "$SCRIPT_DIR/clipsync-server" .
echo -e "${GREEN}[✓]${NC} Binario compilado"

# ── 5. Instalar binario ─────────────────────────
mkdir -p "$BIN_DIR"
cp "$SCRIPT_DIR/clipsync-server" "$BIN_PATH"
chmod +x "$BIN_PATH"
echo -e "${GREEN}[✓]${NC} Binario instalado en $BIN_PATH"

# ── 6. Detener servicio anterior si existe ───────
if systemctl --user is-active clipsync.service &>/dev/null; then
    echo -e "${YELLOW}[…]${NC} Deteniendo servicio anterior..."
    systemctl --user stop clipsync.service 2>/dev/null || true
fi

# ── 7. Crear directorios ────────────────────────
mkdir -p "$SYSTEMD_DIR"
mkdir -p "$LOG_DIR"

# ── 8. Crear unit file de systemd ────────────────
cat > "$SERVICE_FILE" <<UNIT
[Unit]
Description=ClipSync — Universal Clipboard Sync via BLE
After=graphical-session.target bluetooth.target
Wants=bluetooth.target

[Service]
Type=simple
ExecStart=${BIN_PATH}
WorkingDirectory=${BIN_DIR}
Restart=on-failure
RestartSec=5

# Necesario para acceso al clipboard y D-Bus
Environment=DISPLAY=:0
Environment=XAUTHORITY=%h/.Xauthority
Environment=DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%U/bus
Environment=PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin:/snap/bin:%h/go/bin

StandardOutput=append:${LOG_DIR}/clipsync.out.log
StandardError=append:${LOG_DIR}/clipsync.err.log

[Install]
WantedBy=default.target
UNIT

echo -e "${GREEN}[✓]${NC} Systemd unit creado en $SERVICE_FILE"

# ── 9. Habilitar lingering (para que corra sin sesión activa) ──
if ! loginctl show-user "$USER" 2>/dev/null | grep -q "Linger=yes"; then
    echo -e "${YELLOW}[…]${NC} Habilitando lingering para usuario $USER..."
    sudo loginctl enable-linger "$USER" 2>/dev/null || true
fi

# ── 10. Recargar, habilitar y arrancar ───────────
systemctl --user daemon-reload
systemctl --user enable clipsync.service
systemctl --user start clipsync.service
echo -e "${GREEN}[✓]${NC} Servicio habilitado y arrancado"

# ── 11. Verificar ────────────────────────────────
sleep 2
if systemctl --user is-active clipsync.service &>/dev/null; then
    echo ""
    echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  ClipSync corriendo como servicio de Linux${NC}"
    echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  Dashboard:  ${CYAN}http://localhost:8066${NC}"
    echo -e "  Logs out:   $LOG_DIR/clipsync.out.log"
    echo -e "  Logs err:   $LOG_DIR/clipsync.err.log"
    echo ""
    echo -e "  ${YELLOW}Comandos útiles:${NC}"
    echo "    Detener:      systemctl --user stop clipsync"
    echo "    Arrancar:     systemctl --user start clipsync"
    echo "    Estado:       systemctl --user status clipsync"
    echo "    Ver logs:     journalctl --user -u clipsync -f"
    echo "    Recompilar:   bash $(basename "$0")"
    echo "    Desinstalar:  systemctl --user disable --now clipsync"
    echo ""
    echo -e "  ${CYAN}El servicio arranca automáticamente al iniciar sesión.${NC}"
    echo ""
else
    echo ""
    echo -e "${RED}[✗] El servicio no parece estar corriendo.${NC}"
    echo ""
    echo "  Revisá los logs:"
    echo "    cat $LOG_DIR/clipsync.err.log"
    echo "    systemctl --user status clipsync"
    echo ""
    echo "  Si el error es de Bluetooth, verificá:"
    echo "    hciconfig              # ver adaptador"
    echo "    sudo hciconfig hci0 up # activar adaptador"
    echo "    bluetoothctl show      # verificar BlueZ"
    echo ""
    exit 1
fi
