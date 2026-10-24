#!/bin/sh
# Isolated loopback sshd; does not change the user's authorized_keys or service.
set -eu
cd "$(dirname "$0")"
work=$(mktemp -d "${TMPDIR:-/tmp}/termide-ssh.XXXXXX")
pid=
daemon=
cleanup() {
    for p in "$pid" "$daemon"; do
        if [ -n "$p" ]; then kill "$p" 2>/dev/null || true; wait "$p" 2>/dev/null || true; fi
    done
    rm -rf "$work"
}
trap cleanup EXIT INT TERM
ssh-keygen -q -t ed25519 -N '' -f "$work/host_key"
ssh-keygen -q -t ed25519 -N 'test-passphrase' -f "$work/client_key"
cp "$work/client_key.pub" "$work/authorized_keys"
mkdir -p "$work/bin" "$work/project"
(cd .. && go build -o "$work/bin/termide" ./cmd/termide && go build -o "$work/bin/termide-server" ./cmd/termide-server)
git -C "$work/project" init -q
(cd "$work/project" && exec "$work/bin/termide-server" --daemon) > "$work/supervisor.log" 2>&1 &
daemon=$!
cat > "$work/sshd_config" <<EOF
Port 22347
ListenAddress 127.0.0.1
HostKey $work/host_key
PidFile $work/sshd.pid
AuthorizedKeysFile $work/authorized_keys
StrictModes no
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM no
AllowUsers $(id -un)
SetEnv PATH=$work/bin:/usr/bin:/bin
EOF
/usr/sbin/sshd -D -e -f "$work/sshd_config" > "$work/sshd.log" 2>&1 &
pid=$!
export TERMIDE_SSH_TEST_DIR="$work"
"${GRADLE:-gradle}" :app:testDebugUnitTest --rerun-tasks --console=plain
