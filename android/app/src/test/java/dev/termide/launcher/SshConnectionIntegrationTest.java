package dev.termide.launcher;

import org.junit.Test;
import static org.junit.Assert.*;
import static org.junit.Assume.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

public class SshConnectionIntegrationTest {
    @Test public void realSshKeyAuthTrustPtyResizeAndUnicode() throws Exception {
        String dir = System.getenv("TERMIDE_SSH_TEST_DIR");
        assumeTrue("Run android/test-ssh.sh to enable real SSH integration", dir != null);
        File known = new File(dir, "client_known_hosts");
        Files.deleteIfExists(known.toPath());
        AtomicInteger prompts = new AtomicInteger();
        for (int attempt = 0; attempt < 2; attempt++) {
            ByteArrayOutputStream output = new ByteArrayOutputStream();
            CountDownLatch ready = new CountDownLatch(1);
            SshConnection c = new SshConnection(known, new SshConnection.Listener() {
                public boolean trust(String host, String fingerprint) {
                    assertTrue(fingerprint.startsWith("SHA256:"));
                    prompts.incrementAndGet(); return true;
                }
                public void output(byte[] bytes) { synchronized(output) { output.write(bytes, 0, bytes.length); } }
                public void status(String text, boolean active) { if (active) ready.countDown(); }
            });
            byte[] key = Files.readAllBytes(Path.of(dir, "client_key"));
            Thread runner = new Thread(() -> c.run(new SshConfig("127.0.0.1", 22347,
                    System.getProperty("user.name"), "", true), "", key, "test-passphrase"));
            try {
                c.resize(91, 37);
                runner.start();
                assertTrue("SSH authentication failed", ready.await(20, TimeUnit.SECONDS));
                c.send("stty size; printf '\\125\\156\\151\\143\\157\\144\\145: 😀'; exit\r");
                runner.join(15000);
                assertFalse("SSH session did not exit; output=" + output.toString(StandardCharsets.UTF_8)
                        + "; stack=" + java.util.Arrays.toString(runner.getStackTrace()), runner.isAlive());
                String text = output.toString(StandardCharsets.UTF_8);
                assertTrue(text, text.contains("37 91"));
                assertTrue(text, text.contains("Unicode: 😀"));
            } finally { c.close(); runner.join(3000); }
        }
        assertEquals("saved host key should not prompt again", 1, prompts.get());
    }

    @Test public void rejectingUnknownHostStopsAuthentication() throws Exception {
        String dir = System.getenv("TERMIDE_SSH_TEST_DIR");
        assumeTrue(dir != null);
        CountDownLatch prompted = new CountDownLatch(1);
        AtomicInteger connections = new AtomicInteger();
        SshConnection c = new SshConnection(new File(dir, "rejected_known_hosts"), new SshConnection.Listener() {
            public boolean trust(String host, String fingerprint) { prompted.countDown(); return false; }
            public void output(byte[] data) { }
            public void status(String text, boolean active) { if (active) connections.incrementAndGet(); }
        });
        try {
            c.run(new SshConfig("127.0.0.1", 22347, System.getProperty("user.name"), "", true),
                    "", Files.readAllBytes(Path.of(dir, "client_key")), "test-passphrase");
            assertEquals(0, prompted.getCount());
            assertEquals(0, connections.get());
        } finally { c.close(); }
    }

    @Test public void execStartsRealLocalTermideAndOpensProject() throws Exception {
        String dir = System.getenv("TERMIDE_SSH_TEST_DIR");
        assumeTrue(dir != null);
        CountDownLatch dashboard = new CountDownLatch(1);
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        SshConnection c = new SshConnection(new File(dir, "ide_known_hosts"), new SshConnection.Listener() {
            public boolean trust(String host, String fingerprint) { return true; }
            public void output(byte[] bytes) {
                synchronized(output) {
                    output.write(bytes, 0, bytes.length);
                    if (output.toString(StandardCharsets.UTF_8).contains("(no tasks)")) dashboard.countDown();
                }
            }
            public void status(String text, boolean active) { }
        });
        byte[] key = Files.readAllBytes(Path.of(dir, "client_key"));
        Thread runner = new Thread(() -> c.run(new SshConfig("127.0.0.1", 22347,
                System.getProperty("user.name"), dir + "/project", false), "", key, "test-passphrase"));
        try {
            runner.start();
            assertTrue("Local TUI did not open workspace", dashboard.await(20, TimeUnit.SECONDS));
            c.send("\u0003");
            runner.join(10000);
            assertFalse("TUI did not exit; output=" + output.toString(StandardCharsets.UTF_8)
                    + "; stack=" + java.util.Arrays.toString(runner.getStackTrace()), runner.isAlive());
        } finally { c.close(); runner.join(3000); }
    }

    @Test public void changedHostKeyIsRejectedWithoutTrustPrompt() throws Exception {
        String dir = System.getenv("TERMIDE_SSH_TEST_DIR");
        assumeTrue(dir != null);
        // The client key is deliberately NOT the server's host key.
        String[] wrongKey = new String(Files.readAllBytes(Path.of(dir, "client_key.pub")), StandardCharsets.UTF_8).split(" ");
        File known = new File(dir, "changed_known_hosts");
        Files.write(known.toPath(), ("[127.0.0.1]:22347 " + wrongKey[0] + " " + wrongKey[1] + "\n").getBytes(StandardCharsets.UTF_8));
        AtomicInteger prompts = new AtomicInteger(), connected = new AtomicInteger();
        SshConnection c = new SshConnection(known, new SshConnection.Listener() {
            public boolean trust(String host, String fingerprint) { prompts.incrementAndGet(); return true; }
            public void output(byte[] bytes) { }
            public void status(String text, boolean active) { if (active) connected.incrementAndGet(); }
        });
        try {
            c.run(new SshConfig("127.0.0.1", 22347, System.getProperty("user.name"), "", true),
                    "", Files.readAllBytes(Path.of(dir, "client_key")), "test-passphrase");
            assertEquals(0, prompts.get());
            assertEquals(0, connected.get());
        } finally { c.close(); }
    }
}
