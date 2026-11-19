# Editor Inspector

Status: `NEEDS_REVIEW`

Expected native actions: 120; observed AX containers: 7.

This compares independent compiled plist data with AX observations. Hidden values and missing identity remain unverified.
Historical detectedActionCards counts distinct UI strings, including toolbar controls and parameters; it is not a native action count.

| Native index | Identifier | Status |
|---|---|---|
| 0 | is.workflow.actions.comment | AX_UNEXPOSED |
| 1 | is.workflow.actions.gettext | AX_UNEXPOSED |
| 2 | is.workflow.actions.setvariable | AX_UNEXPOSED |
| 3 | is.workflow.actions.gettext | AX_UNEXPOSED |
| 4 | is.workflow.actions.appendvariable | AX_UNEXPOSED |
| 5 | is.workflow.actions.gettext | AX_UNEXPOSED |
| 6 | is.workflow.actions.gettext | AX_UNEXPOSED |
| 7 | is.workflow.actions.base64encode | NOT_FOUND |
| 8 | is.workflow.actions.setitemname | NOT_FOUND |
| 9 | is.workflow.actions.getitemname | NOT_FOUND |
| 10 | is.workflow.actions.detect.text | NOT_FOUND |
| 11 | is.workflow.actions.gettext | NOT_FOUND |
| 12 | is.workflow.actions.base64encode | NOT_FOUND |
| 13 | is.workflow.actions.setitemname | NOT_FOUND |
| 14 | is.workflow.actions.setvariable | NOT_FOUND |
| 15 | is.workflow.actions.getitemname | NOT_FOUND |
| 16 | is.workflow.actions.detect.text | NOT_FOUND |
| 17 | is.workflow.actions.gettext | NOT_FOUND |
| 18 | is.workflow.actions.base64encode | NOT_FOUND |
| 19 | is.workflow.actions.setitemname | NOT_FOUND |
| 20 | is.workflow.actions.getitemname | NOT_FOUND |
| 21 | is.workflow.actions.setvariable | NOT_FOUND |
| 22 | is.workflow.actions.gettext | NOT_FOUND |
| 23 | is.workflow.actions.base64encode | NOT_FOUND |
| 24 | is.workflow.actions.setitemname | NOT_FOUND |
| 25 | is.workflow.actions.detect.text | NOT_FOUND |
| 26 | is.workflow.actions.setvariable | NOT_FOUND |
| 27 | is.workflow.actions.gettext | NOT_FOUND |
| 28 | is.workflow.actions.setvariable | NOT_FOUND |
| 29 | is.workflow.actions.conditional | NOT_FOUND |
| 30 | is.workflow.actions.gettext | NOT_FOUND |
| 31 | is.workflow.actions.setvariable | NOT_FOUND |
| 32 | is.workflow.actions.conditional | NOT_FOUND |
| 33 | is.workflow.actions.conditional | NOT_FOUND |
| 34 | is.workflow.actions.gettext | NOT_FOUND |
| 35 | is.workflow.actions.setvariable | NOT_FOUND |
| 36 | is.workflow.actions.conditional | NOT_FOUND |
| 37 | is.workflow.actions.conditional | NOT_FOUND |
| 38 | is.workflow.actions.gettext | NOT_FOUND |
| 39 | is.workflow.actions.setvariable | NOT_FOUND |
| 40 | is.workflow.actions.conditional | NOT_FOUND |
| 41 | is.workflow.actions.conditional | NOT_FOUND |
| 42 | is.workflow.actions.gettext | NOT_FOUND |
| 43 | is.workflow.actions.setvariable | NOT_FOUND |
| 44 | is.workflow.actions.conditional | NOT_FOUND |
| 45 | is.workflow.actions.conditional | NOT_FOUND |
| 46 | is.workflow.actions.gettext | NOT_FOUND |
| 47 | is.workflow.actions.setvariable | NOT_FOUND |
| 48 | is.workflow.actions.conditional | NOT_FOUND |
| 49 | is.workflow.actions.gettext | NOT_FOUND |
| 50 | is.workflow.actions.setvariable | NOT_FOUND |
| 51 | is.workflow.actions.conditional | NOT_FOUND |
| 52 | is.workflow.actions.gettext | NOT_FOUND |
| 53 | is.workflow.actions.setvariable | NOT_FOUND |
| 54 | is.workflow.actions.conditional | NOT_FOUND |
| 55 | is.workflow.actions.conditional | NOT_FOUND |
| 56 | is.workflow.actions.gettext | NOT_FOUND |
| 57 | is.workflow.actions.setvariable | NOT_FOUND |
| 58 | is.workflow.actions.conditional | NOT_FOUND |
| 59 | is.workflow.actions.conditional | NOT_FOUND |
| 60 | is.workflow.actions.gettext | NOT_FOUND |
| 61 | is.workflow.actions.setvariable | NOT_FOUND |
| 62 | is.workflow.actions.conditional | NOT_FOUND |
| 63 | is.workflow.actions.conditional | NOT_FOUND |
| 64 | is.workflow.actions.gettext | NOT_FOUND |
| 65 | is.workflow.actions.setvariable | NOT_FOUND |
| 66 | is.workflow.actions.conditional | NOT_FOUND |
| 67 | is.workflow.actions.conditional | NOT_FOUND |
| 68 | is.workflow.actions.gettext | NOT_FOUND |
| 69 | is.workflow.actions.setvariable | NOT_FOUND |
| 70 | is.workflow.actions.conditional | NOT_FOUND |
| 71 | is.workflow.actions.gettext | NOT_FOUND |
| 72 | is.workflow.actions.setvariable | NOT_FOUND |
| 73 | is.workflow.actions.conditional | NOT_FOUND |
| 74 | is.workflow.actions.gettext | NOT_FOUND |
| 75 | is.workflow.actions.setvariable | NOT_FOUND |
| 76 | is.workflow.actions.conditional | NOT_FOUND |
| 77 | is.workflow.actions.conditional | NOT_FOUND |
| 78 | is.workflow.actions.gettext | NOT_FOUND |
| 79 | is.workflow.actions.setvariable | NOT_FOUND |
| 80 | is.workflow.actions.conditional | NOT_FOUND |
| 81 | is.workflow.actions.conditional | NOT_FOUND |
| 82 | is.workflow.actions.gettext | NOT_FOUND |
| 83 | is.workflow.actions.setvariable | NOT_FOUND |
| 84 | is.workflow.actions.conditional | NOT_FOUND |
| 85 | is.workflow.actions.conditional | NOT_FOUND |
| 86 | is.workflow.actions.gettext | NOT_FOUND |
| 87 | is.workflow.actions.setvariable | NOT_FOUND |
| 88 | is.workflow.actions.conditional | NOT_FOUND |
| 89 | is.workflow.actions.conditional | NOT_FOUND |
| 90 | is.workflow.actions.gettext | NOT_FOUND |
| 91 | is.workflow.actions.setvariable | NOT_FOUND |
| 92 | is.workflow.actions.conditional | NOT_FOUND |
| 93 | is.workflow.actions.gettext | NOT_FOUND |
| 94 | is.workflow.actions.setvariable | NOT_FOUND |
| 95 | is.workflow.actions.gettext | NOT_FOUND |
| 96 | is.workflow.actions.setvariable | NOT_FOUND |
| 97 | is.workflow.actions.gettext | NOT_FOUND |
| 98 | is.workflow.actions.setvariable | NOT_FOUND |
| 99 | is.workflow.actions.conditional | NOT_FOUND |
| 100 | is.workflow.actions.gettext | NOT_FOUND |
| 101 | is.workflow.actions.setvariable | NOT_FOUND |
| 102 | is.workflow.actions.conditional | NOT_FOUND |
| 103 | is.workflow.actions.conditional | NOT_FOUND |
| 104 | is.workflow.actions.gettext | NOT_FOUND |
| 105 | is.workflow.actions.setvariable | NOT_FOUND |
| 106 | is.workflow.actions.conditional | NOT_FOUND |
| 107 | is.workflow.actions.conditional | NOT_FOUND |
| 108 | is.workflow.actions.gettext | NOT_FOUND |
| 109 | is.workflow.actions.setvariable | NOT_FOUND |
| 110 | is.workflow.actions.conditional | NOT_FOUND |
| 111 | is.workflow.actions.gettext | NOT_FOUND |
| 112 | is.workflow.actions.comment | NOT_FOUND |
| 113 | is.workflow.actions.gettext | NOT_FOUND |
| 114 | is.workflow.actions.appendvariable | NOT_FOUND |
| 115 | is.workflow.actions.text.combine | NOT_FOUND |
| 116 | is.workflow.actions.gettext | NOT_FOUND |
| 117 | is.workflow.actions.gettext | NOT_FOUND |
| 118 | is.workflow.actions.comment | NOT_FOUND |
| 119 | is.workflow.actions.output | NOT_FOUND |

## Limitations

- ACTION_COUNT_MISMATCH
- EDITOR_BOTTOM_UNVERIFIED
