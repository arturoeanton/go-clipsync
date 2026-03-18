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
	setClipboard(body.Content)
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
<title>ClipSync</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#f5f5f7;
  --surface:#ffffff;
  --border:#d2d2d7;
  --border-light:#e8e8ed;
  --text:#1d1d1f;
  --text-secondary:#86868b;
  --text-tertiary:#aeaeb2;
  --accent:#0071e3;
  --accent-hover:#0077ed;
  --green:#34c759;
  --red:#ff3b30;
  --radius:12px;
  --radius-sm:8px;
}
body{
  font-family:'Inter',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
  background:var(--bg);color:var(--text);
  min-height:100vh;
  -webkit-font-smoothing:antialiased;
}
.container{max-width:720px;margin:0 auto;padding:32px 24px}

/* Header */
header{padding:48px 0 40px;text-align:center}
header h1{font-size:1.75rem;font-weight:600;letter-spacing:-.02em;color:var(--text)}
header p{color:var(--text-secondary);margin-top:4px;font-size:.875rem;font-weight:400}

/* Stats */
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:32px}
.stat{
  background:var(--surface);
  border:1px solid var(--border-light);
  border-radius:var(--radius);
  padding:20px 16px;
  text-align:center;
}
.stat .value{font-size:1.5rem;font-weight:600;color:var(--text)}
.stat .label{font-size:.6875rem;color:var(--text-secondary);text-transform:uppercase;letter-spacing:.04em;margin-top:4px;font-weight:500}
.stat.connected .value{color:var(--green)}
.stat.disconnected .value{color:var(--red)}

/* Section */
.section{margin-bottom:32px}
.section-title{font-size:.8125rem;font-weight:600;color:var(--text-secondary);text-transform:uppercase;letter-spacing:.04em;margin-bottom:12px}

/* Search */
.search-bar{margin-bottom:16px}
.search-bar input{
  width:100%;
  background:var(--surface);
  border:1px solid var(--border-light);
  border-radius:var(--radius-sm);
  padding:10px 14px;
  color:var(--text);
  font-size:.875rem;
  font-family:inherit;
  outline:none;
  transition:border-color .2s;
}
.search-bar input:focus{border-color:var(--accent)}
.search-bar input::placeholder{color:var(--text-tertiary)}

/* History */
.history{display:flex;flex-direction:column;gap:1px;background:var(--border-light);border-radius:var(--radius);overflow:hidden}
.clip{
  background:var(--surface);
  padding:14px 16px;
  cursor:pointer;
  transition:background .15s;
  position:relative;
}
.clip:hover{background:#fafafa}
.clip .content{
  font-size:.8125rem;
  color:var(--text);
  line-height:1.5;
  max-height:48px;
  overflow:hidden;
  word-break:break-word;
}
.clip .meta{
  display:flex;
  justify-content:space-between;
  align-items:center;
  margin-top:8px;
  font-size:.6875rem;
  color:var(--text-tertiary);
}
.clip .source{
  display:inline-block;
  padding:2px 8px;
  border-radius:4px;
  font-weight:500;
  font-size:.625rem;
  text-transform:uppercase;
  letter-spacing:.03em;
}
.clip .source.mac,.clip .source.linux{background:#f0f0f5;color:var(--text-secondary)}
.clip .source.android{background:#e8f8ee;color:#1b7d3a}
.clip .copied{
  position:absolute;right:16px;top:14px;
  color:var(--green);font-size:.75rem;font-weight:500;
  opacity:0;transition:opacity .25s;
}
.clip .copied.show{opacity:1}

/* Pairing section */
.pair-card{
  background:var(--surface);
  border:1px solid var(--border-light);
  border-radius:var(--radius);
  padding:32px;
  text-align:center;
}
.pair-card p{color:var(--text-secondary);font-size:.875rem;line-height:1.5;margin-bottom:20px}
.pair-card .paired-label{color:var(--green);font-weight:600;font-size:1rem}
.pair-card .paired-sub{color:var(--text-tertiary);font-size:.8125rem;margin-top:4px}

#qr-canvas{margin:0 auto;background:white;padding:12px;border-radius:var(--radius-sm);width:fit-content;border:1px solid var(--border-light)}

/* Buttons */
.btn{
  display:inline-block;
  padding:8px 20px;
  border-radius:980px;
  font-size:.8125rem;
  font-weight:500;
  font-family:inherit;
  cursor:pointer;
  transition:opacity .15s;
  border:none;
}
.btn:hover{opacity:.85}
.btn-primary{background:var(--accent);color:white}
.btn-secondary{background:transparent;color:var(--text-secondary);border:1px solid var(--border)}
.btn-danger{background:var(--red);color:white}

.empty{text-align:center;padding:40px;color:var(--text-tertiary);font-size:.875rem;background:var(--surface);border-radius:var(--radius)}

.footer-actions{text-align:center;padding-top:8px}

@media(max-width:600px){
  .stats{grid-template-columns:repeat(2,1fr)}
  .container{padding:20px 16px}
  header{padding:32px 0 28px}
}
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>ClipSync</h1>
    <p>Clipboard universal — sincronización multi-desktop via BLE</p>
  </header>

  <div class="stats" id="stats">
    <div class="stat" id="stat-ble"><div class="value">—</div><div class="label">BLE</div></div>
    <div class="stat"><div class="value" id="stat-total">0</div><div class="label">Total</div></div>
    <div class="stat"><div class="value" id="stat-mac">0</div><div class="label">Desktop</div></div>
    <div class="stat"><div class="value" id="stat-android">0</div><div class="label">Android</div></div>
  </div>

  <div class="section">
    <div class="section-title">Historial</div>
    <div class="search-bar">
      <input type="text" id="search" placeholder="Buscar..." autocomplete="off">
    </div>
    <div id="history"></div>
  </div>

  <div class="section">
    <div class="section-title">Vinculación</div>
    <div class="pair-card" id="qr-section">
      <div id="qr-unpaired">
        <p>Escaneá el código QR desde la app ClipSync en Android para vincular. Podés usar el mismo QR en múltiples desktops.</p>
        <div id="qr-canvas"></div>
        <div style="margin-top:16px">
          <button class="btn btn-secondary" onclick="regenerateQR()">Regenerar QR</button>
        </div>
      </div>
      <div id="qr-paired" style="display:none">
        <div class="paired-label">Dispositivo vinculado</div>
        <p class="paired-sub">Sincronización activa — persistente entre reinicios</p>
        <button class="btn btn-danger" onclick="doUnpair()" style="margin-top:16px">Desvincular</button>
      </div>
    </div>
  </div>

  <div class="footer-actions">
    <button class="btn btn-secondary" onclick="doClearDB()">Limpiar historial</button>
  </div>
</div>

<script src="https://cdn.jsdelivr.net/npm/qrcodejs2@0.0.2/qrcode.min.js"></script>
<script>
async function loadStatus(){
  try{
    const r=await fetch('/api/status');
    const s=await r.json();
    const ble=document.getElementById('stat-ble');
    ble.querySelector('.value').textContent=s.ble_connected?'On':'Off';
    ble.className='stat '+(s.ble_connected?'connected':'disconnected');
    document.getElementById('stat-total').textContent=s.stats?.total||0;
    document.getElementById('stat-mac').textContent=(s.stats?.from_mac||0)+(s.stats?.from_linux||0);
    document.getElementById('stat-android').textContent=s.stats?.from_android||0;
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
    if(!items.length){el.innerHTML='<div class="empty">Sin clips todavía</div>';return}
    el.className='history';
    el.innerHTML=items.map((c,i)=>{
      const preview=c.content.length>200?c.content.slice(0,200)+'…':c.content;
      const time=new Date(c.timestamp+'Z').toLocaleString();
      return '<div class="clip" onclick="copyClip(this,\''+btoa(unescape(encodeURIComponent(c.content)))+'\')">'+
        '<div class="content">'+escHtml(preview)+'</div>'+
        '<div class="meta"><span class="source '+c.source+'">'+c.source+'</span><span>'+c.length+' chars · '+time+'</span></div>'+
        '<div class="copied">Copiado</div></div>';
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
  new QRCode(container,{text:d.url,width:200,height:200,colorDark:'#1d1d1f',colorLight:'#ffffff',correctLevel:QRCode.CorrectLevel.M});
}

async function regenerateQR(){
  const r=await fetch('/api/qr/new');
  const d=await r.json();
  const container=document.getElementById('qr-canvas');
  container.innerHTML='';
  new QRCode(container,{text:d.url,width:200,height:200,colorDark:'#1d1d1f',colorLight:'#ffffff',correctLevel:QRCode.CorrectLevel.M});
}

async function doUnpair(){
  if(!confirm('¿Desvincular dispositivo? La sincronización se detendrá.'))return;
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

generateQR();
loadStatus();loadHistory();
setInterval(()=>{loadStatus();loadHistory(document.getElementById('search').value)},3000);
</script>
</body>
</html>`
