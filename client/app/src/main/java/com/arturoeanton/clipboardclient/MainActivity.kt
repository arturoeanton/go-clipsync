package com.arturoeanton.clipboardclient

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.util.TypedValue
import android.view.Gravity
import android.widget.*
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions

class MainActivity : AppCompatActivity() {

    private lateinit var statusText: TextView
    private lateinit var actionBtn: Button

    companion object {
        private const val PERMISSION_REQUEST_CODE = 1001
    }

    private val qrLauncher = registerForActivityResult(ScanContract()) { result ->
        if (result.contents != null) {
            handleQRResult(result.contents)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.statusBarColor = Color.parseColor("#0D0D1A")
        window.navigationBarColor = Color.parseColor("#0D0D1A")

        val root = ScrollView(this).apply {
            setBackgroundColor(Color.parseColor("#0D0D1A"))
        }

        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(dp(32), dp(80), dp(32), dp(48))
        }

        // === Icon ===
        layout.addView(TextView(this).apply {
            text = "📋"
            textSize = 72f
            gravity = Gravity.CENTER
        })

        layout.addView(spacer(16))

        // === Title ===
        layout.addView(TextView(this).apply {
            text = "ClipSync"
            setTextColor(Color.WHITE)
            textSize = 36f
            typeface = Typeface.create("sans-serif-medium", Typeface.BOLD)
            gravity = Gravity.CENTER
        })

        layout.addView(TextView(this).apply {
            text = "Universal Clipboard via Bluetooth"
            setTextColor(Color.parseColor("#8888AA"))
            textSize = 14f
            gravity = Gravity.CENTER
            setPadding(0, dp(4), 0, 0)
        })

        layout.addView(spacer(40))

        // === Status ===
        statusText = TextView(this).apply {
            text = "Escaneá el QR del dashboard\npara vincular y sincronizar"
            setTextColor(Color.parseColor("#7777AA"))
            textSize = 15f
            gravity = Gravity.CENTER
            setLineSpacing(dp(4).toFloat(), 1f)
        }
        layout.addView(statusText)

        layout.addView(spacer(32))

        // === ONE BUTTON ===
        actionBtn = Button(this).apply {
            text = "📷  Escanear QR"
            setTextColor(Color.WHITE)
            textSize = 18f
            typeface = Typeface.create("sans-serif-medium", Typeface.BOLD)
            isAllCaps = false
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#6C5CE7"))
                cornerRadius = dp(32).toFloat()
            }
            setPadding(dp(48), dp(18), dp(48), dp(18))
            setOnClickListener { launchQRScanner() }
        }
        layout.addView(actionBtn)

        layout.addView(spacer(24))

        // === Accessibility link (small, secondary) ===
        layout.addView(TextView(this).apply {
            text = "⚙ Activar sync automático"
            setTextColor(Color.parseColor("#6C5CE7"))
            textSize = 13f
            gravity = Gravity.CENTER
            setPadding(0, dp(8), 0, dp(8))
            setOnClickListener {
                startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
                Toast.makeText(this@MainActivity,
                    "Buscá 'ClipSync' y activalo", Toast.LENGTH_LONG).show()
            }
        })

        layout.addView(spacer(48))

        // === How it works (minimal) ===
        val steps = listOf(
            "1. Abrí localhost:8066 en la Mac",
            "2. Tocá 'Escanear QR' acá arriba",
            "3. Listo — el clipboard se sincroniza"
        )
        for (step in steps) {
            layout.addView(TextView(this).apply {
                text = step
                setTextColor(Color.parseColor("#555577"))
                textSize = 13f
                gravity = Gravity.CENTER
                setPadding(0, dp(3), 0, dp(3))
            })
        }

        root.addView(layout)
        setContentView(root)
        requestPermissions()
        checkExistingPairing()
    }

    override fun onResume() {
        super.onResume()
        checkExistingPairing()
    }

    private fun checkExistingPairing() {
        val token = getSharedPreferences("clipsync_prefs", MODE_PRIVATE)
            .getString("pairing_token", "") ?: ""
        if (token.isNotEmpty()) {
            statusText.text = "✅ Vinculado y sincronizando"
            statusText.setTextColor(Color.parseColor("#00E676"))
            actionBtn.text = "📷  Re-escanear QR"
            (actionBtn.background as? GradientDrawable)?.setColor(Color.parseColor("#00B894"))
        }
    }

    private fun requestPermissions() {
        val permissions = mutableListOf<String>()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            permissions.add(Manifest.permission.BLUETOOTH_ADVERTISE)
            permissions.add(Manifest.permission.BLUETOOTH_CONNECT)
        }
        permissions.add(Manifest.permission.ACCESS_FINE_LOCATION)
        permissions.add(Manifest.permission.CAMERA)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }
        val needed = permissions.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }
        if (needed.isNotEmpty()) {
            ActivityCompat.requestPermissions(this, needed.toTypedArray(), PERMISSION_REQUEST_CODE)
        }
    }

    private fun launchQRScanner() {
        val options = ScanOptions().apply {
            setDesiredBarcodeFormats(ScanOptions.QR_CODE)
            setPrompt("Escaneá el QR del dashboard de ClipSync")
            setBeepEnabled(false)
            setOrientationLocked(true)
        }
        qrLauncher.launch(options)
    }

    private fun handleQRResult(contents: String) {
        if (contents.startsWith("clipsync://pair?token=")) {
            val token = contents.substringAfter("token=")

            // 1. Parar servicio viejo (limpiar token viejo de memoria)
            stopService(Intent(this, ClipboardService::class.java))

            // 2. Guardar token nuevo
            getSharedPreferences("clipsync_prefs", MODE_PRIVATE)
                .edit().putString("pairing_token", token).apply()

            // 3. Reiniciar BLE con token nuevo
            ContextCompat.startForegroundService(this, Intent(this, ClipboardService::class.java))

            // 4. Actualizar UI
            statusText.text = "✅ Vinculado y sincronizando"
            statusText.setTextColor(Color.parseColor("#00E676"))
            actionBtn.text = "📷  Re-escanear QR"
            (actionBtn.background as? GradientDrawable)?.setColor(Color.parseColor("#00B894"))

            Toast.makeText(this, "✅ Token actualizado! Sync activo.", Toast.LENGTH_LONG).show()
        } else {
            Toast.makeText(this, "QR no válido — escaneá el de ClipSync", Toast.LENGTH_SHORT).show()
        }
    }

    // === Helpers ===

    private fun spacer(dpHeight: Int): android.view.View {
        return android.view.View(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT, dp(dpHeight)
            )
        }
    }

    private fun dp(value: Int): Int {
        return TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP, value.toFloat(), resources.displayMetrics
        ).toInt()
    }
}
