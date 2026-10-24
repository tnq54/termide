'use strict';
const $ = id => document.getElementById(id);
const native = window.Native || {send(){}, resize(){}, disconnect(){}, importKey(){}, clearKey(){}, connect(){connectionStatus('SSH chỉ chạy trong APK Android.', false);}};
let term, fit, mode = 'idle', ctrl = false, demoLine = '';
const keys = {esc:'\x1b', tab:'\t', up:'\x1b[A', down:'\x1b[B', left:'\x1b[D', right:'\x1b[C', save:'\x13', quit:'\x03', enter:'\r'};

function ensureTerminal() {
  if (term) return;
  term = new Terminal({fontSize:14,fontFamily:'monospace',cursorBlink:true,scrollback:2000,
    theme:{background:'#102a43',foreground:'#d9e2ec',cursor:'#f0b429',selectionBackground:'#486581'},
    allowProposedApi:false});
  fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open($('terminal'));
  term.onData(data => {
    if (ctrl && data.length === 1) {
      const code = data.toUpperCase().charCodeAt(0);
      if (code >= 64 && code <= 95) data = String.fromCharCode(code - 64);
      else if (data === ' ') data = '\0';
      setCtrl(false);
    }
    send(data);
  });
  term.onResize(({cols,rows}) => { if (mode === 'ssh') native.resize(cols,rows); });
  new ResizeObserver(() => { if (!$('session').hidden) fit.fit(); }).observe($('terminal'));
}
function setCtrl(value) { ctrl=value; $('ctrl').setAttribute('aria-pressed',String(value)); }
function openTerminal() {
  $('connection').hidden=true; $('session').hidden=false;
  ensureTerminal(); fit.fit(); term.focus();
}
function showConnectionForm() { $('connection').hidden=false; $('session').hidden=true; }
function connectionStatus(text, active) {
  $('status').textContent=text;
  $('badge').textContent=active ? 'SSH trực tiếp' : mode === 'demo' ? 'Demo offline' : 'Bản thử độc lập';
}
function restoreProfile(profile) {
  for (const id of ['host','port','user','project']) if (profile[id]) $(id).value=profile[id];
}
function keyImported(ready) {
  $('key-label').textContent=ready ? 'Key sẵn sàng (trong RAM)' : 'Chưa chọn key';
  $('phrase-label').hidden=!ready; $('clear-key').hidden=!ready;
  if (!ready) $('passphrase').value='';
}
function terminalOutput(base64) {
  const data=Uint8Array.from(atob(base64),c=>c.charCodeAt(0));
  if (term && mode === 'ssh') term.write(data);
}
function connect(shell) {
  if (!$('profile').reportValidity()) return;
  const port=Number($('port').value);
  if (!Number.isInteger(port)||port<1||port>65535) { connectionStatus('Port phải từ 1 đến 65535.',false); return; }
  if (!shell && !$('project').value.startsWith('/')) { connectionStatus('Nhập đường dẫn project tuyệt đối trên VPS.',false); return; }
  mode='ssh'; setCtrl(false); openTerminal(); term.reset();
  connectionStatus('Đang kết nối…',false);
  const payload={shell,cols:term.cols,rows:term.rows};
  for (const id of ['host','port','user','project','password','passphrase']) payload[id]=$(id).value;
  native.connect(JSON.stringify(payload));
  $('password').value=''; $('passphrase').value='';
}
function send(data) {
  if (mode === 'ssh') native.send(data);
  else if (mode === 'demo') demoInput(data);
}
function prompt() { term.write('\r\n\x1b[36mtermide-demo\x1b[0m $ '); }
function demoInput(data) {
  if (data.startsWith('\x1b')) return;
  for (const c of data) {
    if (c === '\r' || c === '\n') {
      const command=demoLine.trim(); demoLine='';
      if (command === 'clear') term.clear();
      else if (command === 'help') term.write('\r\nhelp   Hướng dẫn\r\nclear  Xóa màn hình\r\nunicode  Kiểm tra chữ Việt / emoji\r\nĐây là demo terminal, không phải shell Linux.');
      else if (command === 'unicode') term.write('\r\nTiếng Việt: sửa mã, lưu file.  😀 🚀');
      else if (command) term.write('\r\nDemo nhận: '+command.replace(/[\x00-\x1f\x7f]/g,''));
      prompt();
    } else if (c === '\x7f' || c === '\b') {
      if (demoLine) { demoLine=Array.from(demoLine).slice(0,-1).join(''); term.write('\b \b'); }
    } else if (c === '\x03') { term.write('^C'); demoLine=''; prompt(); }
    else if (c === '\x13') connectionStatus('Ctrl+S nhận thành công. Demo không ghi file.',false);
    else if (c.charCodeAt(0)>=32) { demoLine+=c; term.write(c); }
  }
}
$('profile').addEventListener('submit',e=>{e.preventDefault();connect(false);});
$('shell').onclick=()=>connect(true);
$('settings').onclick=()=>{ if ($('session').hidden && term) openTerminal(); else showConnectionForm(); };
$('import-key').onclick=()=>native.importKey();
$('clear-key').onclick=()=>native.clearKey();
$('ctrl').onclick=()=>{setCtrl(!ctrl);term.focus();};
for(const b of document.querySelectorAll('[data-key]')) b.onclick=()=>{send(keys[b.dataset.key]);setCtrl(false);term.focus();};
$('keyboard').onclick=()=>term.focus();
for(const [id,delta] of [['smaller',-1],['larger',1]]) $(id).onclick=()=>{term.options.fontSize=Math.max(10,Math.min(24,term.options.fontSize+delta));fit.fit();};
$('disconnect').onclick=()=>{native.disconnect();mode='idle';setCtrl(false);connectionStatus('Đã ngắt kết nối.',false);showConnectionForm();};
$('demo').onclick=()=>{
  native.disconnect(); mode='demo'; demoLine=''; setCtrl(false); openTerminal(); term.reset();
  term.write('\x1b[1;36mTermIDE / terminal độc lập\x1b[0m\r\n\r\nKhông cần Termux. Không cần mạng cho demo.\r\nThử gõ help, unicode hoặc Ctrl+S.\r\nKết nối VPS để dùng IDE thật.\r\n');
  prompt();connectionStatus('Demo offline • thử bàn phím, màu và cuộn.',false);
};
