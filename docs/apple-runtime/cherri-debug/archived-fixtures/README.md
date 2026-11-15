# Historical Cherri debug fixture sources

These ad-hoc integration/negative-test sources were moved from tests/ so the Go test harness (which enumerates runnable .cherri files) does not try to compile intentionally invalid or superseded samples.

The maintained Apple iPad demo is docs/apple-runtime/cherri-debug/samples/debug-previous-only-api-ipad.cherri.
Go unit tests in debug_stdlib_test.go and debug_timer_native_test.go cover supported compiler macro semantics.

No source was deleted.
