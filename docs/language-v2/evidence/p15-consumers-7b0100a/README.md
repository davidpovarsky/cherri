# Coherent S2 compiler and D2 documentation tuple

Compiler S2 is 7b0100a2a9ce5991ca8ffe82c805278e5aa7754e. Documentation D2 is daa58850140bc1d739806633932d01e084463404. Both immutable staging refs were pushed and read back exactly.

The documentation runner fetched and built a clean ordinary compiler checkout with exact VCS revision and vcs.modified=false. Its real binary passed 38 documentation examples (35 runnable, 3 required diagnostic/compile rejections) and 12 cache provenance controls. All 524 reference-only snippets remain explicitly unexecuted reference text. The complete canonical action-reference body was regenerated and matched byte-for-byte while retaining site frontmatter; native schema, codec and macro fingerprints remain unchanged.

The committed Skill compatibility manifest binds these exact S2/D2 revisions. This is a local tuple proof, not a deployment or complete project acceptance. Docs CI44453561234 was dispatched and is recorded separately until its actual result is inspected. Fresh source/runtime packages, unpacked execution, Remote Builder gates, live activation and final all179 acceptance remain separate. The earlier K01 path failure and its actual no-override packaged repair are retained in the neighboring k01-tempdir-015ac58 archive.
