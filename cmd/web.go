package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func startWebServer() {
	http.HandleFunc("/", handleDashboard)
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/qr", handleQR)
	http.HandleFunc("/api/qr/new", handleQRNew)
	http.HandleFunc("/api/pair", handlePair)
	http.HandleFunc("/api/unpair", handleUnpair)
	http.HandleFunc("/api/cleardb", handleClearDB)
	http.HandleFunc("/api/copy", handleCopy)

	fmt.Println("[Web] 🌐 Dashboard en http://localhost:8066")
	go func() {
		if err := http.ListenAndServe("127.0.0.1:8066", nil); err != nil {
			fmt.Printf("[Web] Error: %s\n", err)
		}
	}()
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	var entries []ClipEntry
	if q != "" {
		entries = searchHistory(q, limit)
	} else {
		entries = getHistory(limit)
	}
	if entries == nil {
		entries = []ClipEntry{}
	}
	json.NewEncoder(w).Encode(entries)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clipMu.Lock()
	content := lastClipContent
	hash := lastClipHash
	clipMu.Unlock()

	status := map[string]interface{}{
		"ble_connected":  bleReady,
		"paired":         isPaired(),
		"current_clip":   truncate(content, 100),
		"current_hash":   fmt.Sprintf("%08x", hash),
		"clip_length":    len(content),
		"stats":          getStats(),
	}
	json.NewEncoder(w).Encode(status)
}

func handleQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	token := generatePairingToken(false)
	url := fmt.Sprintf("clipsync://pair?token=%s", token)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "url": url})
}

func handleQRNew(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	token := generatePairingToken(true)
	url := fmt.Sprintf("clipsync://pair?token=%s", token)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "url": url})
}

func handlePair(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Pairing ahora se hace por BLE, no por HTTP
	json.NewEncoder(w).Encode(map[string]interface{}{"paired": isPaired()})
}

func handleCopy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	setMacClipboard(body.Content)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleUnpair(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	unpair()
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleClearDB(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if db != nil {
		db.Exec("DELETE FROM clipboard_history")
	}
	fmt.Println("[DB] Historial limpiado")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ClipSync — Dashboard</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#0a0a1a;--card:#12122e;--card-hover:#1a1a3e;
  --accent:#6c5ce7;--accent2:#a29bfe;--green:#00e676;--red:#ff5252;--orange:#ffa726;
  --text:#e0e0ff;--text-dim:#7777aa;--text-muted:#44446a;
  --radius:16px;--radius-sm:10px;
}
body{font-family:'Inter',system-ui,sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.container{max-width:900px;margin:0 auto;padding:24px}
header{text-align:center;padding:40px 0 32px}
header h1{font-size:2.2rem;font-weight:700;background:linear-gradient(135deg,#6c5ce7,#a29bfe,#00e676);-webkit-background-clip:text;-webkit-text-fill-color:transparent;display:inline-block}
header p{color:var(--text-dim);margin-top:6px;font-size:.9rem}

.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:24px}
.stat{background:var(--card);border-radius:var(--radius);padding:20px;text-align:center;transition:transform .2s,box-shadow .2s}
.stat:hover{transform:translateY(-2px);box-shadow:0 8px 32px rgba(108,92,231,.15)}
.stat .value{font-size:1.8rem;font-weight:700;color:var(--accent2)}
.stat .label{font-size:.75rem;color:var(--text-dim);text-transform:uppercase;letter-spacing:.5px;margin-top:4px}
.stat.connected .value{color:var(--green)}
.stat.disconnected .value{color:var(--red)}

.section{margin-bottom:24px}
.section h2{font-size:1.1rem;font-weight:600;color:var(--text-dim);margin-bottom:12px;display:flex;align-items:center;gap:8px}

.search-bar{display:flex;gap:8px;margin-bottom:16px}
.search-bar input{flex:1;background:var(--card);border:1px solid #2a2a4a;border-radius:var(--radius-sm);padding:12px 16px;color:var(--text);font-size:.9rem;outline:none;transition:border-color .2s}
.search-bar input:focus{border-color:var(--accent)}
.search-bar input::placeholder{color:var(--text-muted)}

.history{display:flex;flex-direction:column;gap:8px}
.clip{background:var(--card);border-radius:var(--radius-sm);padding:14px 18px;cursor:pointer;transition:all .2s;border-left:3px solid transparent;position:relative}
.clip:hover{background:var(--card-hover);border-left-color:var(--accent);transform:translateX(4px)}
.clip .content{font-size:.85rem;color:var(--text);word-break:break-all;max-height:60px;overflow:hidden;line-height:1.5}
.clip .meta{display:flex;justify-content:space-between;margin-top:8px;font-size:.7rem;color:var(--text-muted)}
.clip .source{padding:2px 8px;border-radius:20px;font-weight:600;font-size:.65rem;text-transform:uppercase;letter-spacing:.5px}
.clip .source.mac{background:rgba(108,92,231,.2);color:var(--accent2)}
.clip .source.android{background:rgba(0,230,118,.15);color:var(--green)}
.clip .copied{position:absolute;right:16px;top:14px;color:var(--green);font-size:.75rem;opacity:0;transition:opacity .3s}
.clip .copied.show{opacity:1}

.qr-section{text-align:center;padding:32px;background:var(--card);border-radius:var(--radius)}
.qr-section button{background:var(--accent);color:white;border:none;padding:12px 28px;border-radius:28px;font-size:.9rem;font-weight:600;cursor:pointer;transition:all .2s}
.qr-section button:hover{background:#7c6cf7;transform:scale(1.02)}
.qr-token{font-family:monospace;font-size:1.2rem;color:var(--accent2);margin-top:16px;padding:12px;background:rgba(108,92,231,.1);border-radius:8px;word-break:break-all}

.empty{text-align:center;padding:40px;color:var(--text-muted)}

@keyframes fadeIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.clip{animation:fadeIn .3s ease both}
.clip:nth-child(2){animation-delay:.05s}
.clip:nth-child(3){animation-delay:.1s}
.clip:nth-child(4){animation-delay:.15s}

@media(max-width:600px){.stats{grid-template-columns:repeat(2,1fr)}}
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>📋 ClipSync</h1>
    <p>Universal Clipboard — Mac ↔ Android via BLE</p>
  </header>

  <div class="stats" id="stats">
    <div class="stat" id="stat-ble"><div class="value">—</div><div class="label">BLE</div></div>
    <div class="stat"><div class="value" id="stat-total">0</div><div class="label">Total Clips</div></div>
    <div class="stat"><div class="value" id="stat-mac">0</div><div class="label">Desde Mac</div></div>
    <div class="stat"><div class="value" id="stat-android">0</div><div class="label">Desde Android</div></div>
  </div>

  <div class="section">
    <h2>📜 Historial</h2>
    <div class="search-bar">
      <input type="text" id="search" placeholder="Buscar en historial..." autocomplete="off">
    </div>
    <div class="history" id="history"></div>
  </div>

  <div class="section">
    <h2>🔗 Pairing</h2>
    <div class="qr-section" id="qr-section">
      <div id="qr-unpaired">
        <div style="font-size:3rem;margin-bottom:16px">📲</div>
        <p style="color:var(--text-dim);margin-bottom:20px;line-height:1.5">Escaneá este QR desde la app ClipSync en Android</p>
        <div id="qr-canvas" style="margin:0 auto;background:white;padding:16px;border-radius:12px;width:fit-content;box-shadow:0 0 40px rgba(108,92,231,.3)"></div>
        <button onclick="regenerateQR()" style="margin-top:16px;background:var(--card);color:var(--accent2);border:1px solid #2a2a4a;padding:8px 20px;border-radius:20px;font-size:.8rem;cursor:pointer">🔄 Regenerar QR</button>
      </div>
      <div id="qr-paired" style="display:none">
        <div style="font-size:3rem;margin-bottom:16px">✅</div>
        <p style="color:var(--green);font-weight:600;font-size:1.1rem">Dispositivo vinculado</p>
        <p style="color:var(--text-dim);margin-top:8px;font-size:.85rem">Sync activo — persistente entre reinicios</p>
        <button onclick="doUnpair()" style="margin-top:16px;background:#e74c5c;color:white;border:none;padding:10px 24px;border-radius:28px;font-size:.85rem;cursor:pointer">🔓 Desvincular</button>
      </div>
    </div>
  </div>

  <div class="section" style="text-align:center">
    <button onclick="doClearDB()" style="background:none;border:1px solid #2a2a4a;color:var(--text-muted);padding:8px 20px;border-radius:20px;font-size:.8rem;cursor:pointer">🗑 Limpiar historial</button>
  </div>
</div>

<script src="https://cdn.jsdelivr.net/npm/qrcodejs2@0.0.2/qrcode.min.js"></script>
<script>
async function loadStatus(){
  try{
    const r=await fetch('/api/status');
    const s=await r.json();
    const ble=document.getElementById('stat-ble');
    ble.querySelector('.value').textContent=s.ble_connected?'✅':'❌';
    ble.className='stat '+(s.ble_connected?'connected':'disconnected');
    document.getElementById('stat-total').textContent=s.stats?.total||0;
    document.getElementById('stat-mac').textContent=s.stats?.from_mac||0;
    document.getElementById('stat-android').textContent=s.stats?.from_android||0;
    // Update pairing status
    if(s.paired){
      document.getElementById('qr-unpaired').style.display='none';
      document.getElementById('qr-paired').style.display='block';
    } else {
      document.getElementById('qr-unpaired').style.display='block';
      document.getElementById('qr-paired').style.display='none';
    }
  }catch(e){}
}

async function loadHistory(q=''){
  try{
    const url=q?'/api/history?q='+encodeURIComponent(q):'/api/history';
    const r=await fetch(url);
    const items=await r.json();
    const el=document.getElementById('history');
    if(!items.length){el.innerHTML='<div class="empty">No hay clips todavía</div>';return}
    el.innerHTML=items.map((c,i)=>{
      const preview=c.content.length>200?c.content.slice(0,200)+'...':c.content;
      const time=new Date(c.timestamp+'Z').toLocaleString();
      return '<div class="clip" onclick="copyClip(this,\''+btoa(unescape(encodeURIComponent(c.content)))+'\')" style="animation-delay:'+(i*.03)+'s">'+
        '<div class="content">'+escHtml(preview)+'</div>'+
        '<div class="meta"><span class="source '+c.source+'">'+c.source+'</span><span>'+c.length+' chars · '+time+'</span></div>'+
        '<div class="copied">✓ Copiado</div></div>';
    }).join('');
  }catch(e){}
}

function copyClip(el,b64){
  const text=decodeURIComponent(escape(atob(b64)));
  fetch('/api/copy',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content:text})});
  const badge=el.querySelector('.copied');
  badge.classList.add('show');
  setTimeout(()=>badge.classList.remove('show'),1500);
}

function escHtml(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}

async function generateQR(){
  const r=await fetch('/api/qr');
  const d=await r.json();
  const container=document.getElementById('qr-canvas');
  container.innerHTML='';
  new QRCode(container,{text:d.url,width:220,height:220,colorDark:'#1a1a2e',colorLight:'#ffffff',correctLevel:QRCode.CorrectLevel.M});
}

async function regenerateQR(){
  const r=await fetch('/api/qr/new');
  const d=await r.json();
  const container=document.getElementById('qr-canvas');
  container.innerHTML='';
  new QRCode(container,{text:d.url,width:220,height:220,colorDark:'#1a1a2e',colorLight:'#ffffff',correctLevel:QRCode.CorrectLevel.M});
}

async function doUnpair(){
  if(!confirm('¿Desvincular dispositivo? El sync se detendrá.'))return;
  await fetch('/api/unpair');
  document.getElementById('qr-unpaired').style.display='block';
  document.getElementById('qr-paired').style.display='none';
  generateQR();
}

async function doClearDB(){
  if(!confirm('¿Borrar todo el historial?'))return;
  await fetch('/api/cleardb');
  loadHistory();
}

let searchTimer;
document.getElementById('search').addEventListener('input',e=>{
  clearTimeout(searchTimer);
  searchTimer=setTimeout(()=>loadHistory(e.target.value),300);
});

// Auto-generar QR al cargar
generateQR();
loadStatus();loadHistory();
setInterval(()=>{loadStatus();loadHistory(document.getElementById('search').value)},3000);
</script>
</body>
</html>`
