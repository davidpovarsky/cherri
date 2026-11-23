# Corrected immutable compiler: genuine Linux ARM64 pair

Compiler source S: `57b6d919757e1d0262993c6f11c6dc1cd17d6177`.
The implementation `5411d526` and its separate reviewed provenance metadata
were published to the isolated `audit/cherri-p10r15-f01-compiler-20261123`
branch. The normal push created that previously absent ref; authenticated
remote verification returned the exact source commit.

One genuine ordinary shallow clone with canonical GitHub origin and no
alternate object store produced both executables. Its working source stayed
clean. Go 1.24.13 built Linux ARM64 with CGO disabled, buildvcs=true,
trimpath, -p=1 and -ldflags=-s -w. The original clone/build streams and
go-version metadata are retained here; no compiler or test was replayed
during intake.

| Binary | Original bytes | SHA-256 |
| --- | ---: | --- |
| cherri | 16,777,400 | 5a1c0922925a8f3fb001052c7077f3a1df242c4cb8f04609d06f432d84683f5a |
| cherri-migrate | 5,505,208 | 348c1acf59f1eebb8f93c380d0e5ed442bcf520e2562452733fa6a56248da0bd |

Both original binaries passed the existing production package identity
validator: expected main package, Linux/ARM64, exact S and
vcs.modified=false. The compiler build took 55.764452 seconds and the
migrator 1.320500 seconds. The original clone took 16.856272 seconds.
All commands exited zero without timeout.

The complete original executables remain outside Git at
`/mnt/development/projects/evidence/cherri-p10r15-f01-arm64-pair-20261123`.
They are not copied into this metadata packet. Original R7 source,
executables and receipts remain unchanged.

This evidence establishes executable identity and reproducible source
selection. It does not establish Apple behavior, private S4, new raw505,
signing, installed-consumer acceptance, main promotion or full179.
Those producers retain separate source-bound records.
