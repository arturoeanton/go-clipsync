package com.arturoeanton.clipboardclient

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.util.Log
import android.widget.Toast

/**
 * Actividad transparente que recibe texto via Share o PROCESS_TEXT
 * y lo envía al servicio BLE para sync con Mac.
 */
class ShareReceiverActivity : Activity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val text = extractText(intent)

        if (text != null && text.isNotBlank()) {
            Log.i("ClipSync", "[Share → Mac] Recibido (${text.length} chars): ${text.take(50)}")

            // Enviar al servicio via broadcast o directamente al GATT
            ClipboardService.sendToMac(text)

            Toast.makeText(this, "📋 Enviado a Mac (${text.length} chars)", Toast.LENGTH_SHORT).show()
        } else {
            Log.w("ClipSync", "[Share] No se recibió texto")
            Toast.makeText(this, "No se recibió texto", Toast.LENGTH_SHORT).show()
        }

        finish()
    }

    private fun extractText(intent: Intent?): String? {
        if (intent == null) return null

        return when (intent.action) {
            Intent.ACTION_SEND -> intent.getStringExtra(Intent.EXTRA_TEXT)
            Intent.ACTION_PROCESS_TEXT -> intent.getCharSequenceExtra(Intent.EXTRA_PROCESS_TEXT)?.toString()
            else -> null
        }
    }
}
