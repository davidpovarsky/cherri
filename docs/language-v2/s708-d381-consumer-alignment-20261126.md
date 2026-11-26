# S708, D381 and authoring Skill alignment

This component binds the actual external compiler pair, documentation and authoring Skill manifest. The final implementation source **M remains a separate release decision**. Final source/runtime archives must take Skill files and `skillCommit` from M; their compiler and migrator remain built from S708.

## Source and artifact identities

| Component | Exact identity |
| --- | --- |
| External compiler S | `708ed78eda1cad0f1f453559afce100edc59b8a9` |
| S tree | `95cedc6d55514907f853ecc326ccd3afe98bd9f7` |
| Docs D | `381de199405b8acbdf465e3f1420b01070c57f20` |
| D tree | `bfd3ff576d16d30e1d6cef6e15bbb7a62bcd2137` |
| Qualified authoring K | `e96a363e9cb7e7347e383e873c43f9f7382cbeed` |
| Compatibility manifest | `3afbda0784fa3eddf1fcb8e9e06f56aa7a7353694267887f40236015c07d05c9` |
| ARM64 Linux compiler | `21cd060ebe4e32ba94f6ac0581cd742094d83bb8a71e2ff4068a78d79b35f5a6` |
| ARM64 Linux migrator | `f0507d9f6338b29b8c3e78a4144745bf45cf3f1928bf1e3ed8ae8a1b47b4ac9a` |
| Generated reference body | `370ccff0d1d2708117522d847ebd306124751d7cfb4968fc9c4dcadb38333a20` |
| Macro fingerprint | `bfa446ea152ef48e9a2b7f7177ef0c54409bc98bc2d1dfcf9b8e282e1f3c580e` |

The compiler and migration tool were actually built once with Go1.24.13, `CGO_ENABLED=0`, `-buildvcs=true -trimpath -ldflags='-s -w'`. Both have exact S708 clean embedded VCS identity and expected tool/module/platform roles. The original stage runtime inspector accepted the pair. Independent intake rechecked 7,787 physical Git inputs and 23 original process receipts. The ordinary build checkout is shallow; this component does not claim an ancestry test from that shallow checkout.

## Documentation evidence

[Docs workflow 45747139317](https://github.com/davidpovarsky/cherrilang.org/actions/runs/45747139317) completed successfully on exact D381.

- 46 authored examples executed: 43 compiled native workflow plists and 3 exact diagnostic/compile rejections, through 92 actual CLI commands.
- 525 reference-only fences remain explicitly unexecuted. They are not counted as runnable examples.
- The original generator and `--check` succeeded. Four selected Go parents and ten causal child controls passed: fourteen exact test identities.
- Generated, compiler-committed and Docs reference bodies agree in full bytes: 505 native actions, 16 macros, 521 headings and the unchanged 119-byte frontmatter.
- The Jekyll site and search built successfully. Six real artifact corruption controls passed, 87 local links were verified, and no historical route entered default search.

The preceding workflow 45745574747 remains failed. Its actual fourteen Go tests succeeded but its then-current receipt verifier still expected twelve. D381 adds the two exact source-authored children, retains the original twelve identities, and preserves strict unique event, package, time and parent/child ordering checks. Five focused Python methods and eight causal mutations qualified the repair. Their original GREEN owner receipt remains `UNEXPECTED_RESULT` because its footer detector looked for literal backslashes; independent intake consumes the original successful process/stream without rerunning or relabeling that owner result.

All three successful GitHub artifact ZIPs and all original failed-run evidence are retained in the companion archive. The intake reconstructs all 571 classified fence records from D381 source and verifies the source, artifact and stream hashes. It does not invent missing native execution evidence.

## Existing iOS producer

The unchanged `ios/scripts/build_documentation_index.py` generated and verified the offline index using the new manifest. The original ordinary Docs checkout contains four exact pinned sources. The result has 569 sections and 546,645 bytes, SHA256 `f6f071b9786903cb8774298b6a7c71d905fa725fc03f28a05f155e4416f394f4`.

This resolves the concrete documentation fingerprint mismatch observed before any app build in the J708 iOS run. It is a documentation producer result, not a native app/XCTest/runtime result.

## Consumer scope and remaining gates

The reviewed OpenMinis workflow already separates current implementation builds from manifest-pinned runtime builds. Final archives use source M, runtime S, and docs D. Skill installers read the same manifest and retain source/role/checksum and rollback verification. The dated12f capture adapter and old signer-source observer fixture remain historical and unchanged. CI owns the separate BRG29 nested documentation inventory repair.

The live K18 installation and independent Pure/Pack routes were not changed. Required next steps are an exact M source, fresh current-installation capture, original staging/package verification, all39 entrypoint and58 signing controls, and original live activation/rollback/reapply. Site/main publication and full179 acceptance remain separate gates.

## Original receipt hashes

| Receipt | SHA-256 |
| --- | --- |
| Actual S708 pair build | `b85ef4cdaf6cfd5e219ee0db72e7e68d8e41bc9b5338fe62a4d6231018c16e90` |
| Pair independent intake | `3923502edaf6f65ea9c0f93a757f3d71b1bdd4b54578a6bc42dbf346969c8433` |
| Successful Docs acquisition | `434f66b517758eca5a331dc56503bf1139d59792cce53e56bfb63a21b3370d9f` |
| Docs independent intake | `3d4fcfae404c54d3e9549bc2c47dda4030bad53e4a514bd56ce10ac6a778a846` |
| Original14 event correction intake | `2375cf4397d3e5e09fe946bfd506e4ca5047fd0fd9bad722e8995629c6ab2fbd` |
| iOS documentation preflight | `315d8947f60bff494e7bc962c39e4698109b638ccbe80ac1a2f81c4a8b097939` |
| Current consumer source audit | `9fe6c2e1202df08f3bc0e053b039deaa03d1cd92807829961f2971ecd602953c` |
| Final M stage plan v2 | `8bff677398a1d2711f407a5f1dddb1a064e1e074f1b6ccbc68abdf9955a9fb9e` |

The public originals and their exact inventory are stored under `docs/language-v2/evidence/s708-d381-consumer-alignment-20261126/`. Byte-exact sealed binary backups are outside the checkout at `/mnt/development/projects/preserved/cherri-docs-consumers-20261126/final-S708-compiler-pair-01/`. No private corpus input is in this component.
