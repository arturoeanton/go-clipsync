package com.arturoeanton.clipboardclient

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.core.content.ContextCompat

/**
 * Receives BOOT_COMPLETED and MY_PACKAGE_REPLACED broadcasts to auto-start
 * the ClipboardService after device reboot or app update.
 *
 * Only starts if there are existing pairing tokens (user has paired at least once).
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val action = intent?.action ?: return
        Log.i("ClipSync", "BootReceiver: $action")

        if (action == Intent.ACTION_BOOT_COMPLETED ||
            action == Intent.ACTION_MY_PACKAGE_REPLACED
        ) {
            // Only auto-start if user has paired at least once
            val tokens = context.getSharedPreferences("clipsync_prefs", Context.MODE_PRIVATE)
                .getString("pairing_token", "") ?: ""
            if (tokens.isNotBlank()) {
                Log.i("ClipSync", "Auto-starting ClipboardService (${tokens.split("|").size} tokens)")
                val serviceIntent = Intent(context, ClipboardService::class.java)
                ContextCompat.startForegroundService(context, serviceIntent)
            } else {
                Log.i("ClipSync", "No pairing tokens — skipping auto-start")
            }
        }
    }
}
