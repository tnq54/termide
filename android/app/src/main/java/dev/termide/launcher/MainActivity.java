package dev.termide.launcher;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.os.Bundle;
import android.widget.Toast;

/**
 * Launcher that asks Termux:Task to open a Termux session running the
 * on-device termide binary. Host/project come from the launching intent
 * extras when present, otherwise from BuildConfig defaults baked at build
 * time (-PtermideHost / -PtermideProject).
 *
 * <p>Integration contract (Termux:Task plugin): action
 * {@code com.termux.app.RUN_COMMAND} with extras
 * {@code com.termux.app.run_command.path} (executable),
 * {@code .arguments} (String[]), {@code .background} (boolean) and
 * {@code .session_action} ("0" = new foreground session).
 */
public final class MainActivity extends Activity {

    static final String ACTION_RUN_COMMAND = "com.termux.app.RUN_COMMAND";
    static final String EXTRA_PATH = "com.termux.app.run_command.path";
    static final String EXTRA_ARGUMENTS = "com.termux.app.run_command.arguments";
    static final String EXTRA_BACKGROUND = "com.termux.app.run_command.background";
    static final String EXTRA_SESSION_ACTION = "com.termux.app.run_command.session_action";
    static final String TERMIDE_BIN = "/data/data/com.termux/files/usr/bin/termide";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Intent starter = getIntent();
        String host = starter != null ? starter.getStringExtra("host") : null;
        String project = starter != null ? starter.getStringExtra("project") : null;
        if (host == null) {
            host = BuildConfig.TERMIDE_HOST;
        }
        if (project == null) {
            project = BuildConfig.TERMIDE_PROJECT;
        }
        Intent run = new Intent(ACTION_RUN_COMMAND);
        run.setClassName("com.termux", "com.termux.app.RunCommandService");
        run.putExtra(EXTRA_PATH, TERMIDE_BIN);
        run.putExtra(EXTRA_ARGUMENTS, new String[] {"--ssh", host, "--project", project});
        run.putExtra(EXTRA_BACKGROUND, false);
        run.putExtra(EXTRA_SESSION_ACTION, "0");
        try {
            startService(run);
        } catch (SecurityException | IllegalStateException e) {
            toast("Termux:Task missing or permission denied: " + e.getMessage());
        } catch (ActivityNotFoundException e) {
            toast("Install Termux and Termux:Task first");
        }
        finish();
    }

    private void toast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show();
    }
}
