# U04 public parser route: explicit P10R15 contract clarification

The active execution mandate, section E3, requires preserving existing Legacy public behavior. Section F1 forbids silent Legacy fallback for V2, and living-plan D005 preserves the original frontend. These instructions govern this execution. The old U04 expectation mixed the singular new-language path with migration-only Legacy wording, leaving an avoidable conflict with the supported explicit Legacy route.

Previous contract SHA256: 3c427d654759f2b09f61f288778dba3a7c53e195e04af9b03677d9c9ba8a5a0a. Previous U04 expectation: New source path is singular; legacy reference importer is build-time/migration-only.

Revised contract SHA256: 9fdfa8910b09824dc61395d8ddf4aa8fe8914619af0fdde59c63b67ab10b895e. Revised U04 expectation: Default and explicit V2 source paths use the V2 frontend without automatic Legacy fallback; explicit Legacy selection remains supported; invalid or conflicting modes fail closed.

Only U04's expected text changes. Its ID, requirement title, integration tier, planned status, all other 93 entries, and the 52 repair/33 recovery contracts remain unchanged. Historical results and receipts retain their original contract and status; they are not reinterpreted as passing. Current U04 evidence is produced only by actual selected CLI tests from the new source.

The route matrix runs both filename and compile-subcommand entry points. It verifies default and explicit V2, rejects Legacy-only source in those modes without output, preserves both explicit Legacy selectors, rejects invalid/conflicting modes, and decodes the emitted Text producer/Show Result consumer, requiring the Show input to reference the exact Text UUID. A wrong-reference corruption must fail that same oracle. The existing check-command and migration-guidance negatives also execute. Shared declaration readers, serializers and native backend remain supported; this contract does not prohibit those shared components.

No compiler implementation or emitted production behavior changes in this contract clarification. Tests and the formal producer must run successfully before a current-source U04 PASS is reported. The historical 179 result on ae7f877 remains 44 PASS / 1 FAIL / 134 skipped.
