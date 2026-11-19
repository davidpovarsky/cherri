# Unicode export diagnostic: verified local preparation

This is diagnostic preparation at source 2bd8bac35a5f2c8224d4027860f731a9fac6998e, based on S2 7b0100a2a9ce5991ca8ffe82c805278e5aa7754e. **Apple execution is NOT_RUN; no formal export acceptance is granted.**

The original signed production export run [44435457189](https://github.com/davidpovarsky/cherri/actions/runs/44435457189), at source 71225b71f5353a0f661722d154a8b670b52e76f6, returned the genuine target callback P15_EXPORT_UTF8_DECODE_FAIL. Its single mutable verdict cannot establish that earlier type/name/payload checks passed. The original fixture and oracle remain unchanged (source SHA256 073c2d4fbbe461f35b17796e4fef814f65f85a71acd0a5efa9ad0f195287b079).

The separate public fixture observes ten independent conditions and eight Base64 byte witnesses. Witnesses cover the original text, payload text, decoded File, restored text, independent literal-decode text, original-after text, composed name and status. Each witness is capped at 512 characters. The literal is 84 UTF-8 bytes and contains punctuation, a newline, Hebrew combining marks and a cherry emoji. No external/private input or arbitrary file is read. EQ and NE are distinct observations, and the literal-decode control separates the export path from the shared Decode/Get Text boundary.

## Executed checks

- One clean source-bound Windows Go build, GOMAXPROCS=1 and -p=1, exit 0 in 18.906 seconds. The source checkout remained clean. The worktree build lacks embedded VCS fields; its source binding is the recorded clean Git revision, command and binary hash, not a claim of embedded provenance.
- The actual CLI Phase0 attempted 63 fixtures: 61 produced artifacts and the two preserved F04 nested-List controls were rejected with the precise E_TARGET_VALUE_SHAPE diagnostic and no artifact. They remain BLOCKED_PLATFORM/NOT_RUN, not native passes.
- The new fixture emitted 170 native actions. The production compile phase verifies the actual original-text logging edge, envelope Encode/Decode/File-name chain, dictionary extraction, distinct literal and dynamic decode inputs, independent flag branches, bounded byte-witness branches, and dynamic callback fields.
- Fifteen independent mutations of that real emitted graph were rejected: changed Unicode input, wrong logging edge, bypassed File construction, wrong payload key, aliased decode/control inputs, aliased byte witness, changed Base64 line breaks, raised/incorrect caps, post-cap overwrite, changed NE operator, forged positive flag, unbounded summary input, and hardcoded output.
- Twenty-two Python test methods passed with no failures, errors or skips. These execute reviewed shell selection, strict callback/trace reconciliation, wrong independent flags, all eight altered byte witnesses, and the existing publication quarantine/privacy controls.
- Existing Phase0 release, F04 rejection and publication negative controls also passed. Raw reports retain their separate counts.

These checks establish the diagnostic's source/provenance, wiring and bounded output. The exact signed Apple run must still determine the export/Unicode behavior. A diagnostic result never replaces the original acceptance oracle.

The manifest binds each archived raw file. The original output directory, compiler binary and all generated fixtures remain outside this archive. Only the reviewed public diagnostic unsigned plist is included.
