# S3 Unicode and binding callback failures

Both runs compiled, signed and imported their exact synthetic fixtures and
received one fresh correlated SUCCESS callback. Both failed the mandatory
runtime/output contract because the returned semantic bytes differed.

Unicode run 44459407425 observed TYPE=N, NAME=Y, COUNT=N, STATUS=Y, PAYLOAD=N,
RESTORED_EQ=N, RESTORED_NE=Y, LITERAL_EQ=Y, LITERAL_NE=N, UNCHANGED=Y.
The first payload and decoded-file byte witnesses are empty. Original,
independent literal decode, and post-export original witnesses agree.
The restored-text witness decodes to a UUID-shaped string that has not been
identified as an exact native producer. This is not a Unicode acceptance pass.

Binding capture job 135342271270 in run 44459414583 returned only the expected
prefix with an empty encoded Clipboard observation. Before/after trace events
exist. The original result Text action has a mandatory literal prefix, so this
observation does not independently locate the failure inside capture semantics
or the later Clipboard observer. The typed-defaults job is a separate outcome
and is not reported by this archive.

The source-level empty debug-log seed is a causal hypothesis: Text("") is set
before the first append, and the diagnostic reads the first exported row. Native
entry count and first/last payload observations are required before attributing
the failure to that seed. No production behavior or expected result is changed
by this evidence commit.

Each downloaded artifact's complete published-file inventory and digest was
checked against its publication manifest. Raster/raw inputs remained quarantined.
The ZIPs are the exact already-published synthetic artifacts, not a pixel
redaction proof. Physical-device execution remains blocked.

The local structural reproduction used the retained clean S3 Windows compiler
(SHA256 439eff31c429874506bff1a18b3d19177f08127ece69f296dcb6a3329a3c6689).
Its source bytes match the CI fixture hashes, but its random UUIDs and unsigned
plist hashes differ. These local plists are diagnostic graph reproductions,
not replacements for the source-bound signed native receipts.
