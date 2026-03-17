#!/bin/bash
set -e

# ═══════════════════════════════════════════════════
#  ClipSync — Instalar app Android via ADB
# ═══════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APK_PATH="$SCRIPT_DIR/client/app/build/outputs/apk/debug/app-debug.apk"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${CYAN}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║  📱 ClipSync — Instalar en Android via ADB    ║${NC}"
echo -e "${CYAN}╚═══════════════════════════════════════════════╝${NC}"
echo ""

# ── 1. Verificar ADB ────────────────────────────
if ! command -v adb &>/dev/null; then
    echo -e "${RED}[✗] adb no está instalado.${NC}"
    echo ""
    echo "  Instalá con Homebrew:"
    echo -e "    ${CYAN}brew install android-platform-tools${NC}"
    echo ""
    echo "  O descargá desde:"
    echo "    https://developer.android.com/tools/releases/platform-tools"
    exit 1
fi
echo -e "${GREEN}[✓]${NC} adb encontrado: $(which adb)"

# ── 2. Verificar dispositivo conectado ──────────
echo -e "${YELLOW}[…]${NC} Buscando dispositivos..."

# Obtener lista de serials de dispositivos físicos y emuladores
DEVICES=($(adb devices | grep -v "List" | grep "device$" | awk '{print $1}'))
DEVICE_COUNT=${#DEVICES[@]}

if [ "$DEVICE_COUNT" -eq 0 ]; then
    echo -e "${RED}[✗] No se detectó ningún dispositivo Android.${NC}"
    echo ""
    echo "  Verificá que:"
    echo "    1. El teléfono está conectado por USB"
    echo "    2. Depuración USB está habilitada"
    echo "       (Ajustes → Opciones de desarrollador → Depuración USB)"
    echo "    3. Autorizaste la conexión en el teléfono"
    echo ""
    echo "  Dispositivos detectados:"
    adb devices
    exit 1
fi

if [ "$DEVICE_COUNT" -eq 1 ]; then
    TARGET="${DEVICES[0]}"
    echo -e "${GREEN}[✓]${NC} Dispositivo detectado: $TARGET"
else
    echo -e "${YELLOW}[!]${NC} Se detectaron $DEVICE_COUNT dispositivos:"
    echo ""
    for i in "${!DEVICES[@]}"; do
        MODEL=$(adb -s "${DEVICES[$i]}" shell getprop ro.product.model 2>/dev/null || echo "desconocido")
        echo "  $((i+1))) ${DEVICES[$i]}  ($MODEL)"
    done
    echo ""
    read -p "  Elegí el número del dispositivo [1]: " CHOICE
    CHOICE=${CHOICE:-1}
    INDEX=$((CHOICE-1))
    if [ "$INDEX" -lt 0 ] || [ "$INDEX" -ge "$DEVICE_COUNT" ]; then
        echo -e "${RED}[✗] Opción inválida.${NC}"
        exit 1
    fi
    TARGET="${DEVICES[$INDEX]}"
    echo -e "${GREEN}[✓]${NC} Dispositivo seleccionado: $TARGET"
fi

# ── 3. Verificar/Compilar APK ───────────────────
if [ ! -f "$APK_PATH" ]; then
    echo -e "${YELLOW}[…]${NC} APK no encontrado. Compilando..."
    cd "$SCRIPT_DIR/client"

    if [ ! -f "gradlew" ]; then
        echo -e "${RED}[✗] No se encontró gradlew en client/.${NC}"
        exit 1
    fi

    chmod +x gradlew
    ./gradlew assembleDebug

    if [ ! -f "$APK_PATH" ]; then
        echo -e "${RED}[✗] La compilación no generó el APK esperado.${NC}"
        exit 1
    fi
    echo -e "${GREEN}[✓]${NC} APK compilado"
else
    APK_SIZE=$(du -h "$APK_PATH" | awk '{print $1}')
    echo -e "${GREEN}[✓]${NC} APK encontrado ($APK_SIZE)"
fi

# ── 4. Instalar ─────────────────────────────────
echo -e "${YELLOW}[…]${NC} Instalando en $TARGET..."
adb -s "$TARGET" install -r "$APK_PATH"

echo ""
echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ✅ ClipSync instalado en Android${NC}"
echo -e "${GREEN}══════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Abrí la app ${CYAN}ClipSync${NC} en tu teléfono"
echo -e "  y escaneá el QR del dashboard: ${CYAN}http://localhost:8066${NC}"
echo ""
