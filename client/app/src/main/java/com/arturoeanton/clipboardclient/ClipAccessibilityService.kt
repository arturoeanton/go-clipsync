package com.arturoeanton.clipboardclient

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.ClipboardManager
import android.content.Context
import android.util.Log
import android.view.accessibility.AccessibilityEvent

/**
 * Accessibility Service que detecta cambios de clipboard en background.
 * Esto bypasa la restricción de Android 10+ que impide leer clipboard
 * desde apps en background.
 *
 * El usuario debe habilitar esta opción en:
 * Ajustes → Accesibilidad → ClipSync
 */
class ClipAccessibilityService : AccessibilityService() {

    companion object {
        private const val TAG = "ClipSync"
        var isRunning = false
            private set
    }

    private var clipboardManager: ClipboardManager? = null
    private var lastHash: Long = 0

    override fun onServiceConnected() {
        super.onServiceConnected()
        isRunning = true

        clipboardManager = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager

        // Leer clipboard actual
        readCurrentClipboard()

        // Registrar listener — funciona desde Accessibility Service
        clipboardManager?.addPrimaryClipChangedListener {
            onClipboardChanged()
        }

        serviceInfo = AccessibilityServiceInfo().apply {
            eventTypes = AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED or
                    AccessibilityEvent.TYPE_VIEW_TEXT_CHANGED
            feedbackType = AccessibilityServiceInfo.FEEDBACK_GENERIC
            flags = AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS
            notificationTimeout = 100
        }

        Log.i(TAG, "✅ Accessibility Service conectado — clipboard sync automático activo")
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        // No necesitamos procesar eventos de UI, solo lo usamos para clipboard access
    }

    override fun onInterrupt() {
        Log.w(TAG, "Accessibility Service interrumpido")
    }

    override fun onDestroy() {
        super.onDestroy()
        isRunning = false
        Log.i(TAG, "Accessibility Service detenido")
    }

    private fun readCurrentClipboard() {
        try {
            val clip = clipboardManager?.primaryClip
            if (clip != null && clip.itemCount > 0) {
                val text = clip.getItemAt(0).text?.toString()
                if (!text.isNullOrBlank()) {
                    lastHash = crc32(text)
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error leyendo clipboard inicial: ${e.message}")
        }
    }

    private fun onClipboardChanged() {
        try {
            val clip = clipboardManager?.primaryClip ?: return
            if (clip.itemCount == 0) return

            val text = clip.getItemAt(0).text?.toString()
            if (text.isNullOrBlank()) return

            val hash = crc32(text)
            if (hash == lastHash) return

            lastHash = hash
            Log.i(TAG, "[Accessibility] Clipboard cambió (${text.length} chars): ${text.take(50)}")

            // Enviar a Mac via BLE
            ClipboardService.sendToMac(text)
        } catch (e: Exception) {
            Log.e(TAG, "Error en clipboard change: ${e.message}")
        }
    }

    private fun crc32(text: String): Long {
        val crc = java.util.zip.CRC32()
        crc.update(text.toByteArray())
        return crc.value
    }
}
