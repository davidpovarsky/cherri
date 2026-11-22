# Actions App descriptor negatives: source-backed scope

## Contract and provenance

This change addresses the observed descriptor-loss part of Asana
`1220373052713862` and the descriptor clause of `1219603547340941`. It does
not complete either task. The external builder contract belongs to
`1220372963578685`.

The checked-in test observation is the public metadata row for
`com.sindresorhus.Actions.IsLocationServicesEnabledIntent` from Actions
4.0.1. Its descriptor contains Team `YG56YK5RN5`, bundle
`com.sindresorhus.Actions`, intent `IsLocationServicesEnabledIntent`, name
`Actions`, and the native Boolean `ActionRequiresAppInstallation=true`.
The extracted intent has no parameters. The workflow surrounding it is
synthetic and contains no user Shortcut data.

Original extraction: GitHub Actions run
<https://github.com/davidpovarsky/cherri/actions/runs/39821499226>.
The retained public `actions-app-intents.json` has 99 entries and SHA-256
`d4390ac5bb39b3f025f0ad55ddad052678ae9585c586168c2a2cfd07e2c1c062`.
The retained `actions-version.txt` contains `4.0.1` and has SHA-256
`cd6321af37482f49efc326524a9fcd7fc86902a48da8d4bbd3dc718d290c3fea`.
These are observations of that extraction, not current installed-app claims.

## Implemented checks; execution pending

The test uses the existing native filesystem import pipeline and both actual
frontends: XML/binary × hybrid/lossless × Legacy/V2, eight paths. It requires
the complete decoded native tree to match an independently authored input,
including all three actions, descriptor values and native Boolean kind, the
producer UUID and consumer reference, and the unchanged neighboring comment.
Hybrid import must retain its truthful `THIRD_PARTY_NO_CANONICAL_DEF`
classification. No production normalization builds the expected descriptor.

Fourteen causal corruptions start from an actual successful imported output:
empty/missing descriptor; wrong Team, Bundle, Intent or Name; false/string/
missing installation flag; changed outer identifier; wrong producer or
consumer UUID; changed neighbor; and an extra action. Each must be rejected
by the full-tree oracle, while the original remains unchanged. This proves
the test detects native transport damage; it does not require the compiler to
reject arbitrary opaque native input.

The source was prepared on integration commit
`5c772e4bd5719530eeb1a03a1413af6739f61a26`. No Go execution, signing, runtime
execution or named Actions App support is claimed by this source change.

## Exact remaining external-consumer dependency

The original task locates `actions_app_bridge.py`, `README.md`, the versioned
catalog, and `actions-app-4.0.1-build-kit.zip` under the historical Linux
`cherri-remote/pure-upstream/apple-actions-pack/external-build` workspace.
It pins ShortcutKit to `ee52c32937053268e4a035eba4b372fe77ec6411` and Pure to
`bcc0095754e2cc0baaeb4d93dae172b8153b4716`. The bridge's actual opt-in,
dependency-presence and version interface has not been recovered.

The connected Windows host retains the public extraction but not those
bridge/build-kit files. The known Apple Actions Pack branch at
`141b9cc96cf1df3f9d1593548dc49ae8a46a256a` has no `external-build` tree; the
original extraction run currently exposes zero artifacts; the task has no
attachments; exact Library title searches found no bridge or build-kit.
These are bounded recovery results, not a claim the files never existed.

Disabled/missing-external/version mismatch consumer controls remain blocked
on that interface. No replacement manifest keys, bridge engine, availability
check or unsupported descriptor was invented. The separate Pure/Pack 0.1.1
surface and installed Apple runtime remain unchanged and unverified here.
