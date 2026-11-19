# Source-bound b34 CI failure and F04 consumer repair

The [original Build & Test run](https://github.com/davidpovarsky/cherri/actions/runs/44443478882) is FAILURE at `b34ae47ba79d15c08e8a8bdec9a761a7a4b722bb`. Linux Full Go ended at 02:27:03 UTC after 6m50s; macOS ended at 03:12:32 after 8m38s. Nix passed. A stale watcher did not reflect those completions. The full logs contain no observed test timeout or panic.

`summary.json` enumerates every failed top-level/subtest name, including the downstream K01 producer failure. The raw public GitHub job logs and extracted contexts are preserved byte-for-byte. K01 causal diagnosis remains open in this receipt; the log reports self-test exit 1 after the existing-Shortcut prepare/edit progress marker.

`f6de7901f0397da32a691d9fb0d7c8f557028db8` corrects the stale F04 unit consumer. A record and a record with a List field retain one native Dictionary body producer and one input List producer. The original List-valued body now requires the exact `E_TARGET_VALUE_SHAPE` diagnostic and zero output bytes. The compiler guard and signed native failure oracles are unchanged. An exact clean-source gate passed 33 named cases, with no failures or skips. It covers the corrected consumer, known List result rejections, supported/explicitly unverified boundaries, and the original F04 probe rejection consumers.

This unit repair does not declare nested List cardinality implemented. Actual nonempty and empty native F04 failures remain preserved in the native follow-up archives. No Apple execution or replacement full-CI pass is claimed by the local test receipt.
