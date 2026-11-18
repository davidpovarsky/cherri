# P10R15 parser acceptance: P01–P05

Prepared 2026-11-18 from verified integration base
`ab9a264ba370890397d74042e00c11a3b4c84ffd`.

This batch executes the five existing parser contracts through the production
lexer, parser, formatter, and language service. Its scope is unit and service
behavior. Swift UI, Cocoa integration, Apple Shortcuts runtime, linked repair
requirements, and release closure require their own evidence.

The immutable contract remains
`tests/language-v2/acceptance-contract.json`. None of its wording, tiers,
expected outcomes, or hash constants was changed.

## Contract coverage

| ID | Required tiers | Production proof |
| --- | --- | --- |
| P01 | unit | Exact token types and decoded values for plain, raw, multiline and raw multiline text; ordinary JSON/braces stay literal; f-string parts preserve escaped braces, Unicode and the expected arithmetic AST. Lost-brace and raw-escape mutations are rejected. |
| P02 | unit + service | Unterminated string, nested block comment, partial named call and malformed f-string execute in bounded child processes. Tests assert the exact diagnostic messages, byte spans and counts; intact surrounding bindings; relevant completion names and named insertion text; omission of an already-used label; service diagnostic propagation. Missing and shifted diagnostics are rejected. |
| P03 | unit | Explicit expected AST trees cover arithmetic precedence, associativity, unary/logical operations, parentheses, calls, member access and indexing. Comparison chains are rejected. Legal grouped comparisons remain legal, and formatting/reparsing preserves each tree. Operand-order mutations are rejected. |
| P04 | unit | Bare return followed by LF, CRLF, line comments or multiline comments remains bare; the following call remains a separate statement. Semicolon and closing-brace boundaries work. Same-line calls remain return values. Removing the newline causally changes the return boundary. |
| P05 | unit + service | Exact Hebrew, Hebrew with combining marks, decomposed Latin, mixed digit and supplementary-plane identifier names agree across tokens, AST bindings and completion insertion. Byte/UTF-16 spans are checked. Identifier controls U+202E/U+2066/U+200D/U+FEFF and a leading combining mark are diagnosed. |

These are five obligations and seven tier records, not seven independent tests.
The named producers are in `p15_parser_acceptance_test.go`:

- `TestP15P01LiteralAndInterpolationValues`
- `TestP15P02BoundedRecoveryAndCompletion`
- `TestP15P03PrecedenceAndComparisonChains`
- `TestP15P04ReturnTerminators`
- `TestP15P05UnicodeIdentifierTokenizerCompletion`

P02 starts the current test binary with only
`TestP15ParserRecoveryChild` selected, a four-second Go test timeout and a
six-second parent deadline. The no-hang assertion exercises actual parsing and
service completion; it does not substitute a simplified parser.

## Defects reproduced before correction

The initial bounded batch on the integration base failed with real production
behavior (SentinelX receipt `job_e8be3404acd1`):

1. An unterminated nested block comment produced no syntax diagnostic.
2. An unescaped closing brace inside an f-string returned the same lexical
   error repeatedly without advancing. The production parser hung; its child
   process was stopped by the four-second test timeout.
3. Chained relational/equality comparisons were accepted without diagnostics.
4. A bare return consumed the call on the next line, including CRLF and
   comment-separated cases.
5. Combining marks split otherwise valid identifiers into error tokens.

After adding the formatter round-trip check, a second causal failure
(`job_5661a5e9600d`) showed that formatting changed `(1 + 2) * 3` into
`1 + 2 * 3` and `10 - (3 - 2)` into `10 - 3 - 2`. It also removed
the grouping that keeps `(1 < 2) == true` legal.

The fixes are limited to the existing syntax and service implementation:

- Emit a pending lexical error for an unterminated block comment, retaining
  its actual source span and comment trivia.
- Advance past an invalid f-string brace, report that brace's exact span, and
  consume an immediately following f-string terminator during recovery.
- Retain parse-time grouping information and reject ungrouped comparison
  chains without adding a new AST node.
- Stop optional return values at an actual leading newline.
- Share the identifier start/continuation rules between lexer and service.
  Combining marks continue identifiers but cannot start them; identifier
  spelling is preserved without Unicode normalization.
- Preserve binary, unary and postfix target grouping when formatting.

## Documented expression tree

From lower to higher precedence:

| Operators | Association |
| --- | --- |
| `||` | left |
| `&&` | left |
| `==`, `!=` | comparison chaining rejected |
| `<`, `<=`, `>`, `>=`, `contains`, `beginsWith`, `endsWith` | comparison chaining rejected |
| `+`, `-` | left |
| `*`, `/`, `%` | left |
| prefix `!`, `+`, `-` | prefix |
| call, indexing, member access | postfix |

For example, `1 + 2 * 3 == 7 && false || true` has the tree
`(|| (&& (== (+ 1 (* 2 3)) 7) false) true)`.
Use `a < b && b < c` for two comparisons. An explicitly grouped comparison
such as `(a < b) == true` remains a distinct, legal expression.

The formatter preserves AST shape, including right children at the same
precedence. It does not assume associativity merely because two arithmetic
expressions might have equal mathematical values.

## Executed verification

The corrected production batch passed in receipt `job_21e6848c87fe`:

```sh
go test -count=1 -timeout 90s . -run '^TestP15P0[1-5]'
```

The complete affected syntax and service packages passed in receipt
`job_44b6570f8a6d`:

```sh
go test -count=1 -timeout 90s ./internal/language/syntax ./internal/language/service
```

The bounded acceptance producer and its negative controls passed in receipt
`job_03ef2f1f902d`:

```sh
go test -count=1 -timeout 180s ./tools/language-acceptance -run '^TestParserAcceptance'
```

The runner now calls the exact named production tests and requires an explicit
Go JSON pass event for every selected name. Empty selections and nonexistent
test names fail closed. Receipts contain the actual command arguments and
producer IDs. Additional evidence controls remove P02's service record and
substitute a wrong producer identity; both mutations must fail validation.

`TestParserAcceptanceProductionEvidence` emits a
`P15_PARSER_RECEIPT:` JSON log with the observed source SHA, working-tree
cleanliness, five-contract report and seven executed tier records. The
179-obligation audit remains complete and gives no inherited PASS to any other
obligation. The main integration owner refreshes the combined report at the
final integrated SHA.

## U04 remains unresolved

U04 says the normal public binary must not invoke the legacy script parser,
and its expected outcome also says that the legacy reference importer is
build-time/migration-only. The current living master plan's decision D005
requires preserving the original frontend and its semantics. The current
mandate likewise requires preserving working behavior, including the explicit
public `--legacy` route and dual-mode import/compile fidelity.

These statements do not establish one unambiguous acceptance contract.
CL03's production CLI test proves that the default V2 route rejects Legacy
syntax with migration guidance. That scoped proof does not prove U04's stronger
migration-only wording.

U04 therefore stays **UNVERIFIED**. This batch neither rewrites the historical
contract nor removes the explicit Legacy route. Its audit blocker names the
D005 conflict. Existing CL01, CL02 and CL03 producers and their evidence remain
unchanged.
