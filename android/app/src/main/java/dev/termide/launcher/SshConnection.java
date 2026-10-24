package dev.termide.launcher;

import com.jcraft.jsch.*;
import java.io.*;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.Base64;
import java.util.concurrent.*;

/** One SSH PTY. No Android dependencies, allowing JVM integration tests. */
final class SshConnection implements AutoCloseable {
    static {
        // Use the same portable implementations on Android and in JVM tests.
        JSch.setConfig("xdh", "com.jcraft.jsch.bc.XDH");
        JSch.setConfig("ssh-ed25519", "com.jcraft.jsch.bc.SignatureEd25519");
        JSch.setConfig("ssh-ed448", "com.jcraft.jsch.bc.SignatureEd448");
        JSch.setConfig("keypairgen.eddsa", "com.jcraft.jsch.bc.KeyPairGenEdDSA");
    }
    interface Listener {
        boolean trust(String host, String fingerprint);
        void output(byte[] bytes) throws InterruptedException;
        void status(String text, boolean connected);
    }

    private final Listener listener;
    private final File knownHosts;
    private final ExecutorService writer = Executors.newSingleThreadExecutor();
    private volatile Session session;
    private volatile Channel channel;
    private volatile OutputStream input;
    private volatile boolean closed;
    private volatile int columns = 80, rows = 24;

    SshConnection(File knownHosts, Listener listener) {
        this.knownHosts = knownHosts;
        this.listener = listener;
    }

    // Runs on a dedicated worker: connect and then continuously drain the PTY.
    void run(SshConfig config, String password, byte[] privateKey, String passphrase) {
        try {
            JSch jsch = new JSch();
            knownHosts.createNewFile();
            jsch.setKnownHosts(knownHosts.getAbsolutePath());
            HostKeyRepository saved = jsch.getHostKeyRepository();
            jsch.setHostKeyRepository(new HostKeyRepository() {
                @Override public int check(String host, byte[] key) {
                    int result = saved.check(host, key);
                    if (result != NOT_INCLUDED || closed) return result;
                    try {
                        String fingerprint = "SHA256:" + Base64.getEncoder().withoutPadding()
                                .encodeToString(MessageDigest.getInstance("SHA-256").digest(key));
                        if (!listener.trust(host, fingerprint) || closed) return NOT_INCLUDED;
                        saved.add(new HostKey(host, key), null);
                        return OK;
                    } catch (Exception e) {
                        return NOT_INCLUDED;
                    }
                }
                @Override public void add(HostKey key, UserInfo info) { saved.add(key, info); }
                @Override public void remove(String h, String type) { saved.remove(h, type); }
                @Override public void remove(String h, String type, byte[] key) { saved.remove(h, type, key); }
                @Override public String getKnownHostsRepositoryID() { return saved.getKnownHostsRepositoryID(); }
                @Override public HostKey[] getHostKey() { return saved.getHostKey(); }
                @Override public HostKey[] getHostKey(String h, String type) { return saved.getHostKey(h, type); }
            });
            if (privateKey != null) {
                byte[] phrase = passphrase.getBytes(java.nio.charset.StandardCharsets.UTF_8);
                try { jsch.addIdentity("imported", privateKey, null, phrase); }
                finally { Arrays.fill(phrase, (byte) 0); }
            }
            Session s = jsch.getSession(config.user, config.host, config.port);
            session = s;
            if (closed) return;
            s.setConfig("StrictHostKeyChecking", "yes");
            s.setConfig("PreferredAuthentications", "publickey,password");
            if (!password.isEmpty()) s.setPassword(password);
            s.setServerAliveInterval(15000);
            s.setServerAliveCountMax(3);
            listener.status("Đang kết nối SSH…", false);
            s.connect(20000);
            s.setTimeout(0);
            if (closed) return;
            Channel c;
            if (config.shell) {
                ChannelShell shell = (ChannelShell) s.openChannel("shell");
                shell.setPtyType("xterm-256color", columns, rows, 0, 0);
                c = shell;
            } else {
                ChannelExec exec = (ChannelExec) s.openChannel("exec");
                exec.setPty(true);
                exec.setPtyType("xterm-256color", columns, rows, 0, 0);
                exec.setCommand(config.command());
                c = exec;
            }
            channel = c;
            InputStream output = c.getInputStream();
            input = c.getOutputStream();
            if (closed) return;
            c.connect(15000);
            resize(columns, rows);
            listener.status(config.user + "@" + config.host, true);
            byte[] buffer = new byte[8192];
            int n;
            while (!closed && (n = output.read(buffer)) != -1) {
                listener.output(Arrays.copyOf(buffer, n));
            }
            if (!closed) listener.status("Phiên đã kết thúc. Mã thoát: " + c.getExitStatus(), false);
        } catch (Exception e) {
            if (!closed) {
                // Never log credentials or raw server error text.
                listener.status("Không kết nối được (" + e.getClass().getSimpleName()
                        + "). Kiểm tra mạng, đăng nhập và host key.", false);
            }
        } finally {
            if (privateKey != null) Arrays.fill(privateKey, (byte) 0);
            close();
        }
    }

    void send(String text) {
        if (closed) return;
        try {
            writer.execute(() -> {
                try {
                    OutputStream out = input;
                    if (out != null && !closed) {
                        out.write(text.getBytes(java.nio.charset.StandardCharsets.UTF_8));
                        out.flush();
                    }
                } catch (IOException e) { close(); listener.status("Mất kết nối SSH.", false); }
            });
        } catch (RejectedExecutionException ignored) { }
    }

    void resize(int cols, int lines) {
        columns = Math.max(10, Math.min(500, cols));
        rows = Math.max(2, Math.min(250, lines));
        if (closed) return;
        try {
            writer.execute(() -> {
                Channel c = channel;
                if (c instanceof ChannelShell) ((ChannelShell) c).setPtySize(columns, rows, 0, 0);
                if (c instanceof ChannelExec) ((ChannelExec) c).setPtySize(columns, rows, 0, 0);
            });
        } catch (RejectedExecutionException ignored) { }
    }

    @Override public void close() {
        closed = true;
        Channel c = channel;
        if (c != null) c.disconnect();
        Session s = session;
        if (s != null) s.disconnect();
        input = null;
        writer.shutdownNow();
    }
}
