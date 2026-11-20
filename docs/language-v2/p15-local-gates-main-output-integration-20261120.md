# Local gate integration with explicit main output

Implementation: 718e7a16ce0bfb1d5b3d132df06ab96f6c9793e9. This source retains the original root M02 compiler ancestry, the BRG18/BRG21/RP52/BRG30 component ancestry, and the separate M02 local producer ancestry. It does not duplicate or rewrite their original commits or evidence.

## Verified result

A clean selected fast replay passed **596 named tests**, with zero failures or named skips, in **12.609 seconds**. The actual four-producer replay passed **702 outer named tests**, with zero failures or skips, in **110.000 seconds**. Both processes recorded the same clean implementation before and after execution.

| Actual fresh producer | Required inner named PASS | Immutable source files | Local tiers | Seconds |
| --- | ---: | ---: | --- | ---: |
| BRG18 | 54 | 70 | native-roundtrip, integration | 10.182 |
| BRG21 | 33 | 64 | security, integration, service | 12.690 |
| M02 | 172 | 76 | native-structure | 16.345 |
| RP52 | 98 | 73 | integration, repository | 26.220 |

The inner counts describe separate complete production selections; they are not additional formal requirement counts. The independent intake checked all **140 distinct source blobs** from the four process receipts against immutable Git objects, all raw and artifact hashes, exact required/discovered event sets, process outcomes and source identities. The proof additionally binds the two changed BRG30 inventory files, for **142 source witnesses**.

BRG18, BRG21 and RP52 each pass their own local selected requirement. M02 has only its actual native-structure record: its local report remains **NOT_RUN, zero accepted cases, one skipped case**, with the original iOS obligation unmet. Every global closure flag remains false.

## Integration repair and preserved failure

The clean ancestry merge at 46d6a25daddb5101110f1376cc65dbd767407ec2 genuinely failed its first Go invocation. The compiler reported exactly three missing arguments in the new RP52 test's calls to shared graph helpers. No named test executed. Original JSONL SHA256: 216bda6e557c939d9a96e06ff21766331b434a494a8d8bab8b06d34b6b3d01a0.

The narrow repair supplies the expected main-output mode from each authored public route. V2 graphs include their explicit terminal Nothing; selected Legacy graphs retain their original implicit result. The full graph and wrong-reference assertions remain, and a new opposite-mode rejection checks that a returned mode flag cannot select the oracle.

The critical source inventories now include the shared M02 analyzer, lowerer, migration, syntax and module helpers. BRG18 uses 70 paths, BRG21 64, RP52 73, BRG30 74 and M02 76. A small acquisition adapter reuses the existing immutable Git reader in batches of at most 64 objects, only after enforcing the unchanged 4 MiB per object and 16 MiB aggregate limits over the entire request. Real cross-batch source, counterfeit second-batch digest, cache isolation, path and resource-boundary controls passed.

No compiler semantics, codec, immutable requirement contract or consumer manifest was changed by this repair. Merged component behavior remains pinned to its original ancestry.

## Evidence and limits

Fast original JSONL: 699,855 bytes, SHA256 19eed74c77bba6eb06362c67b553a541de20732ced7d0a339e8a4382a66e5703.

Actual outer original JSONL: 833,419 bytes, SHA256 12f28d6cde82407325a4754236af68b13b4f34b61a19e4cd75eda24a25b9f6bf.

The evidence directory retains 30 exact original process artifacts, original merge/source receipts, the source patch and separately labeled derivative intake records. The first derivative audit's schema-assumption error is preserved; it did not change any test result or original byte.

The root compiler component at 510ecfdd was separately reviewed against proof 8f82f586: all 144 archived originals and 67 Git witnesses matched. Its expanded affected process remains failed with the original JSON optional-absence leaf and parent. This selected integration replay does not repair or relabel that independent failure.

BRG30's complete 32-minute scenario remains proved only at c83007d8, with its original proof f4218c8 retained. This integration ran its inventory/parser/budget controls but did not repeat or promote that historical full scenario at the new source. Apple execution, published consumer compatibility, full Go/CI and final 179-row acceptance remain separate open work.
