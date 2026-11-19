# Remote promotion fault tests at f6e6194

Source commit: `f6e6194a27a4e40265ff7946189344b8e718a46e`.

The exact committed Python source bytes were replayed on a POSIX filesystem with Python 3. The suite passed 15 top-level tests, including actual file replacement, byte/mode/relative-link preservation, exclusive locking, interrupted-write journal recovery, SIGTERM boundaries, stale concurrent state rejection, canonical Git origin retention, and the manual CLI reapply logging-failure regression. The execution receipt binds the tested source file hashes and complete test log.

Evidence validators were tested with synthetic complete report models plus deliberate corruptions. The actual archived intermediate entrypoint report with 36 PASS rows and one NOT_RUN row is a mandatory rejected control. Complete synthetic models are unit inputs, not release acceptance receipts.

The manual reapply regression invokes the CLI in a forked process, injects an events.jsonl ENOSPC failure after the actual runner replacement, and requires a nonzero process outcome with the original files, modes, and relative symlink restored.

No live Remote Builder activation, external signing acceptance, Apple cryptographic verification, or Apple device execution is established by these unit tests. Promotion still requires the exact independently produced stage, entrypoint, fault, package-integrity, and candidate smoke receipts described in the protocol.
