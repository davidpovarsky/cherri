# Historical S3 Linux full-Go job

Run 44459420585, job 135342289479, source 8b3fcf71455ea2c1b4c1cf75f929ccc96f227d46:

https://github.com/davidpovarsky/cherri/actions/runs/44459420585/job/135342289479

The completed Linux job failed. Its actual terminal Go output contains 24 passing packages, five packages with no test files, and one failed package. The root compiler package passed in 459.615 seconds; tools/language-acceptance passed in 653.134 seconds. There are no named test-failure markers in this job log.

The sole failed package was an archived standalone Go overlay under the required-encoding evidence directory. It was incorrectly discovered by go test ./... and could not resolve EvidenceRecord, nativeEncodingTestID or validateNativeEncodingRecord. Native's separate 259e9fb repair preserves those exact bytes as a .go.txt artifact. This historical job predates that fix, and is not relabeled as a successful job.

The completed job logs were retrieved from the official GitHub job-logs API because gh run view deferred logs while the macOS sibling job was still running. The complete 1,298,843-byte log is retained outside Git with SHA256 162c740af63c63948fcae3cd6c56d3f33f3acbd1c8668d508305bd096055edd1. This archive retains exact job metadata, package/failure observations, selected diagnostic lines and the complete log's digest.

Scope: this one historical Linux job. No claim is made about S4, macOS completion, the private opt-in corpus, simulator execution, every optional test, downstream skipped steps, or overall release acceptance.
