# Unchanged Date diagnostic: verified native result

The original 27-field public Date fixture passed on source `55ba009aa66810aedda466c12680b84d00b59766` in [run 44705639934](https://github.com/davidpovarsky/cherri/actions/runs/44705639934), attempt 1. The completed run and its original callback, ZIP, publication references and signing provenance were retained. The unchanged source auditor returned `PASS_BOUNDED_NATIVE_CONTRACT` with exit 0 and verified all 22 published references.

## Actual values

| Observation | Type | Count | Exact value |
| --- | --- | --- | --- |
| Start and same specified Date | Date | 1 each | 2001-02-03T04:05:06Z |
| Later specified Date | Date | 1 | 2001-02-03T04:06:06Z |
| Equal Dates, Seconds | Number | 1 | 0 |
| Later Date, Seconds | Number | 1 | 60 |
| Equal Dates, Minutes | Number | 1 | 0 |
| Later Date, Minutes | Number | 1 | 1 |
| Same producer, Seconds | Number | 1 | 0 |
| Independent numeric zero | Number | 1 | 0 |

Every one of the 27 original Base64/value/count fields matched in its original order. The source fixture and expected JSON are byte-identical to the earlier failed diagnostic source `7f044196d042dcdbbcc7401dcc96177f059b0db3`. Original run `44680412319` remains failed; its missing interval outputs were not rewritten. The repaired production Date codec is commit `e259c829fa0345702e0989a25b5bbac1296d1fa6`: the two Date fields use the existing native token-string reference encoding.

## Original evidence and strict intake

The original artifact is `14616400322`, 55,746 bytes, SHA256 `d496122e3b4214820038842d10717faf21b0ad80e0dda3302a17872d3de2c481`. The exact-source auditor SHA256 is `0816dfe220fd3d13cd067ac21756ff4f6ba214810113148ffae6e8748c5a3a2e`; its actual stdout SHA256 is `00a0e0e0733d3d85675eb179974adfac4d604c492d348925057655a9e39ac2c1`. Source, expected contract, unsigned artifact, nonce, callback, all original published hashes and the auditor's causal rejection controls are recorded in [the machine-readable proof](evidence/date-runtime-fixed-44705639934.json).

The successful callback was observed at `2026-11-20T09:08:55.664Z`. Import, execution, correlated callback and required trace gates passed. Editor capture remained partial. This result proves the bounded Date diagnostic on this component source. It does not close the twelve-case label matrix, Date log retention, physical-device testing, D03, per-action type coverage or the final 179-requirement candidate. No formal requirement memberships were added.
