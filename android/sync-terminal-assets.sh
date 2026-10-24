#!/bin/sh
set -eu
cd "$(dirname "$0")"
(cd terminal && npm ci --ignore-scripts --no-audit --no-fund)
mkdir -p app/src/main/assets/vendor
cp terminal/node_modules/@xterm/xterm/lib/xterm.js app/src/main/assets/vendor/
cp terminal/node_modules/@xterm/xterm/css/xterm.css app/src/main/assets/vendor/
cp terminal/node_modules/@xterm/xterm/LICENSE app/src/main/assets/vendor/xterm-LICENSE
cp terminal/node_modules/@xterm/addon-fit/lib/addon-fit.js app/src/main/assets/vendor/
cp terminal/node_modules/@xterm/addon-fit/LICENSE app/src/main/assets/vendor/addon-fit-LICENSE
