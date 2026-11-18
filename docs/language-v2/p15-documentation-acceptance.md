# P15 documentation production contracts

D01 now checks the complete generated reference for all 505 native definitions
and 15 separately listed compiler macros. The previous renderer discarded
optional/default metadata for every primary parameter and quoted Number/Bool
defaults as Text. Independent schema-wide assertions reproduced that defect.
The existing generator now preserves primary metadata and typed literal
defaults, and its new --check mode compares the entire file without writing.
Eight mutations exercise the real generator executable with the original
fingerprint intact: optionality, default, callable identity, wire key, return
type, macro identity, category and extra section.

D02 now runs the public CLI check command and actual unsigned compilation for
all active canonical-guide fences. Deliberately invalid code must declare its
exact diagnostic code and must not create an unsigned artifact. The old
parse/analyze-only test missed the unsupported time trigger example; the guide
now uses the supported Wi-Fi joined declaration. Setup examples bind the
question to a real action parameter. The current local guide has 17 runnable
examples and one intentional E_ASSIGN_IMMUTABLE error.

The separate cherrilang.org helper was tested against real compiler builds:
the old cache accepted a different-commit binary and dirty source, and erased
an unmanaged directory before a failed fetch. Its repair validates source
identity, clean VCS metadata and a binary digest receipt before execution,
builds in an owned stage, reserves new destinations exclusively and preserves
occupied caches. Its actual 12 controls and 37 example checks have separate
docs-repository/immutable-compiler receipts. Generated signatures and four
explicitly labelled Legacy reference blocks are not counted as runnable code.

The Skill search script formerly searched all historical pages by default;
an actual shell control reproduced the retired-syntax recommendation.
Default search now selects only fork-language/. Historical pages require the
explicit --legacy-reference option. Missing active docs fail closed and real
grep errors remain errors. Five actual shell controls verify this boundary.
D03 remains UNVERIFIED: these controls do not prove the published site's
search index, the installed Skill or iOS application behavior.

Working-tree validations precede the implementation commit. Clean-SHA receipts
are added separately. No Apple runtime, publication, installed-consumer or
overall release completion is inferred from these documentation contracts.
