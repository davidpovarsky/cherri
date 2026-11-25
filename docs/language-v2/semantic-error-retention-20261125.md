# Preserve all semantic compile errors — 2026-11-25

## Reproduced failure

The frozen differential gate at ca26086ba81ed7247b7b009aff2f59d5943dc9d6 failed BRG33 because its actual child TestP15T04OpaqueTypeProduction/date_quantity failed. Its collector already uses permissive JSON decoding, so the Go OutputType incompatibility did not cause this failure.

The unchanged negative source binds unit to text("hr") and compares CurrentDate against qty(3, unit). Analysis correctly reports both:

1. E_LITERAL_REQUIRED on the bare runtime unit reference, from the new shared constructor validator.
2. E_ARGUMENT_TYPE on the complete qty call, from the existing requirement that a date comparison prove a scalar date duration.

CompileSourceToPlist returned only the first analyzer error. The enclosing comparison's existing diagnostic was therefore hidden from the public compile result even though it remained present in analysis and the service. The original test correctly requires zero native bytes and the original comparison error. The same masking defect also hides a second independent invalid typed declaration.

## Bounded repair

A shared semanticCompileErrors helper returns every error-severity diagnostic in original analyzer order using errors.Join. Both public V2 compilation and the existing isolation error collector reuse it. A single error keeps its previous message text. Warning diagnostics do not become errors, and a valid compilation continues normally.

The analyzer, constructor contracts, date-duration proof, original tests and exact diagnostic spans are unchanged. Invalid source is still rejected before a native backend session is created. The existing isolation list of erroneous source lines is unchanged; its accompanying error now reports the same complete diagnostic set as compilation.

## Focused evidence

Evidence directory: /mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/go-events.

- t04-red.log, SHA256 4a9e867cf8ab145c9d79f49272e2272fb03bee0dbd15b9d855335a1b48818434: the exact frozen T04/date_quantity failure. Other T04 production subcases passed.
- semantic-errors-red.log, SHA256 59b37206994aaad9876ca013761b2a2481499aede101b2d74d9bdfb95ee11a5a: new controls demonstrate hidden errors for the constructor/comparison pair and for two independent statements. The single-error and success controls already pass.
- semantic-errors-green.log, SHA256 e5111157b0081ae70e56f53011aa2adb08cf3e5b0678be148675b4cce382d5a9: the unchanged T04 production and service tests pass, alongside both new public-API controls and all four constructor-validation tests. This checks that exact rejection contracts, valid native compilation, literal color values, quantity units/named arguments, and authored unit templates survive the reporting repair.
- git diff --check passed. No original test assertion or expected fixture output was changed. No native execution or full-suite acceptance is claimed.

The separate Go event repair also passed its originally failing clean-source RP50 producer: rp50-green.log, SHA256 b11d7567e5645d8d353eeda99e2c36aa5045802f0207a3dccc96d91de090f42a, executed on b38bdcf07905a2ce87333f6c64977c5f32406cec. Its nine evidence-corruption controls passed and AI quality evaluation remained unexecuted.
