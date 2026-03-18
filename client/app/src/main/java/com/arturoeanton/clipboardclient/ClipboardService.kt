package com.arturoeanton.clipboardclient

import android.app.*
import android.bluetooth.*
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.ParcelUuid
import android.util.Log
import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.zip.CRC32

class ClipboardService : Service() {

    companion object {
        private const val TAG = "ClipSync"
        private const val NOTIFICATION_ID = 1
        private const val CHANNEL_ID = "clipsync_channel"

        val SERVICE_UUID: UUID = UUID.fromString("12345678-1234-5678-1234-56789abcdef0")
        val CONTENT_UUID: UUID = UUID.fromString("12345678-1234-5678-1234-56789abcdef1")
        val HASH_UUID: UUID = UUID.fromString("12345678-1234-5678-1234-56789abcdef2")
        val PAIRING_UUID: UUID = UUID.fromString("12345678-1234-5678-1234-56789abcdef3")
        val CCC_DESCRIPTOR_UUID: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
        private const val PREFS_NAME = "clipsync_prefs"
        private const val PREF_TOKEN = "pairing_token"

        var instance: ClipboardService? = null
            private set

        /** Enviar texto a los desktops via BLE desde cualquier parte de la app */
        fun sendToMac(text: String) {
            val svc = instance ?: run {
                Log.w(TAG, "Servicio no activo, no se puede enviar")
                return
            }
            svc.lastClipContent = text
            svc.lastClipHash = svc.crc32(text)
            svc.notifyDesktops()
            Log.i(TAG, "[Android → Desktops] Enviado via Share (${text.length} chars)")
        }

        /** Devuelve el último contenido cacheado del clipboard (fallback) */
        fun sendToMac_getLastContent(): String? {
            return instance?.lastClipContent?.ifBlank { null }
        }
    }

    private var gattServer: BluetoothGattServer? = null
    private var bluetoothManager: BluetoothManager? = null
    private var clipboardManager: ClipboardManager? = null
    private var lastClipHash: Long = 0
    private var lastClipContent: String = ""

    // Multi-device: set of connected desktops
    private val connectedDevices: MutableSet<BluetoothDevice> =
        ConcurrentHashMap.newKeySet()

    private var contentCharacteristic: BluetoothGattCharacteristic? = null
    private var hashCharacteristic: BluetoothGattCharacteristic? = null
    private val chunkBuffer = mutableMapOf<Int, ByteArray>() // Para reassembly de chunks
    private var expectedChunks = 0

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        instance = this
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification("Iniciando..."))

        bluetoothManager = getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager
        clipboardManager = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager

        setupGattServer()
        startAdvertising()
        startClipboardWatcher()

        Log.i(TAG, "Servicio iniciado")
    }

    override fun onDestroy() {
        super.onDestroy()
        try {
            bluetoothManager?.adapter?.bluetoothLeAdvertiser?.stopAdvertising(advertiseCallback)
            gattServer?.close()
        } catch (e: SecurityException) {
            Log.e(TAG, "Error limpiando BLE: ${e.message}")
        }
        connectedDevices.clear()
        Log.i(TAG, "Servicio detenido")
    }

    // === GATT Server Setup ===

    private fun setupGattServer() {
        try {
            gattServer = bluetoothManager?.openGattServer(this, gattCallback)

            val service = BluetoothGattService(SERVICE_UUID, BluetoothGattService.SERVICE_TYPE_PRIMARY)

            // Clipboard Content characteristic (Read + Write + Notify)
            contentCharacteristic = BluetoothGattCharacteristic(
                CONTENT_UUID,
                BluetoothGattCharacteristic.PROPERTY_READ or
                        BluetoothGattCharacteristic.PROPERTY_WRITE or
                        BluetoothGattCharacteristic.PROPERTY_NOTIFY,
                BluetoothGattCharacteristic.PERMISSION_READ or
                        BluetoothGattCharacteristic.PERMISSION_WRITE
            ).apply {
                addDescriptor(BluetoothGattDescriptor(
                    CCC_DESCRIPTOR_UUID,
                    BluetoothGattDescriptor.PERMISSION_READ or BluetoothGattDescriptor.PERMISSION_WRITE
                ))
            }

            // Clipboard Hash characteristic (Read + Notify)
            hashCharacteristic = BluetoothGattCharacteristic(
                HASH_UUID,
                BluetoothGattCharacteristic.PROPERTY_READ or
                        BluetoothGattCharacteristic.PROPERTY_NOTIFY,
                BluetoothGattCharacteristic.PERMISSION_READ
            ).apply {
                addDescriptor(BluetoothGattDescriptor(
                    CCC_DESCRIPTOR_UUID,
                    BluetoothGattDescriptor.PERMISSION_READ or BluetoothGattDescriptor.PERMISSION_WRITE
                ))
            }

            // Pairing Token characteristic (Read only — desktops read it to validate)
            val pairingCharacteristic = BluetoothGattCharacteristic(
                PAIRING_UUID,
                BluetoothGattCharacteristic.PROPERTY_READ,
                BluetoothGattCharacteristic.PERMISSION_READ
            )

            service.addCharacteristic(contentCharacteristic)
            service.addCharacteristic(hashCharacteristic)
            service.addCharacteristic(pairingCharacteristic)
            gattServer?.addService(service)

            Log.i(TAG, "GATT Server configurado")
        } catch (e: SecurityException) {
            Log.e(TAG, "Permisos BLE insuficientes: ${e.message}")
        }
    }

    private val gattCallback = object : BluetoothGattServerCallback() {
        override fun onConnectionStateChange(device: BluetoothDevice, status: Int, newState: Int) {
            if (newState == BluetoothGattServer.STATE_CONNECTED) {
                connectedDevices.add(device)
                val count = connectedDevices.size
                Log.i(TAG, "Desktop conectado: ${device.address} (total: $count)")
                updateNotification("$count desktop(s) conectado(s)")
            } else {
                connectedDevices.remove(device)
                val count = connectedDevices.size
                Log.i(TAG, "Desktop desconectado: ${device.address} (total: $count)")
                if (count > 0) {
                    updateNotification("$count desktop(s) conectado(s)")
                } else {
                    updateNotification("Esperando conexión...")
                }
            }
        }

        override fun onCharacteristicReadRequest(
            device: BluetoothDevice, requestId: Int, offset: Int,
            characteristic: BluetoothGattCharacteristic
        ) {
            try {
                when (characteristic.uuid) {
                    CONTENT_UUID -> {
                        val data = lastClipContent.toByteArray()
                        val chunk = if (offset < data.size) data.copyOfRange(offset, minOf(data.size, offset + 512)) else byteArrayOf()
                        gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, offset, chunk)
                    }
                    HASH_UUID -> {
                        val buf = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(lastClipHash.toInt()).array()
                        gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, buf)
                    }
                    PAIRING_UUID -> {
                        val token = getPairingToken()
                        if (token.isNotEmpty()) {
                            val data = token.toByteArray()
                            gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, data)
                            Log.i(TAG, "Token de pairing enviado a ${device.address} (${token.take(8)}...)")
                        } else {
                            gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, byteArrayOf())
                            Log.w(TAG, "No hay token de pairing guardado")
                        }
                    }
                    else -> {
                        gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_FAILURE, 0, null)
                    }
                }
            } catch (e: SecurityException) {
                Log.e(TAG, "Error en read: ${e.message}")
            }
        }

        override fun onCharacteristicWriteRequest(
            device: BluetoothDevice, requestId: Int,
            characteristic: BluetoothGattCharacteristic, preparedWrite: Boolean,
            responseNeeded: Boolean, offset: Int, value: ByteArray
        ) {
            try {
                if (characteristic.uuid == CONTENT_UUID && value.size >= 2) {
                    val chunkIndex = value[0].toInt() and 0xFF
                    val totalChunks = value[1].toInt() and 0xFF
                    val data = value.copyOfRange(2, value.size)

                    if (totalChunks <= 1) {
                        // Texto corto — un solo chunk
                        val text = String(data)
                        if (text.isNotBlank()) {
                            Log.i(TAG, "[Desktop ${device.address} → Android] Recibido (${text.length} chars)")
                            setAndroidClipboard(text)
                            // Relay to other connected desktops
                            notifyDesktops(excludeDevice = device)
                        }
                    } else {
                        // Texto largo — buffering de chunks
                        chunkBuffer[chunkIndex] = data
                        expectedChunks = totalChunks
                        Log.d(TAG, "Chunk ${chunkIndex+1}/$totalChunks recibido (${data.size} bytes)")

                        if (chunkBuffer.size >= totalChunks) {
                            // Todos los chunks recibidos — reassemblar
                            val full = ByteArray(chunkBuffer.values.sumOf { it.size })
                            var pos = 0
                            for (i in 0 until totalChunks) {
                                val chunk = chunkBuffer[i] ?: continue
                                System.arraycopy(chunk, 0, full, pos, chunk.size)
                                pos += chunk.size
                            }
                            chunkBuffer.clear()
                            val text = String(full, 0, pos)
                            Log.i(TAG, "[Desktop ${device.address} → Android] Recibido completo (${text.length} chars, $totalChunks chunks)")
                            setAndroidClipboard(text)
                            // Relay to other connected desktops
                            notifyDesktops(excludeDevice = device)
                        }
                    }
                }
                if (responseNeeded) {
                    gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, null)
                }
            } catch (e: SecurityException) {
                Log.e(TAG, "Error en write: ${e.message}")
            }
        }

        override fun onDescriptorWriteRequest(
            device: BluetoothDevice, requestId: Int,
            descriptor: BluetoothGattDescriptor, preparedWrite: Boolean,
            responseNeeded: Boolean, offset: Int, value: ByteArray
        ) {
            try {
                // Cliente habilitando/deshabilitando notificaciones
                if (responseNeeded) {
                    gattServer?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, 0, null)
                }
                Log.i(TAG, "Notificaciones configuradas para ${descriptor.characteristic.uuid} (${device.address})")
            } catch (e: SecurityException) {
                Log.e(TAG, "Error en descriptor write: ${e.message}")
            }
        }
    }

    // === BLE Advertising ===

    private fun startAdvertising() {
        try {
            val advertiser = bluetoothManager?.adapter?.bluetoothLeAdvertiser ?: run {
                Log.e(TAG, "BLE Advertiser no disponible")
                return
            }

            val settings = AdvertiseSettings.Builder()
                .setAdvertiseMode(AdvertiseSettings.ADVERTISE_MODE_LOW_LATENCY)
                .setConnectable(true)
                .setTxPowerLevel(AdvertiseSettings.ADVERTISE_TX_POWER_HIGH)
                .build()

            val data = AdvertiseData.Builder()
                .setIncludeDeviceName(false)
                .addServiceUuid(ParcelUuid(SERVICE_UUID))
                .build()

            val scanResponse = AdvertiseData.Builder()
                .setIncludeDeviceName(true)
                .build()

            advertiser.startAdvertising(settings, data, scanResponse, advertiseCallback)
            Log.i(TAG, "Advertising iniciado")
        } catch (e: SecurityException) {
            Log.e(TAG, "Permisos para advertising insuficientes: ${e.message}")
        }
    }

    private val advertiseCallback = object : AdvertiseCallback() {
        override fun onStartSuccess(settingsInEffect: AdvertiseSettings?) {
            Log.i(TAG, "Advertising activo como 'ClipSync'")
            updateNotification("Advertising BLE activo")
        }

        override fun onStartFailure(errorCode: Int) {
            Log.e(TAG, "Error advertising: $errorCode")
            updateNotification("Error BLE: $errorCode")
        }
    }

    // === Clipboard ===

    private val clipboardHandler = android.os.Handler(android.os.Looper.getMainLooper())
    private val clipboardPoller = object : Runnable {
        override fun run() {
            try {
                val clip = clipboardManager?.primaryClip
                if (clip != null && clip.itemCount > 0) {
                    val text = clip.getItemAt(0).text?.toString()
                    if (text != null && text.isNotBlank()) {
                        val hash = crc32(text)
                        if (hash != lastClipHash) {
                            lastClipContent = text
                            lastClipHash = hash
                            Log.i(TAG, "[Android → Desktops] Cambio detectado (${text.length} chars): ${text.take(50)}")
                            notifyDesktops()
                        }
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "Error polling clipboard: ${e.message}")
            }
            clipboardHandler.postDelayed(this, 750)
        }
    }

    private fun startClipboardWatcher() {
        // Leer contenido inicial
        try {
            val clip = clipboardManager?.primaryClip
            if (clip != null && clip.itemCount > 0) {
                val text = clip.getItemAt(0).text?.toString()
                if (!text.isNullOrBlank()) {
                    lastClipContent = text
                    lastClipHash = crc32(text)
                    Log.i(TAG, "Clipboard inicial: ${text.take(30)}... (${text.length} chars)")
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error leyendo clipboard inicial: ${e.message}")
        }

        // Iniciar polling cada 750ms
        clipboardHandler.post(clipboardPoller)
        Log.i(TAG, "Clipboard poller iniciado (750ms)")
    }

    private fun setAndroidClipboard(text: String) {
        lastClipContent = text
        lastClipHash = crc32(text)
        clipboardManager?.setPrimaryClip(ClipData.newPlainText("ClipSync", text))
    }

    fun getPairingToken(): String {
        return getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
            .getString(PREF_TOKEN, "") ?: ""
    }

    fun savePairingToken(token: String) {
        getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
            .edit().putString(PREF_TOKEN, token).apply()
        Log.i(TAG, "Token de pairing guardado: ${token.take(8)}...")
    }

    /**
     * Notifica a todos los desktops conectados del cambio de clipboard.
     * @param excludeDevice Si no es null, salta ese device (el que envió el texto, para evitar loops).
     */
    private fun notifyDesktops(excludeDevice: BluetoothDevice? = null) {
        if (connectedDevices.isEmpty()) return

        val devices = connectedDevices.toSet() // Snapshot thread-safe
        val targetCount = if (excludeDevice != null) devices.size - 1 else devices.size

        if (targetCount <= 0 && excludeDevice != null) {
            Log.d(TAG, "No hay otros desktops para relay")
            return
        }

        try {
            for (device in devices) {
                if (device == excludeDevice) continue
                try {
                    // Notificar hash
                    hashCharacteristic?.let { char ->
                        char.value = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(lastClipHash.toInt()).array()
                        gattServer?.notifyCharacteristicChanged(device, char, false)
                    }
                    // Notificar contenido
                    contentCharacteristic?.let { char ->
                        val data = lastClipContent.toByteArray()
                        char.value = if (data.size > 512) data.copyOfRange(0, 512) else data
                        gattServer?.notifyCharacteristicChanged(device, char, false)
                    }
                    Log.d(TAG, "Notificado: ${device.address}")
                } catch (e: SecurityException) {
                    Log.e(TAG, "Error notificando ${device.address}: ${e.message}")
                }
            }

            if (excludeDevice != null) {
                Log.i(TAG, "[Relay] Clipboard reenviado a $targetCount desktop(s)")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error en notifyDesktops: ${e.message}")
        }
    }

    private fun crc32(text: String): Long {
        val crc = CRC32()
        crc.update(text.toByteArray())
        return crc.value
    }

    // === Notifications ===

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID, "ClipSync", NotificationManager.IMPORTANCE_LOW
            ).apply { description = "Sincronización de clipboard" }
            (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager)
                .createNotificationChannel(channel)
        }
    }

    private fun buildNotification(text: String): Notification {
        val sendIntent = android.content.Intent(this, SendClipboardActivity::class.java)
        val sendPending = android.app.PendingIntent.getActivity(
            this, 0, sendIntent,
            android.app.PendingIntent.FLAG_UPDATE_CURRENT or android.app.PendingIntent.FLAG_IMMUTABLE
        )

        return Notification.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setContentTitle("ClipSync")
            .setContentText(text)
            .setOngoing(true)
            .addAction(Notification.Action.Builder(
                null, "Enviar Clipboard", sendPending
            ).build())
            .build()
    }

    private fun updateNotification(text: String) {
        try {
            val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            nm.notify(NOTIFICATION_ID, buildNotification(text))
        } catch (_: Exception) {}
    }
}
