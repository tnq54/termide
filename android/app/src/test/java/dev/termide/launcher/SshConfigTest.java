package dev.termide.launcher;

import static org.junit.Assert.*;
import org.junit.Test;

public class SshConfigTest {
    @Test public void projectCannotInjectShellCommands() {
        SshConfig c = new SshConfig("example.org", 22, "dev", "/srv/a'b;$(id)", false);
        assertEquals("export PATH=\"$HOME/bin:$HOME/.local/bin:$PATH\"; cd '/srv/a'\"'\"'b;$(id)' && exec termide --local --project '/srv/a'\"'\"'b;$(id)'", c.command());
    }
    @Test public void shellDoesNotNeedProject() {
        assertTrue(new SshConfig("example.org", 2222, "dev", "", true).shell);
    }
    @Test public void invalidConnectionDetailsRejected() {
        for (int port : new int[]{0, -1, 65536}) {
            assertThrows(IllegalArgumentException.class, () -> new SshConfig("host", port, "dev", "/srv", false));
        }
        assertThrows(IllegalArgumentException.class, () -> new SshConfig("", 22, "dev", "/srv", false));
        assertThrows(IllegalArgumentException.class, () -> new SshConfig("host", 22, "", "/srv", false));
        assertThrows(IllegalArgumentException.class, () -> new SshConfig("host", 22, "dev", "relative", false));
    }
}
