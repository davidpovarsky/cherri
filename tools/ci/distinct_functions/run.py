"""One focused repair qualification on the exact current joined source.
Original failed streams remain immutable; this is not full-suite or native credit.
The existing selected-producer scheduling policy remains sequential.
"""
from pathlib import Path
import base64
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import stat
import sys
import traceback
import time

sys.dont_write_bytecode = True
SOURCE = "57cf869d555f14fa79576d48b74ddcff656d3f08"
TREE = "579b1f078d07a937a5a1831ec84d0227b5184966"
PARENTS = tuple(["2403037ea09ccfdd65c90a6ee1d158b5fd2760c0","0886f66a88e387c2c44e5db761184540735383ee"])
BRANCH = "probe/cherri-p10r15-oracle-focused-57cf86-20261127"
HELPER_SHA = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
PREFIX = "tools/ci/distinct_functions/"
CONTROLLER_FILES = [".github/workflows/p10r15-distinct-functions.yml", PREFIX+"run.py", PREFIX+"owned_source_process.py"]
PACKAGE = "github.com/electrikmilk/cherri"
REGRESSION_COMMAND = tuple(["go","test","-count=1","-json","-timeout=3m","-parallel=2",".","./internal/language/backend","./internal/shortcutcompare","-run","^(TestFunctionBodyMutation_DetectedByNormalizedComparator|TestFunctionNoContentEpilogueNormalization|TestP15ConditionalInputBindingIdentityControls|TestP15FullSuiteSubprocessParallelPolicy|TestP15LegacyCompoundWrittenConditions|TestP15LegacyConditionalOperandSnapshots|TestP15LegacyConditionalSnapshotIdentityControls|TestP15MeasureInputBoundaryOracleControls|TestP15MeasurePreviousMatchesAuthoredBaseline|TestP15MeasurePreviousOracleRejectsCausalFaults|TestP15MeasureVoidGraphRejectsCausalMutations|TestP15MeasureVoidLoopBoundaryGraphs|TestP15MeasureVoidNativeGraphs|TestP15MeasureVoidPreservesPrevious|TestP15ScopedValueTransportComposition|TestP15ScopedValueTransportGraphFaults|TestP15ScopedValueTransportLexicalStorage|TestP15_LegacyMeasureRejectsUnprovenForms|TestScopedMeasureInputCaptureRestoreOrderAndFailures|TestScopedMeasureRejectsIncompleteInputBoundaryBeforeEffects)$"])
EXPECTED_PACKAGES = tuple(["github.com/electrikmilk/cherri","github.com/electrikmilk/cherri/internal/language/backend","github.com/electrikmilk/cherri/internal/shortcutcompare"])
NO_TEST_PACKAGES = frozenset()
REQUIRED_PARENTS = tuple(tuple(x) for x in [["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms"],["github.com/electrikmilk/cherri","TestP15ConditionalInputBindingIdentityControls"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalOperandSnapshots"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureRejectsIncompleteInputBoundaryBeforeEffects"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization"]])
REQUIRED_SCENARIO_CHILDREN = tuple(tuple(x) for x in [["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_EARLY_YIELD"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_EARLY_YIELD#01"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_LEGACY_CONTROL_VALUE"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_RAW_NATIVE"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_VALUE_REQUIRED"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_VALUE_REQUIRED#01"],["github.com/electrikmilk/cherri","TestP15_LegacyMeasureRejectsUnprovenForms/E_BLOCK_VALUE_REQUIRED#02"],["github.com/electrikmilk/cherri","TestP15ConditionalInputBindingIdentityControls/consistent_private_input_hygiene"],["github.com/electrikmilk/cherri","TestP15ConditionalInputBindingIdentityControls/literal_input_spelling_remains_exact"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/different_marker"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/excessive_limit"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/missing_limit"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/missing_test"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/nested_child_unchanged"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/nested_child"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/no_opt_in"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/ordinary"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/padded_marker"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/selected_all"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/selected_producer_unchanged"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/selected_producer"],["github.com/electrikmilk/cherri","TestP15FullSuiteSubprocessParallelPolicy/serial_limit"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_skip/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_skip/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_skip/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_skip/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_take/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_take/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_take/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_action_take/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_effect/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_effect/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_effect/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_effect/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_false_first/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_false_first/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_false_second/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_false_second/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three_actions/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three_actions/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three_actions/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three_actions/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_three/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_true/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/and_true/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/nested_action/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/nested_action/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/nested_action/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/nested_action/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_skip/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_skip/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_skip/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_skip/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_take/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_take/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_take/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_action_take/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_effect/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_effect/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_effect/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_effect/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_false/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_false/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_first_true/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_first_true/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_second_true/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_second_true/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three_actions/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three_actions/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three_actions/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three_actions/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/or_three/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/repeat_action/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/repeat_action/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/repeat_action/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyCompoundWrittenConditions/repeat_action/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalOperandSnapshots/v17/debug"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalOperandSnapshots/v17/release"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalOperandSnapshots/v27/debug"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalOperandSnapshots/v27/release"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/authored_variable_reference"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/consistent_private_hygiene"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/drop_timer"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/duplicate_timer"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/foreign_timer_reference"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/literal_timer_spelling_remains_exact"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/timer_after_read"],["github.com/electrikmilk/cherri","TestP15LegacyConditionalSnapshotIdentityControls/wrong_clock"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/capture_explicit_input"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/capture_null_input"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/drop_capture"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/drop_restore"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/duplicate_helper_uuid"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/external_restore_consumer"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/extra_capture_parameter"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/nested_slot_collision"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/orphan_capture"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/release_instrumentation"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/restore_coercion"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/restore_foreign_slot"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_false/restore_missing_uuid"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/capture_explicit_input"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/capture_null_input"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/drop_capture"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/drop_restore"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/duplicate_helper_uuid"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/external_restore_consumer"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/extra_capture_parameter"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/nested_slot_collision"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/orphan_capture"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/release_instrumentation"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/restore_coercion"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/restore_foreign_slot"],["github.com/electrikmilk/cherri","TestP15MeasureInputBoundaryOracleControls/legacy_true/restore_missing_uuid"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/body_owned_previous/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/date_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/debug_previous/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/empty_text/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/entry_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_date_input/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/nested_log/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/no_content_date/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousMatchesAuthoredBaseline/zero_number/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/coerced_restore"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/explicit_capture"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/removed_consumer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=false/wrong_restore_slot"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/capture_after_timer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/coerced_restore"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/explicit_capture"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/removed_consumer"],["github.com/electrikmilk/cherri","TestP15MeasurePreviousOracleRejectsCausalFaults/legacy=true/wrong_restore_slot"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/boundary_input"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/boundary_output"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/changed_delay"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/date_extra_reference"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/duplicate_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/early_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/early_stop"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/late_start"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/lost_authored_nothing"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/lost_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/lost_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/reverse_effects"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/stop_extra_parameter"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/wrong_elapsed_name"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/wrong_elapsed_origin"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/wrong_start_clock"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/debug/wrong_stop_origin"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/boundary_input"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/boundary_output"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/changed_delay"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/duplicate_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/early_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/lost_authored_nothing"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/lost_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/lost_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_false/release/reverse_effects"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/boundary_input"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/boundary_output"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/changed_delay"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/date_extra_reference"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/duplicate_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/early_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/early_stop"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/late_start"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/lost_authored_nothing"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/lost_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/lost_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/reverse_effects"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/stop_extra_parameter"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/wrong_elapsed_name"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/wrong_elapsed_origin"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/wrong_start_clock"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/debug/wrong_stop_origin"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/boundary_input"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/boundary_output"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/changed_delay"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/duplicate_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/early_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/lost_authored_nothing"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/lost_boundary"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/lost_effect"],["github.com/electrikmilk/cherri","TestP15MeasureVoidGraphRejectsCausalMutations/legacy_true/release/reverse_effects"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_false/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_false/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_false/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_false/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_true/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_true/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_true/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n0/following_true/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_false/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_false/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_false/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_false/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_true/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_true/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_true/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n1/following_true/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_false/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_false/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_false/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_false/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_true/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_true/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_true/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidLoopBoundaryGraphs/n3/following_true/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/argument_effect/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/argument_effect/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/argument_effect/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/argument_effect/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/assignment_tail/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/assignment_tail/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/assignment_tail/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/assignment_tail/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/binding_tail/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/binding_tail/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/binding_tail/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/binding_tail/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/body_nothing/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/body_nothing/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/body_nothing/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/body_nothing/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/empty/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/empty/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/empty/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/empty/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/if_tail/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/if_tail/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/if_tail/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/if_tail/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_empty/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_empty/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_empty/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_empty/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_wait/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_wait/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_wait/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/nested_wait/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/repeat_tail/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/repeat_tail/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/repeat_tail/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/repeat_tail/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/stop_before_void_tail/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/stop_before_void_tail/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/stop_before_void_tail/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/stop_before_void_tail/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/two_effects/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/two_effects/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/two_effects/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/two_effects/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/value_then_void/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/value_then_void/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/value_then_void/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/value_then_void/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/wait/legacy_false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/wait/legacy_false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/wait/legacy_true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidNativeGraphs/wait/legacy_true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=false/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=false/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=false/release"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=true/debug"],["github.com/electrikmilk/cherri","TestP15MeasureVoidPreservesPrevious/nested=true/legacy=true/release"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_for_two/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_for_two/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_for_two/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_for_two/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_one_empty_list/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_one_empty_list/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_one_empty_list/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_one_empty_list/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_three_two_items/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_three_two_items/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_three_two_items/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_three_two_items/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_zero_none/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_zero_none/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_zero_none/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/alias_zero_none/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/deep_list/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/deep_list/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/deep_list/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/deep_list/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dictionary_array/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dictionary_array/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dictionary_array/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dictionary_array/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/distinct_logical_cells/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/distinct_logical_cells/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/distinct_logical_cells/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/distinct_logical_cells/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dynamic_loop_payload/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dynamic_loop_payload/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dynamic_loop_payload/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/dynamic_loop_payload/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_list/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_list/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_list/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_list/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_loop_payload/debug/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_loop_payload/debug/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_loop_payload/release/measure"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportComposition/nested_loop_payload/release/ordinary"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/drop_body_cell"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/duplicate_body_cell"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/duplicate_uuid"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/extra_nothing"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/output_wrong_value"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/restore_wrong_value"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/wrong_elapsed_log"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/debug/wrong_timer_start"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/drop_body_cell"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/duplicate_body_cell"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/duplicate_uuid"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/extra_nothing"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/output_wrong_value"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportGraphFaults/release/restore_wrong_value"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/debug"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/debug/source_write_changed"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/debug/storage_collision"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/debug/swap_results"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/debug/wrong_snapshot"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/release"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/release/source_write_changed"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/release/storage_collision"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/release/swap_results"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/captured_write_then_shadow/release/wrong_snapshot"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/debug"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/debug/source_write_changed"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/debug/storage_collision"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/debug/swap_results"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/debug/wrong_snapshot"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/release"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/release/source_write_changed"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/release/storage_collision"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/release/swap_results"],["github.com/electrikmilk/cherri","TestP15ScopedValueTransportLexicalStorage/repeated/release/wrong_snapshot"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/body"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/capture"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/debugStartTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/debugStopTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/value/restore"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/body"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/capture"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/debugStartTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/debugStopTimer"],["github.com/electrikmilk/cherri/internal/language/backend","TestScopedMeasureInputCaptureRestoreOrderAndFailures/void/restore"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/double_body_math_operator"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/fibonacci_body_condition_threshold"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionBodyMutation_DetectedByNormalizedComparator/greetDefault_default_value"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/body_mutation_is_preserved"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/branch_local_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/empty_parameter_maps"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_explicit_empty_output"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_explicit_input"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_list_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_null_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/exit_scalar_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/extra_action_metadata_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/main_frame_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/no_function_envelope_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nonterminal_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_explicit_input"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_null_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/nothing_scalar_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/omitted_parameters"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/only_one_terminal_pair_is_removed"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/unpaired_exit_is_observable"],["github.com/electrikmilk/cherri/internal/shortcutcompare","TestFunctionNoContentEpilogueNormalization/value_return_is_preserved"]])
REQUIRED_IDENTITIES = REQUIRED_PARENTS + REQUIRED_SCENARIO_CHILDREN
SOURCE_PINS = {
    "build_profile.go": "c2fcfa5f7a46eda8a16ce3fe7ea8aa6ced74191b38bdb8019aa9623d53aa60ae",
    "internal/language/analysis/scoped_blocks_test.go": "caae50e35bbe840daa8e81a5b6c04282b14976bf73708c87d77c794cfc5d5783",
    "internal/language/analysis/scoped_blocks_void_test.go": "223848d594d2dfd67f9b9aa5d354307022ad1025a125d8b48dc19ac6f4a7e1ad",
    "internal/language/backend/contracts.go": "fde3e1c72b29b8d8bbf706b3c856f468f6be64c5d091d87321189a5130283e33",
    "internal/language/backend/scoped_cache_previous_test.go": "f44b843e3326bf6c964816acd295ffe896b1370d12ac7c3b2f10599facddef32",
    "internal/language/backend/scoped_measure.go": "d3a4dc25954bd147beb85a906a78aa04157e3bafba60748d6f7f9c71d2bb713c",
    "internal/language/backend/scoped_measure_input_test.go": "0f0856fa0a7fe72c064dbd2175f6179a7531e762a4578de1dec35be3c3661fad",
    "internal/language/backend/scoped_measure_test.go": "f454a81a1a16d3643e984e2fb894e5c459fb3eb928a795fe37b6db2c05193624",
    "internal/language/backend/scoped_measure_void_test.go": "f59c4291599de39d85b2a666647da4a67f5e7c9625e2c70e22035dbd3b945524",
    "internal/language/lower/lower.go": "4a28c73289d811282aac7be7bdea5d06f50e51486d951ea685f4064fe77c9eef",
    "internal/language/lower/lower_test.go": "1890a8692fab2b7a8609e449635c982a0d0330331cd75f739439494080e93579",
    "internal/language/lower/main_output.go": "41721975525ff359d3880c06d796aff98663c1b91f42e050fcb8625f50350921",
    "internal/language/lower/scoped_blocks.go": "0a7286724bc23b55236e779c82a6abf273f53330ed8a7963324a95cc4f02c7db",
    "internal/language/lower/scoped_blocks_test.go": "05838d0ed4b08707d126491e3391c3c561b18bde19b76242a140a35ed69e2fa2",
    "internal/language/lower/scoped_value_identity_test.go": "d9a8408b81d63a0171cb6aabf7557ca1cd28f809c2e4423cf9a154723756679d",
    "internal/language/schema/scoped_loop_result_test.go": "124bd195e6711af50b15e10a6dbedbc8fc9c0cc5f6e20ba9a7f3bb5f3163f7e3",
    "internal/language/syntax/scoped_blocks_test.go": "6fe2f1d013f63b60e86e16415c8e0dcccf14889f10ac932e19b5cb23f2815a7c",
    "internal/shortcutcompare/functions_regression_test.go": "b195b626e5593932170fdd194bcd5e550b460bd39c921e1c5dd8ec7ce42256fd",
    "internal/shortcutcompare/no_content_epilogue_test.go": "b2ec7b9cfb2ea1f70f6f250001fa8b08bcb6b742c4710961b53cf9f94ccaab98",
    "internal/shortcutcompare/normalize.go": "360ca60c0363381f314952e0e7c84ad049c722dd3420694985709fd2677cafd4",
    "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift": "b49403d851b5c906d9eea3dca76cfca1c3cb9a217f779c76584afac945375c80",
    "ios/CherriUITests/CherriJoinedContractUITests.swift": "db1eea0fda4d6b382d6704ccf661bc88dc1693df151651ee05c58cb9b98da2a7",
    "language_backend_adapter.go": "a095d66cce83cdae7815092932ba1b875af60d6b5f9893890f5233eb18a99d5f",
    "p10_ssot_propagation_test.go": "61fc8628dabd8d88f9cc4b1036b7c6532526b363a2ee73950e42e0ab0a58a77e",
    "p10r9_hybrid_test.go": "459966d5253eee8e7d91c18925cdf1de83e115d14040e2eb2ddc429d41fc9272",
    "p15_cache_previous_faults_test.go": "77e4520b225285e0909e5991e162bb0c67c5b161dacb984a4a3051f4cbdd97d1",
    "p15_cache_previous_test.go": "6351f2e5f06b8709f85248af1aee15c03b34db216c0e72f08b6cbba844c10643",
    "p15_cache_ttl_captures_test.go": "822129f13e7a8c06aa6883c9e8ae1918830a2f516dc9130a463c5ba76ae5be04",
    "p15_cache_ttl_contract_test.go": "0a1de8d4855eb358297383b06da63617c026aac12511e2d739c875b5dbb2f9e5",
    "p15_cache_ttl_test.go": "eed602931c9c107fab194f3a206f154328a5b2f4396ce433add591306986dbd1",
    "p15_combined_value_blocks_test.go": "ed6f4ba43d546807e2ab9558163309a89cc30debd1f760611eb9a0bb0c64f112",
    "p15_extracted_install_contract_test.go": "5bcc206ada657943a6d0efaf4b10e11037df9ddbcbd95915ea25252afc314ee0",
    "p15_function_optional_predicate_test.go": "3023245cabdc0088d962fc6dfae174c4a7a024e2df0697d83b0b71eb437c1f75",
    "p15_install_actual_tools_test.go": "f643c0ec301bd8bae257e0f0d0ff71cef2fb8b6fb0847f6fda0597ec9803ab58",
    "p15_legacy_compound_conditions_test.go": "9b93bdb03a81014b2d928f93c93b1d6680a4ccbf18052b9de878232b6eb184de",
    "p15_legacy_conditional_operands_test.go": "ce9e32e25bad3400acc0570f84cde67e158bde31f9733b4599bc7e30a8bfe850",
    "p15_legacy_conditional_snapshot_oracle_test.go": "313e4ac6a64fbf24cc96b1963c3ec19e165cdc236cc311cc1ecf38a15bd0a974",
    "p15_main_output_graph_test.go": "af6dfa89af76970505122e47ee09a16851d49de3559ec441507aae90355bf0d3",
    "p15_main_void_return_test.go": "bb493466658026ce3378ae2d442daca96e9d261468210c3c2c886b49c91cab0f",
    "p15_measure_input_boundary_oracle_test.go": "d43b586b18f83da24b6505aed906a0d29894e3553d3fdf6c829136ffcd1a4beb",
    "p15_measure_previous_test.go": "846e933ce55f8028fee12ff9df2e0f5c145e262e36f5c993cda3571e3dd2ad98",
    "p15_measure_void_controls_test.go": "b2a105fd4a2c915e768f977b1ea7b186902a4be9004205130423134547f6cadb",
    "p15_measure_void_test.go": "894a9b20f09591d6ac0349b58264fa1afec451a8776bdf9e9d685ecc9a2589bb",
    "p15_migration_atomic_contract_test.go": "a0601b40c453b3fce85dafdad1baf2c18a80613ddc17522ddb8bdde7cf5dacbf",
    "p15_none_clear_boundary_test.go": "2312222a2495c673d6053182a19c35c719bbcb3404c7fb7b3631457bd8bef0f6",
    "p15_rp26_caller_input_void_test.go": "982b9e94e4870fd40b08f378ba3b4a8e48f195259b449d0efe6a391069d9e363",
    "p15_scoped_blocks_legacy_test.go": "b255796452e49fc3854ffc9a24d9b70bca1f3b386583a3ba7fc63ec21374f7d8",
    "p15_scoped_blocks_test.go": "9eefec5a98309c2f3825087a14e0a7e905398ba0bf573f1f5af6afcb73800ca5",
    "p15_scoped_value_transport_test.go": "b42a1b58edfb3e03d322ed96c52b3bfdc39103a19bd777431ae3e8dbd4db8988",
    "scoped_blocks_legacy.go": "4061b8f21a7aaffbae14c6f56f774586286a2a027235da1c0df6a03c3ff03d72",
    "tools/language-ai-eval/README.md": "b6e3121c69e3684b61a108e7f03068ab4d0ed16c2c73199b038dc9fd3f1c3fb1",
    "upstream_rehearsal_test.go": "d37cf9aa88dcae762572f7b1a30b6dbd9b818ff89199f0347476f60eb2686760"
}


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def validate_go(raw, packages, required):
    events = [json.loads(line) for line in raw.decode("utf-8").splitlines() if line.strip()]
    require(events and all(type(e) is dict for e in events), "ORIGINAL_GO_EVENTS")
    require(not any(e.get("Action") in ("fail", "skip") for e in events), "FAILED_OR_SKIPPED_GO_EVENT")
    passed_packages = [e["Package"] for e in events if e.get("Action") == "pass" and not e.get("Test")]
    require(sorted(passed_packages) == sorted(packages), "EXACT_COMPLETE_PACKAGE_TERMINALS")
    passed = [(e["Package"], e["Test"]) for e in events if e.get("Action") == "pass" and e.get("Test")]
    require(len(passed) == len(set(passed)), "DUPLICATE_NAMED_TERMINAL")
    for package, name in required:
        require(passed.count((package, name)) == 1, "MISSING_REQUIRED_PARENT:"+name)
    return {"packages": passed_packages, "namedPasses": len(passed), "requiredParents": required,
            "failed": 0, "skipped": 0, "originalStreamSHA256": sha(raw)}

def inspect_regression_stream(path, packages, no_test_packages, required, tick=lambda: None):
    """Read the entire authentic Go 1.24 JSONL without filtering or truncation.

    Ordinary original test skips remain visible. Required affected parents must
    pass; a package with no test files is distinct from a skipped named test.
    """
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode), "REGULAR_ORIGINAL_GO_STREAM")
    allowed = set(packages)
    require(len(allowed) == len(packages) and set(no_test_packages) <= allowed,
            "CLOSED_ORIGINAL_PACKAGE_SET")
    require(len(required) == len(set(required)) and all(p in allowed for p, _ in required),
            "CLOSED_AFFECTED_PARENT_SET")
    starts, package_terminals, named, builds = {}, {}, {}, {}
    output_events = 0
    output_bytes = 0
    digest = hashlib.sha256()
    size = lines = 0
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    with path.open("rb") as stream:
        for raw in stream:
            tick()
            size += len(raw)
            digest.update(raw)
            require(raw.endswith(b"\n") and raw.strip(), "COMPLETE_NONEMPTY_JSON_LINE")
            event = json.loads(raw.decode("utf-8"), object_pairs_hook=unique)
            lines += 1
            require(type(event) is dict and type(event.get("Action")) is str, "GO_EVENT_OBJECT")
            action = event["Action"]
            if action in ("build-output", "build-fail"):
                identity = event.get("ImportPath")
                require(type(identity) is str and identity and not event.get("Test"),
                        "GO_124_BUILD_IDENTITY")
                state = builds.setdefault(identity, {"outputEvents": 0, "failed": False})
                if action == "build-output":
                    require(type(event.get("Output")) is str, "ORIGINAL_BUILD_OUTPUT")
                    state["outputEvents"] += 1
                else:
                    require(not state["failed"], "DUPLICATE_BUILD_FAILURE")
                    state["failed"] = True
                continue
            require(action in ("start", "run", "pause", "cont", "pass", "fail", "skip", "output"),
                    "KNOWN_TEST_EVENT")
            package = event.get("Package")
            require(package in allowed, "UNEXPECTED_TEST_PACKAGE")
            name = event.get("Test")
            require(name is None or (type(name) is str and bool(name)), "NAMED_TEST_IDENTITY")
            if "Elapsed" in event:
                value = event["Elapsed"]
                require(type(value) in (int, float) and value == value and
                        value >= 0 and value < float("inf"), "FINITE_EVENT_ELAPSED")
            if action == "output":
                require(type(event.get("Output")) is str, "ORIGINAL_TEST_OUTPUT")
                output_events += 1
                output_bytes += len(event["Output"].encode("utf-8"))
                continue
            if action == "start":
                require(name is None and package not in starts and package not in package_terminals,
                        "ONE_PACKAGE_START")
                starts[package] = lines
                continue
            require(package in starts and package not in package_terminals,
                    "PACKAGE_LIFETIME")
            if name is None:
                require(action in ("pass", "fail", "skip"), "PACKAGE_TERMINAL_ACTION")
                package_terminals[package] = action
                continue
            key = (package, name)
            if action == "run":
                require(key not in named, "ONE_NAMED_START")
                named[key] = {"runLine": lines, "terminal": None, "paused": False}
            elif action in ("pause", "cont"):
                require(key in named and named[key]["terminal"] is None, "RUNNING_NAMED_EVENT")
                require(named[key]["paused"] is (action == "cont"), "PAUSE_CONT_ORDER")
                named[key]["paused"] = action == "pause"
            else:
                require(action in ("pass", "fail", "skip") and key in named and
                        named[key]["terminal"] is None, "ONE_NAMED_TERMINAL")
                named[key]["terminal"] = action
                named[key]["terminalLine"] = lines
    after = path.lstat()
    require(all(getattr(before, key) == getattr(after, key) for key in
                ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")),
            "UNCHANGED_COMPLETE_ORIGINAL_STREAM")
    require(lines > 0 and size == before.st_size, "ENTIRE_ORIGINAL_STREAM_READ")
    missing_packages = sorted(allowed - set(package_terminals))
    invalid_packages = {p: status for p, status in package_terminals.items()
                        if status != "pass" and not (status == "skip" and p in no_test_packages)}
    incomplete = sorted(key for key, value in named.items() if value["terminal"] is None)
    failed = sorted(key for key, value in named.items() if value["terminal"] == "fail")
    skipped = sorted(key for key, value in named.items() if value["terminal"] == "skip")
    mandatory_missing = sorted(key for key in required
                               if key not in named or named[key]["terminal"] != "pass")
    build_failures = sorted(key for key, value in builds.items() if value["failed"])
    complete = (set(starts) == allowed and not missing_packages and not invalid_packages and
                not incomplete and not failed and not mandatory_missing and not build_failures)
    return {"schema": "cherri-complete-go-regression-stream/1", "passed": complete,
            "packageTerminals": package_terminals, "expectedPackages": sorted(allowed),
            "noTestFilePackages": sorted(no_test_packages), "missingPackages": missing_packages,
            "nonpassingPackages": invalid_packages, "requiredParents": list(required),
            "requiredParentsNotPassed": mandatory_missing, "namedRuns": len(named),
            "namedPasses": sum(v["terminal"] == "pass" for v in named.values()),
            "namedFailures": failed, "namedSkips": skipped, "incompleteNamedTests": incomplete,
            "buildEvents": builds, "buildFailures": build_failures,
            "eventCount": lines, "testOutputEvents": output_events,
            "testOutputUTF8Bytes": output_bytes, "originalStreamBytes": size,
            "originalStreamSHA256": digest.hexdigest(),
            "namedTerminals": [{"package": p, "test": n, **value}
                               for (p, n), value in sorted(named.items())]}


def main():
    require(len(sys.argv) == 1 and platform.system() == "Linux" and platform.machine() in ("aarch64", "arm64"),
            "FIXED_NATIVE_LINUX_ARM64_ROUTE")
    environ = dict(os.environ)
    event = json.loads(Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    require(environ["GITHUB_REPOSITORY"] == "davidpovarsky/cherri" and
            environ["GITHUB_EVENT_NAME"] == "create" and event.get("ref_type") == "branch" and
            event.get("ref") == BRANCH and environ["GITHUB_REF"] == "refs/heads/"+BRANCH and
            environ["GITHUB_RUN_ATTEMPT"] == "1", "FIRST_EXACT_CREATE_EVENT")
    controller_sha = environ["GITHUB_SHA"]
    require(re.fullmatch("[0-9a-f]{40}", controller_sha) is not None, "CONTROLLER_SHA")
    workspace = Path(environ["GITHUB_WORKSPACE"]).resolve(strict=True)
    controller, source = workspace/"controller", workspace/"source"
    for directory in (controller, source):
        require(directory.resolve(strict=True) == directory and (directory/".git").is_dir() and
                not (directory/".git/objects/info/alternates").exists() and
                not (directory/".git/info/grafts").exists(), "ORDINARY_SEPARATE_SOURCE")
    helper = controller/(PREFIX+"owned_source_process.py")
    require(sha(helper.read_bytes()) == HELPER_SHA, "UNCHANGED_QUALIFIED_PROCESS_OWNER")
    spec = importlib.util.spec_from_file_location("distinct_function_owned_process", helper)
    owned = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owned)
    output = Path(environ["RUNNER_TEMP"]).resolve(strict=True)/"cherri-oracle-focused-57cf86"
    output.mkdir(mode=0o700)
    commands = output/"commands"
    commands.mkdir(mode=0o700)
    temporary = output.parent/"cherri-oracle-focused-tmp-57cf86"
    temporary.mkdir(mode=0o700)
    env = {key: environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ", "GOCACHE", "GOMODCACHE") if key in environ}
    env.update(PYTHONDONTWRITEBYTECODE="1", GOMAXPROCS="2", GOFLAGS="-p=2 -mod=readonly", GOMEMLIMIT="1GiB",
               CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL="2", GOTOOLCHAIN="local", GOENV="off", GOWORK="off", GOTELEMETRY="off",
               GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0",
               TMPDIR=str(temporary), GOTMPDIR=str(temporary), CGO_ENABLED="1")
    processes = []
    minimum = {"disk": shutil.disk_usage(output).free, "memory": 1 << 62}
    result = {"schema":"cherri-current-repair-focused-regression/1", "status":"FAILED",
              "source":SOURCE, "tree":TREE, "sourceParents":list(PARENTS), "controller":controller_sha,
              "runId":int(environ["GITHUB_RUN_ID"]), "attempt":1, "event":"create", "branch":BRANCH,
              "host":{"system":platform.system(), "machine":platform.machine(), "release":platform.release(),
                      "python":platform.python_version()}, "runner":"ubuntu-24.04-arm",
              "wholeOwnerSeconds":900, "goTestInternalSeconds":180,
              "compilationInclusiveChildSeconds":600, "processes":processes, "goTests":[],
              "requiredParentCount":20, "requiredScenarioChildCount":416,
              "fullSuiteScheduler":{"CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL":"2", "testParallel":2},
              "productsStatus":"NOT_PRODUCED", "products":{},
              "productsNotProducedReason":"SEPARATE_ROOT_OWNED_BUILD_ONLY_ROUTE_AFTER_FOCUSED_PASS",
              "prior7abFullOutcome":"FAIL_9_OLD_ORACLE_FAMILIES_AND_COMPARATOR_FIXTURE_AND_PACKAGE600_ALARM; original6d559dbf retained",
              "priorFull8333Outcome":"FAIL_ORIGINAL_PACKAGE_ALARM_AND_COMPARATOR_BASELINE; preserved e782d9f8",
              "priorFocused8333Outcome":"PASS_23_PREVIOUSLY_UNREACHED_PARENTS_618_EVENTS; separate d80ece16",
              "formalAcceptanceGranted":False, "rawNativeTypesObserved":False,
              "full179Executed":False, "nativeAppleExecuted":False, "providerInvoked":False,
              "privateCorpusPresent":False, "releasePromoted":False, "startedAt":owned.utc()}
    owned.original(output/"github-event.original.json", Path(environ["GITHUB_EVENT_PATH"]).read_bytes())
    with owned.WholeBudget(900, cleanup_seconds=8, final_reserve=60) as budget:
        def monitor():
            mem = {k:int(v.strip().split()[0])*1024 for k,v in
                   (line.split(":",1) for line in Path("/proc/meminfo").read_text().splitlines())}
            observed = {"disk":shutil.disk_usage(output).free,"memory":mem["MemAvailable"]}
            for key in minimum:
                minimum[key] = min(minimum[key],observed[key])
            require(observed["disk"] >= 2*1024**3 and observed["memory"] >= 1024**3, "HOSTED_LIVE_CAPACITY")
        def run(name, argv, seconds=20, finalizing=False, override=None):
            actual_name = "%03d-%s"%(len(processes)+1,name)
            row = owned.run_owned(argv, cwd=source, env={**env, **(override or {})},
                                  output=commands, name=actual_name, budget=budget,
                                  seconds=seconds, monitor=monitor,
                                  post_reserve=2 if finalizing else None)
            processes.append(row)
            owned.require_success(row)
            require(row["cleanup"]["parentReaped"] and row["cleanup"]["after"]["state"] == "ABSENT",
                    "OWNED_GROUP_REAPED_AND_ABSENT")
            return (commands/(actual_name+".stdout.original")).read_bytes()
        def git(directory,name,*args,finalizing=False):
            return run(name,["git","--no-replace-objects","-C",str(directory),*args],20,finalizing)
        def stable(path):
            before=path.lstat()
            require(stat.S_ISREG(before.st_mode) and before.st_size <= 128*1024**2, "REGULAR_BOUNDED_INPUT")
            digest=hashlib.sha256()
            blob=hashlib.sha1(("blob %d\0"%before.st_size).encode())
            with path.open("rb") as stream:
                for chunk in iter(lambda:stream.read(1024**2),b""):
                    budget.check()
                    digest.update(chunk);blob.update(chunk)
            after=path.lstat()
            require(all(getattr(before,k)==getattr(after,k) for k in
                        ("st_dev","st_ino","st_mode","st_uid","st_gid","st_size","st_mtime_ns","st_ctime_ns")),
                    "FILE_CHANGED_DURING_READ")
            return {"bytes":before.st_size,"sha256":digest.hexdigest(),"blob":blob.hexdigest(),
                    "mode":stat.S_IMODE(before.st_mode)}
        def snapshot(directory,head,tree=None,selected=None,finalizing=False):
            require(not git(directory,"replace","replace","-l",finalizing=finalizing), "NO_REPLACE_REFS")
            actual=git(directory,"identity","rev-parse","HEAD","HEAD^{tree}",finalizing=finalizing).decode().splitlines()
            require(actual[0]==head and (tree is None or actual[1]==tree), "EXACT_SOURCE_IDENTITY")
            require(not git(directory,"status","status","--porcelain=v1","--untracked-files=all",finalizing=finalizing),
                    "CLEAN_SOURCE")
            entries=git(directory,"tree","ls-tree","-rz","--full-tree","HEAD",
                        *(["--",*selected] if selected else []),finalizing=finalizing)
            files={}
            for entry in entries.split(b"\0"):
                if not entry:continue
                metadata,path=entry.split(b"\t",1);mode,kind,blob=metadata.decode().split()
                path=path.decode()
                require(mode in ("100644","100755") and kind=="blob" and
                        not path.startswith("/") and all(part not in ("",".","..",".git") for part in path.split("/"))
                        and path not in files,"CLOSED_TRACKED_PATH")
                observed=stable(directory/path)
                require(observed["blob"]==blob and (observed["mode"]&0o111)==(int(mode,8)&0o111),
                        "PHYSICAL_SOURCE_MATCHES_GIT")
                files[path]=observed
            require(files and (selected is None or set(files)==set(selected)),"COMPLETE_INPUT_INVENTORY")
            return {"source":actual[0],"tree":actual[1],"files":files}
        before=controller_before=None
        try:
            monitor()
            available_cpus=sorted(os.sched_getaffinity(0))
            admission_memory={k:int(v.strip().split()[0])*1024 for k,v in
                              (line.split(":",1) for line in Path("/proc/meminfo").read_text().splitlines())}["MemAvailable"]
            admission_disk=shutil.disk_usage(output).free
            require(len(available_cpus)>=2 and admission_memory>=4*1024**3 and admission_disk>=8*1024**3,
                    "TWO_WORKER_CPU_MEMORY_AND_DISK_ADMISSION")
            result["resourceProfile"]={"GOMAXPROCS":2,"GOFLAGS":"-p=2 -mod=readonly","GOMEMLIMIT":"1GiB",
                "testParallel":2,"availableCpuIds":available_cpus,"admissionMemoryAvailable":admission_memory,
                "admissionMemoryFloor":4*1024**3,"admissionDiskFree":admission_disk,"admissionDiskFloor":8*1024**3,"unchangedLiveMemoryFloor":1024**3,"unchangedLiveDiskFloor":2*1024**3,
                "memoryLimitKind":"Go managed-heap target, not an aggregate hard process memory cap",
                "qualifiedTwoWorkerOwnerSHA256":"567cb33831c5a4be315205068432ecc4ddc03e1de5d9f82ed78b24f08c0e8ac4"}
            before=snapshot(source,SOURCE,TREE)
            require(len(before["files"])==7963,"EXPECTED_SOURCE_FILE_COUNT")
            controller_before=snapshot(controller,controller_sha,selected=CONTROLLER_FILES)
            require(git(source,"source-parents","rev-list","--parents","-n","1","HEAD").decode().split()==
                    [SOURCE,*PARENTS],"EXACT_CURRENT_SOURCE_MERGE_PARENTS")
            owned.original(output/"source-before.original.json",before)
            owned.original(output/"controller-before.original.json",controller_before)
            for path in CONTROLLER_FILES:
                target=output/"controller-originals"/path
                target.parent.mkdir(parents=True,exist_ok=True)
                owned.original(target,(controller/path).read_bytes())
            version=run("go-version",["go","version"]).decode().strip()
            require(version=="go version go1.24.13 linux/arm64","PINNED_GO_TOOLCHAIN")
            result["goVersion"]=version
            result["goEnvironment"]=json.loads(run("go-environment",["go","env","-json","GOOS","GOARCH","CGO_ENABLED","GOVERSION","GOFLAGS","GOTOOLCHAIN","GOMOD","GOCACHE","GOMODCACHE"]))
            require(git(controller,"controller-parent","rev-parse","HEAD^").decode().strip()==SOURCE,
                    "CONTROLLER_IS_EXACT_SOURCE_CHILD")
            for path, digest in SOURCE_PINS.items():
                require(before["files"][path]["sha256"]==digest, "EXACT_REPAIR_AND_ORACLE_SOURCE:"+path)
            observed_parents=[]
            for path in SOURCE_PINS:
                if path.endswith("_test.go"):
                    directory=Path(path).parent.as_posix()
                    package=PACKAGE if directory=="." else PACKAGE+"/"+directory
                    for name in re.findall(r"(?m)^func (Test[A-Za-z0-9_]+)\(",(source/path).read_text()):
                        identity=(package,name)
                        if identity in REQUIRED_PARENTS:
                            observed_parents.append(identity)
            require(sorted(observed_parents)==sorted(REQUIRED_PARENTS) and len(observed_parents)==20,
                    "ALL_20_ORIGINAL_AND_REPAIR_FOCUSED_PARENTS")
            require(len(REQUIRED_SCENARIO_CHILDREN)==416 and len(set(REQUIRED_IDENTITIES))==436,
                    "ALL_EXACT_MEASURE_AND_COMPARATOR_SCENARIOS")
            require(set(EXPECTED_PACKAGES)=={PACKAGE,PACKAGE+"/internal/language/backend",PACKAGE+"/internal/shortcutcompare"},
                    "EXACT_THREE_FOCUSED_PACKAGES")
            # One actual command retains the original180-second package deadline
            # and600-second compilation-inclusive child. The selected -run keeps
            # the unchanged full-suite policy sequential, even with marker2.
            actual_name="%03d-focused-repair-go"%(len(processes)+1)
            process=owned.run_owned(list(REGRESSION_COMMAND),cwd=source,env=env,output=commands,
                                    name=actual_name,budget=budget,seconds=600,monitor=monitor)
            processes.append(process)
            result["focusedRepairSuiteExecuted"]=process["pid"] is not None
            result["originalGoCommand"]=list(REGRESSION_COMMAND)
            result["originalGoProcessName"]=actual_name
            result["originalGoStreams"]={key:process.get(key) for key in ("stdout","stderr")}
            require(process["cleanup"]["parentReaped"] and
                    process["cleanup"]["after"]["state"]=="ABSENT", "OWNED_GO_GROUP_CLOSED")
            require(process["stdout"]["complete"] and process["stderr"]["complete"],
                    "COMPLETE_UNCAPPED_ORIGINAL_GO_STREAMS")
            def intake_tick():
                budget.check()
            summary=inspect_regression_stream(commands/(actual_name+".stdout.original"),
                    list(EXPECTED_PACKAGES),NO_TEST_PACKAGES,
                    list(REQUIRED_IDENTITIES),tick=intake_tick)
            require(summary["originalStreamBytes"]==process["stdout"]["bytes"] and
                    summary["originalStreamSHA256"]==process["stdout"]["sha256"],
                    "ENTIRE_ORIGINAL_GO_STREAM_RECONCILED")
            result["goTests"].append(summary)
            result["goEventIndex"]=owned.original(output/"go-event-index.derived.json",summary)
            result["repairSourcePins"]=SOURCE_PINS
            owned.require_success(process)
            require(summary["passed"] and not summary["namedSkips"],"FOCUSED_REPAIR_REGRESSION_FAILED_OR_SKIPPED")
            result["status"]="PASS_FOCUSED_CURRENT_REPAIR_GO"

        except BaseException as error:
            result["error"]={"type":type(error).__name__,"message":str(error),"traceback":traceback.format_exc()}
        finally:
            try:
                with budget.finalizing():
                    after=snapshot(source,SOURCE,TREE,finalizing=True)
                    owned.original(output/"source-after.original.json",after)
                    controller_after=snapshot(controller,controller_sha,selected=CONTROLLER_FILES,finalizing=True)
                    owned.original(output/"controller-after.original.json",controller_after)
                    require(before==after and controller_before==controller_after,"SOURCE_OR_CONTROLLER_CHANGED")
                    require(sha(helper.read_bytes())==HELPER_SHA,"HELPER_CHANGED")
                    result["sourceUnchanged"]=True
            except BaseException as error:
                result["closingError"]={"type":type(error).__name__,"message":str(error)}
                result["status"]="FAILED"
            result["minimumCapacity"]=minimum
            result["finishedAt"]=owned.utc()
            receipt=owned.original(output/"result.original.json",result)
            print(json.dumps({"path":str(output/"result.original.json"),"sha256":receipt["sha256"],
                              "status":result["status"],"source":SOURCE,
                              "namedPasses":[x["namedPasses"] for x in result["goTests"]],
                              "streamBytes":result.get("originalGoStreams"),"error":result.get("error"),
                              "closingError":result.get("closingError")}))
    return 0 if result["status"]=="PASS_FOCUSED_CURRENT_REPAIR_GO" else 1


if __name__ == "__main__":
    raise SystemExit(main())
