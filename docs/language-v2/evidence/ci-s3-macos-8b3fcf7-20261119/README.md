# Historical S3 macOS full-Go job

Run 44459420585, job 135342289188, source 8b3fcf71455ea2c1b4c1cf75f929ccc96f227d46:

https://github.com/davidpovarsky/cherri/actions/runs/44459420585/job/135342289188

The completed macOS job failed. Its actual terminal Go output contains 24 passing packages, five packages with no test files, and one failed package. The root compiler package passed in 515.843 seconds; tools/language-acceptance passed in 713.698 seconds. There are no named test-failure markers in the complete job log.

The sole failed package was an archived standalone Go overlay under the required-encoding evidence directory. It was incorrectly discovered by go test ./... and could not resolve EvidenceRecord, nativeEncodingTestID or validateNativeEncodingRecord. The separate 259e9fb repair preserves those exact bytes as a .go.txt artifact; root integrated it as 9663bec. This historical S3 job predates that fix and is not relabeled as a successful job.

The complete 1,303,408-byte official GitHub job log is retained outside Git with SHA256 eb5d7b4a49a88c334ae4855a839b06c7fe0877c2f78176fe25d938cf44e0e764. This archive retains exact job metadata, terminal package observations, selected diagnostic lines and the complete log digest. The mandatory WorkflowKit required-encoding step succeeded in this job; this observation alone makes no native runtime claim.

Scope: this one historical macOS job. S4, the private opt-in corpus, simulator execution, every optional test, downstream skipped steps and overall release acceptance remain outside this evidence.
