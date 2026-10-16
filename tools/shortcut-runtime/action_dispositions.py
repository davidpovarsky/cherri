#!/usr/bin/env python3
"""tools/shortcut-runtime/action_dispositions.py

Auditable per-action, per-key parameter disposition registry and classification engine.
Replaces simplistic blanket suppression sets with exact, evidence-backed dispositions:
  - EXPOSED_PARAMETER: Declared as an explicit parameter in Cherri signature variant(s).
  - EMITTED_IMPLICIT_CONSTANT: Handled as an emitted constant or runtime execution flag.
  - IMPLEMENTED_BY_SHARED_BUILDER: Handled by shared compiler builder (AppIntentDescriptor, filter template, etc.).
  - INTENTIONALLY_UNREPRESENTABLE_UI_METADATA: Ephemeral editor UI layout state (e.g., ShowHeaders inspector).
  - NEEDS_RUNTIME_EVIDENCE: Device-specific or opaque picker format requiring canonical Apple export evidence.
  - OPEN_USER_PARAMETER: Genuinely open user parameter needing Cherri parameter modeling.
  - UNSUPPORTED_TARGET: Target platform/OS unsupported or deprecated.
"""

from typing import Dict, Any, List, Optional, Tuple


# Known implicit keys and their architectural rationale
SHARED_BUILDER_KEYS = {
    "IntentAppDefinition": {
        "disposition": "IMPLEMENTED_BY_SHARED_BUILDER",
        "codePath": "action.go:appIntentDescriptor()",
        "rationale": "AppIntent transport descriptor identifying target application bundle; emitted automatically from actionDefinition.appIntent",
        "evidenceRef": "WorkflowKit WFIntentAppDefinition / AppIntentDescriptor"
    },
    "WFContentItemFilter": {
        "disposition": "IMPLEMENTED_BY_SHARED_BUILDER",
        "codePath": "actions_std.go:buildContentItemFilter() / action.go:makeActionParams",
        "rationale": "Content item predicate table template dictionary; emitted by shared filter builder",
        "evidenceRef": "WorkflowKit WFTableTemplateContentItemFilterParameter"
    },
    "WFCompoundType": {
        "disposition": "IMPLEMENTED_BY_SHARED_BUILDER",
        "codePath": "actions_std.go:buildContentItemFilter() / action.go:makeActionParams",
        "rationale": "Compound predicate conjunction type ('0' for All/AND, '1' for Any/OR); emitted by filter builder",
        "evidenceRef": "WorkflowKit WFCompoundType enum"
    },
    "WFContentItemLimitEnabled": {
        "disposition": "IMPLEMENTED_BY_SHARED_BUILDER",
        "codePath": "action.go:makeActionParams",
        "rationale": "Boolean flag indicating whether item limit number is applied; emitted when limit is provided",
        "evidenceRef": "WorkflowKit WFContentItemLimitEnabled"
    },
    "WFAlertTime": {
        "disposition": "IMPLEMENTED_BY_SHARED_BUILDER",
        "codePath": "actions/calendar.cherri:addReminder",
        "rationale": "Alert mode specifier emitted in conjunction with alert custom date/time",
        "evidenceRef": "WorkflowKit WFAlertTime mode"
    }
}

IMPLICIT_CONSTANT_KEYS = {
    "ShowWhenRun": {
        "disposition": "EMITTED_IMPLICIT_CONSTANT",
        "codePath": "actions_std.go (ShowWhenRun parameter)",
        "rationale": "Runtime UI execution toggle (Show when run) managed implicitly by Shortcuts runner",
        "evidenceRef": "WorkflowKit ShowWhenRun parameter"
    },
    "WFTakeScreenshotIgnoreContextualAssistanceLayers": {
        "disposition": "EMITTED_IMPLICIT_CONSTANT",
        "codePath": "actions/mac.cherri / actions_std.go",
        "rationale": "System screenshot layer exclusion flag defaulted to true",
        "evidenceRef": "ActionKit WFTakeScreenshotIgnoreContextualAssistanceLayers"
    }
}

UI_METADATA_KEYS = {
    "ShowHeaders": {
        "disposition": "INTENTIONALLY_UNREPRESENTABLE_UI_METADATA",
        "codePath": "N/A (inspector layout only)",
        "rationale": "Transient Shortcuts inspector layout state; actual headers serialized via WFHTTPHeaders",
        "evidenceRef": "ActionKit ShowHeaders inspector state"
    }
}

SPECIAL_EVIDENCE_KEYS = {
    ("is.workflow.actions.getupcomingevents", "WFGetUpcomingItemCalendar"): {
        "disposition": "NEEDS_RUNTIME_EVIDENCE",
        "codePath": "actions/calendar.cherri:getUpcomingEvents",
        "rationale": "Device-specific calendar picker token requiring canonical Apple-exported dynamic calendar fixture; standard empty/count/specifier signatures supported without picker",
        "evidenceRef": "macOS 27 parameter-encodings.json: WFGetUpcomingItemCalendar"
    },
    ("is.workflow.actions.deletephotos", "ShowWhenRun"): {
        "disposition": "EMITTED_IMPLICIT_CONSTANT",
        "codePath": "WorkflowKit runtime runner / com.apple.Photos LinkActionProvider",
        "rationale": "In macOS 26, is.workflow.actions.deletephotos was a WorkflowKit built-in with ShowWhenRun execution prompt. In macOS 27, photo deletion migrated to LinkActionProvider com.apple.Photos.DeleteAssetsAction with entities. Reconciled across OS versions as an implicit execution toggle.",
        "evidenceRef": "macOS 26 / macOS 27 historical migration evidence"
    }
}


def classify_parameter_disposition(
    action_id: str,
    key: str,
    cherri_declared_keys: set,
    cherri_emitted_keys: set,
    cherri_static_keys: set,
    is_filter_action: bool = False
) -> Dict[str, Any]:
    """Evaluates the auditable disposition of an Apple parameter key against Cherri modeling."""
    # 1. Declared in Cherri parameters
    if key in cherri_declared_keys:
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": "EXPOSED_PARAMETER",
            "rationale": "Declared parameter in Cherri callable variant",
            "codePath": "actions/*.cherri or actions_std.go",
            "evidenceRef": "Cherri actions catalog"
        }

    # 2. Emitted statically or via appendParamsFunc
    if key in cherri_emitted_keys or key in cherri_static_keys:
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": "EMITTED_IMPLICIT_CONSTANT",
            "rationale": "Emitted by action definition as static parameter or appendParamsFunc constant",
            "codePath": "actions/*.cherri or actions_std.go",
            "evidenceRef": "Cherri action definition"
        }

    # 3. Special per-action evidence requirements
    special = SPECIAL_EVIDENCE_KEYS.get((action_id, key))
    if special:
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": special["disposition"],
            "rationale": special["rationale"],
            "codePath": special["codePath"],
            "evidenceRef": special["evidenceRef"]
        }

    # 4. Filter builder keys
    if is_filter_action and key in ("WFContentItemFilter", "WFCompoundType", "WFContentItemLimitEnabled"):
        builder_info = SHARED_BUILDER_KEYS[key]
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": builder_info["disposition"],
            "rationale": builder_info["rationale"],
            "codePath": builder_info["codePath"],
            "evidenceRef": builder_info["evidenceRef"]
        }

    # 5. Shared builders (like IntentAppDefinition)
    if key in SHARED_BUILDER_KEYS:
        builder_info = SHARED_BUILDER_KEYS[key]
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": builder_info["disposition"],
            "rationale": builder_info["rationale"],
            "codePath": builder_info["codePath"],
            "evidenceRef": builder_info["evidenceRef"]
        }

    # 6. Implicit constants (like ShowWhenRun)
    if key in IMPLICIT_CONSTANT_KEYS:
        const_info = IMPLICIT_CONSTANT_KEYS[key]
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": const_info["disposition"],
            "rationale": const_info["rationale"],
            "codePath": const_info["codePath"],
            "evidenceRef": const_info["evidenceRef"]
        }

    # 7. Ephemeral UI metadata
    if key in UI_METADATA_KEYS:
        meta_info = UI_METADATA_KEYS[key]
        return {
            "identifier": action_id,
            "parameterKey": key,
            "disposition": meta_info["disposition"],
            "rationale": meta_info["rationale"],
            "codePath": meta_info["codePath"],
            "evidenceRef": meta_info["evidenceRef"]
        }

    # 8. Unmodeled user parameter
    return {
        "identifier": action_id,
        "parameterKey": key,
        "disposition": "OPEN_USER_PARAMETER",
        "rationale": f"Unmodeled parameter key '{key}' requiring Cherri parameter modeling or evidence",
        "codePath": "unimplemented",
        "evidenceRef": f"macOS 27 runtime extraction for {action_id}"
    }
