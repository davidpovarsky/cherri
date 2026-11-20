# R4 Legacy parity and preserved standard-function literal

## Verified results

The unchanged official differential runner passed on clean source `300aaaddd9fe1421c87789c6a4743e835f226ad1`: 47 of47 fixtures (9 exact verified results and38 proven equivalences), zero baseline, migration, V2 compilation or semantic differences, and17 of17 action cases. This includes `globals.cherri`, `standard.cherri` and `menus.cherri`. A fresh Windows Go1.25.6 CLI build was used;16 runner controls also passed without skips. Original commands, exact executable hash, complete stdout/stderr and per-fixture results are in the archive.

A separate stdfunc run on that same source failed6 PASS/3 FAIL events: the two alias cases and their parent expected the replacement string to be `#`. Those failures are retained. They are not described as a compiler regression.

## Independent oracle correction

The preserved `stdfunc.cherri` contains the one-backslash Legacy literal `"\u0023"`. The original Legacy lexer handles newline, carriage-return and tab escapes and discards the backslash on unknown escapes, so its actual emitted replacement is `u0023`. The exact source is unchanged from d7052f4568db26dba169bdefc72623e48c04968a (SHA2566a5586d0d5f9283472a4c2a286437db74888c2ea6970e1beaf202ad218b78cab).

Independent public-CLI invocations using the retained original d705 binary and the fresh source300 Legacy binary emitted that same literal. The current V2 import emitted it too. The archive contains the original plists, commands, exact file hashes, peer receipt and root independent audit. The historical build notification job_37e198f0df5b identifies the original source, successful build and binary path; the executable has no embedded VCS stamp. No stronger binary attestation is claimed.

Tests-only source `ef9e27ec73c70ca5ddd0b70ef3b3d6a2696b753b` changes that one incorrect expectation, adds an explicit mutation rejecting reinterpretation as `#` for both import aliases, and asserts the literal independently in each real Legacy/V2 graph before the full differential. No library source, generated snapshot, lexer, migrator, comparator or golden was changed.

On that clean commit the exact stdfunc command passed41 named outcomes in18.109s,0 failures/skips; the complete migration package passed19 in2.469s,0 failures/skips. Raw SHA256 values are1bb17c85bd0d2f2160d56e559fe4d63f5f60f8d72fb7fe2bdd64696257e5a65c and11670d259ebf990815cd229fd6b6cd42581536f4e734c0dba1db2d8690ac3c61 respectively.

## Evidence and remaining gates

The archive contains42 original files and was reopened and checked byte-for-byte against every input. Its SHA256 is `665cbd2dac9b02d398f72ff087569602165b169776c2bb24f71adf71ce027104`; the manifest SHA256 is `2860a68f66c9347b90c926dd119aeec8a502cb9304bd943111c5a5232c674d95`. The original failed run, old baseline report and empty original build log are retained, with no old failure relabelled.

These are Windows compilation, structural and test-oracle results. JavaScript behavior and Apple action execution were not exercised. The Legacy task stays open until the final integrated SHA passes Linux/macOS/Nix CI and receives fresh formal179 evaluation. Earlier Apple failures, external acceptance gaps and the original staging branch are unaffected.
