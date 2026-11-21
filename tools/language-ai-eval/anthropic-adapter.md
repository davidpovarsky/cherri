# Anthropic adapter for the sealed language evaluation

This transport implements the existing `cherri-language-ai-eval/v1` subprocess protocol. It does not score generated source or prove AI01. The independent bundle, runner and semantic validator own the fixed six paired tasks and all acceptance decisions.

## Authorized protocol

The adapter requires an anchored bundle through `CHERRI_AI_EVAL_BUNDLE` and `CHERRI_AI_EVAL_ALLOW_GENERATION=1` before even the token-count request. The shared bundle validator verifies the exact approved public request and code hashes. Each task/arm claims an exclusive attempt directory; a completed, failed, refused, truncated or interrupted claim cannot be retried. The pinned manifest fixes:

- Model `claude-sonnet-5-5`, API version `2026-10-21`, HTTPS origin `api.anthropic.com`.
- One token-count preflight (15-second transport deadline), followed by at most one Messages call (90-second transport deadline); the runner applies its unchanged 120-second process limit.
- Thinking disabled, streaming disabled, a fixed system instruction and exactly one stateless user message.
- Identical input/output caps for both arms. No tools, MCP, cache requests, threads, fallbacks, redirects, workspace override or retries.

The provider message contains only the approved arm, task kind, public prompt, starter and guide. Seed, suite metadata, private oracle and validator parameters are excluded. The token-count endpoint returns an estimate; the adapter records it separately from actual reported usage and never represents preflight as a provider-enforced exact input limit. Actual input accounting sums reported input tokens and both cache input components, without double-counting the cache-creation breakdown. Over-budget actual usage is rejected and retained. Unrequested cache/tool/thinking/iteration activity also fails closed.

The adapter reads `ANTHROPIC_API_KEY` and trims only outer whitespace in memory. It never writes credentials or headers to configuration, logs, receipts or stdout. Raw bodies containing a reflected credential are withheld; their hash, byte count and the withholding reason remain in the receipt. Provider exception text is not serialized. No actual provider or counting calls occur in the offline controls.

## Original evidence

For each safely claimed attempt, the adapter retains these host-local files:

| File | Meaning |
|---|---|
| `request.json` | Canonical approved subprocess request |
| `adapter-request.original.json` | Exact stdin bytes |
| `count.request.original.json`, `count.response.original.body` | Original token-count bodies |
| `generation.request.original.json`, `generation.response.original.body` | Original generation bodies, when attempted |
| `generated-source.original.cherri` | Exact concatenated text blocks, with no repair, trimming or inserted separator |
| `reply.json` | Canonical protocol success or error reply |
| `provider-receipt.json` | Source identities, times, HTTP status/IDs, raw hashes and usage accounting |

Receipt schema is `cherri-anthropic-provider/v1`. It includes `request_id`, `task_id`, `arm`, `implementation_sha`, `adapter_sha256`, expected/actual model, API version, UTC start/finish, status/error type, fixed caps and timeouts. `calls.count` and `calls.generation` record attempted flags, stage times, request hash, HTTP status, actual provider request ID, completion and safe error type. A valid server HTTP Date is retained as `response_date`; it is a server receipt, not a cryptographic timestamp. No other raw headers are collected. `originals` binds each retained basename to its SHA256, byte count and completeness. The receipt also preserves the original usage object and normalized input, output, cache and total-input counts.

`GENERATED` requires the exact model, a normal end-turn, plain text, genuine provider IDs and valid actual usage within caps. Refusals and token-limit truncations retain their original content/usage with `REFUSED` or `TRUNCATED` outcomes. Other errors remain `FAILED` or `CANCELLED`. A forced termination or receipt-write failure may leave a STARTED receipt and claimed directory. That is incomplete evidence, never a success or permission to retry. HTTP deadlines and bounded reads are enforced by the transport; the outer process timeout is the final guard if the operating system blocks an operation.

## Offline validation

Run only mock transport controls with:

```sh
python3 -B -m unittest discover -s tools/language-ai-eval -p test_anthropic_adapter.py -v
```

The tests prohibit unmocked HTTPS and exercise the approved wire shape, original-source preservation, count and actual caps, all cache components, refusal/truncation, invalid usage/model/IDs, no retries, anchored input/code identity, safe failures, secret reflection, fixed HTTPS destination, and bounded input/response reads. Mock responses provide no model quality, real provider usage, compiler or Apple evidence.

Official API contracts used for this implementation: [API overview](https://platform.claude.com/docs/en/api/overview), [Messages](https://platform.claude.com/docs/en/api/http/beta/messages/create), [token counting endpoint](https://platform.claude.com/docs/en/api/http/beta/messages/count_tokens), and [token counting interpretation](https://platform.claude.com/docs/en/build-with-claude/token-counting). Actual execution remains subject to the reviewed, anchored evaluation bundle.
