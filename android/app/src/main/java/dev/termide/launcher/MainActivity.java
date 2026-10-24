package dev.termide.launcher;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.content.SharedPreferences;
import android.graphics.Color;
import android.net.Uri;
import android.os.Bundle;
import android.webkit.*;
import org.bouncycastle.jce.provider.BouncyCastleProvider;
import org.json.JSONObject;
import java.io.*;
import java.security.Security;
import java.util.Arrays;
import java.util.Base64;
import java.util.concurrent.*;

public final class MainActivity extends Activity {
    private WebView web;
    private volatile SshConnection connection;
    private volatile boolean destroyed;
    private byte[] privateKey;
    private static final int IMPORT_KEY = 7;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        // Android's built-in BC omits algorithms used by modern OpenSSH keys.
        Security.removeProvider("BC");
        Security.addProvider(new BouncyCastleProvider());
        getWindow().setStatusBarColor(Color.rgb(36, 59, 83));
        getWindow().setNavigationBarColor(Color.rgb(16, 42, 67));
        web = new WebView(this);
        web.setBackgroundColor(Color.rgb(16, 42, 67));
        web.getSettings().setJavaScriptEnabled(true);
        web.getSettings().setAllowFileAccess(false);
        web.getSettings().setAllowContentAccess(false);
        web.getSettings().setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        web.getSettings().setSaveFormData(false);
        web.setWebViewClient(new WebViewClient() {
            @Override public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) { return true; }
            @Override public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url.startsWith("file:///android_asset/")) return null;
                return new WebResourceResponse("text/plain", "UTF-8", new ByteArrayInputStream(new byte[0]));
            }
            @Override public void onPageFinished(WebView view, String url) {
                SharedPreferences p = getPreferences(MODE_PRIVATE);
                JSONObject details = new JSONObject();
                try {
                    for (String key : new String[]{"host", "port", "user", "project"}) {
                        details.put(key, p.getString(key, key.equals("port") ? "22" : ""));
                    }
                } catch (Exception ignored) { }
                js("restoreProfile(" + details + ")");
            }
        });
        web.addJavascriptInterface(new Bridge(), "Native");
        setContentView(web);
        web.loadUrl("file:///android_asset/index.html");
    }

    private void js(String script) {
        runOnUiThread(() -> { if (!destroyed) web.evaluateJavascript(script, null); });
    }

    private void status(String text, boolean connected) {
        js("connectionStatus(" + JSONObject.quote(text) + "," + connected + ")");
    }

    private void disconnect() {
        SshConnection old = connection;
        connection = null;
        if (old != null) new Thread(old::close, "ssh-close").start();
    }

    private final class Bridge {
        @JavascriptInterface public void connect(String json) {
            runOnUiThread(() -> startConnection(json));
        }
        @JavascriptInterface public void send(String text) {
            SshConnection c = connection;
            if (c != null) c.send(text);
        }
        @JavascriptInterface public void resize(int columns, int rows) {
            SshConnection c = connection;
            if (c != null) c.resize(columns, rows);
        }
        @JavascriptInterface public void disconnect() {
            runOnUiThread(() -> { MainActivity.this.disconnect(); status("Đã ngắt kết nối.", false); });
        }
        @JavascriptInterface public void importKey() {
            runOnUiThread(() -> {
                Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
                intent.setType("*/*");
                intent.addCategory(Intent.CATEGORY_OPENABLE);
                startActivityForResult(intent, IMPORT_KEY);
            });
        }
        @JavascriptInterface public void clearKey() {
            runOnUiThread(() -> {
                if (privateKey != null) Arrays.fill(privateKey, (byte) 0);
                privateKey = null;
                js("keyImported(false)");
            });
        }
    }

    private void startConnection(String json) {
        if (destroyed) return;
        try {
            JSONObject p = new JSONObject(json);
            SshConfig config = new SshConfig(p.getString("host"), p.getInt("port"),
                    p.getString("user"), p.optString("project"), p.optBoolean("shell"));
            disconnect();
            SharedPreferences.Editor saved = getPreferences(MODE_PRIVATE).edit();
            for (String key : new String[]{"host", "port", "user", "project"}) saved.putString(key, p.optString(key));
            saved.apply();
            final SshConnection[] owner = new SshConnection[1];
            SshConnection next = new SshConnection(new File(getFilesDir(), "known_hosts"), new SshConnection.Listener() {
                private boolean current() { return !destroyed && connection == owner[0]; }
                @Override public boolean trust(String host, String fingerprint) {
                    if (!current()) return false;
                    CountDownLatch answered = new CountDownLatch(1);
                    boolean[] accepted = {false};
                    runOnUiThread(() -> {
                        if (!current()) { answered.countDown(); return; }
                        new AlertDialog.Builder(MainActivity.this).setTitle("Xác nhận máy chủ SSH")
                                .setMessage(host + "\n\n" + fingerprint + "\n\nSo sánh fingerprint với VPS trước khi tin cậy.")
                                .setPositiveButton("Tin cậy", (d, which) -> { accepted[0] = true; answered.countDown(); })
                                .setNegativeButton("Hủy", (d, which) -> answered.countDown())
                                .setOnCancelListener(d -> answered.countDown()).show();
                    });
                    try { return answered.await(60, TimeUnit.SECONDS) && accepted[0] && current(); }
                    catch (InterruptedException e) { Thread.currentThread().interrupt(); return false; }
                }
                @Override public void output(byte[] bytes) throws InterruptedException {
                    if (!current()) return;
                    CountDownLatch delivered = new CountDownLatch(1);
                    String encoded = Base64.getEncoder().encodeToString(bytes);
                    runOnUiThread(() -> {
                        if (!current()) { delivered.countDown(); return; }
                        web.evaluateJavascript("terminalOutput('" + encoded + "')", result -> delivered.countDown());
                    });
                    if (!delivered.await(15, TimeUnit.SECONDS)) throw new InterruptedException("terminal unavailable");
                }
                @Override public void status(String text, boolean active) {
                    runOnUiThread(() -> { if (current()) MainActivity.this.status(text, active); });
                }
            });
            owner[0] = next;
            connection = next;
            next.resize(p.optInt("cols", 80), p.optInt("rows", 24));
            byte[] key = privateKey == null ? null : privateKey.clone();
            String password = p.optString("password"), passphrase = p.optString("passphrase");
            new Thread(() -> next.run(config, password, key, passphrase), "ssh-session").start();
        } catch (Exception e) {
            status("Thông tin không hợp lệ. Kiểm tra host, port, user và đường dẫn project.", false);
        }
    }

    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request != IMPORT_KEY || result != RESULT_OK || data == null) return;
        Uri uri = data.getData();
        new Thread(() -> {
            try (InputStream in = getContentResolver().openInputStream(uri)) {
                ByteArrayOutputStream out = new ByteArrayOutputStream();
                byte[] buffer = new byte[4096];
                int n;
                while ((n = in.read(buffer)) != -1) {
                    if (out.size() + n > 128 * 1024) throw new IOException("key too large");
                    out.write(buffer, 0, n);
                }
                byte[] bytes = out.toByteArray();
                runOnUiThread(() -> {
                    if (destroyed) { Arrays.fill(bytes, (byte) 0); return; }
                    if (privateKey != null) Arrays.fill(privateKey, (byte) 0);
                    privateKey = bytes;
                    js("keyImported(true)");
                });
            } catch (Exception e) { status("Không đọc được key (tối đa 128 KiB).", false); }
        }, "key-import").start();
    }

    @Override public void onBackPressed() {
        web.evaluateJavascript("document.getElementById('session').hidden", result -> {
            if ("true".equals(result)) finish();
            else js("showConnectionForm()");
        });
    }

    @Override protected void onDestroy() {
        destroyed = true;
        disconnect();
        if (privateKey != null) Arrays.fill(privateKey, (byte) 0);
        web.removeJavascriptInterface("Native");
        web.destroy();
        super.onDestroy();
    }
}
