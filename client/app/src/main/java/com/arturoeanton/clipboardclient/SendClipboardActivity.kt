package com.arturoeanton.clipboardclient

import android.app.Activity
import android.content.ClipboardManager
import android.content.Context
import android.os.Bundle
import android.util.Log
import android.widget.Toast

/**
 * Actividad transparente que se abre al tocar la notificación "Enviar Clipboard".
 * Al estar en foreground, SÍ puede leer el clipboard (bypass restricción Android 10+).
 * Lee el clipboard → envía a Mac via BLE → se cierra.
 */
class SendClipboardActivity : Activity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val clip = clipboard.primaryClip

        // Intentar leer del clipboard del sistema
        var text: String? = null
        if (clip != null && clip.itemCount > 0) {
            text = clip.getItemAt(0).text?.toString()
        }

        // Fallback: usar el contenido cacheado del servicio BLE
        if (text.isNullOrBlank()) {
            text = ClipboardService.sendToMac_getLastContent()
        }

        if (!text.isNullOrBlank()) {
            Log.i("ClipSync", "[Clipboard → Mac] Enviando (${text.length} chars)")
            ClipboardService.sendToMac(text)
            Toast.makeText(this, "📋→💻 Enviado! (${text.length} chars)", Toast.LENGTH_SHORT).show()
        } else {
            Toast.makeText(this, "Clipboard vacío", Toast.LENGTH_SHORT).show()
        }

        finish()
    }
}
