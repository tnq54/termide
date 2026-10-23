# Remote editor — execution ledger

Scope: tasks 5 and 10 of the approved IDE plan. Implement one connected flow:
SSH workspace → file browser → editor → explicit save with conflict detection.

- [x] Root-scoped text files, bounded reads/listing, version-checked atomic saves.
- [x] File operations through server protocol.
- [x] Keyboard-driven browser/editor in Bubble Tea, save/error/dirty states.
- [x] Integrated server/SQLite/TUI tests and architecture build checks.

Ruling: raise minimum Go version from 1.24 to 1.25 to use os.Root.Rename for
atomic saves without abandoning traversal-resistant directory handles. Cost:
builders and CI require Go 1.25 or newer; shipped binaries do not require Go.

Runtime scope is unchanged. This work does not mark Start/approval/LSP complete.

Ruling: use a source-preserving Input model instead of Bubbles textarea: its
sanitizer changes tabs/CRLF. Cost: selection and clipboard integration remain
future work; basic editing, navigation and undo/redo are implemented here.

Independent review found three issues, fixed with RED→GREEN regression tests:
quit during a pending save after undo; standalone CR in paste; null/missing
text accepted as an empty save. Existing bare-CR documents are rejected rather
than rendered incorrectly. No deferred review findings.

Verification: complete Go suite, race tests for editor/fs/server/ui, vet,
Go 1.25 targeted suite, Android arm64 client and Linux amd64 server builds.
Android binary remains untested on a physical phone.
