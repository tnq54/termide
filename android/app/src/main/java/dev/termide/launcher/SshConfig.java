package dev.termide.launcher;

final class SshConfig {
    final String host, user, project;
    final int port;
    final boolean shell;

    SshConfig(String host, int port, String user, String project, boolean shell) {
        this.host = host.trim();
        this.user = user.trim();
        this.project = project;
        this.port = port;
        this.shell = shell;
        if (this.host.isEmpty() || this.user.isEmpty() || port < 1 || port > 65535
                || this.host.matches(".*\\s.*") || this.host.indexOf('\0') >= 0
                || this.user.indexOf('\0') >= 0 || project.indexOf('\0') >= 0
                || (!shell && !project.startsWith("/"))) {
            throw new IllegalArgumentException("Nhập host, user, port 1–65535 và đường dẫn project tuyệt đối.");
        }
    }

    String command() {
        String path = "'" + project.replace("'", "'\"'\"'") + "'";
        return "export PATH=\"$HOME/bin:$HOME/.local/bin:$PATH\"; cd " + path
                + " && exec termide --local --project " + path;
    }
}
