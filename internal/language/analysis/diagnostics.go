package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Severity indicates diagnostic seriousness.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInformation
	SeverityHint
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInformation:
		return "INFO"
	case SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}

// Stable diagnostic codes from §16.2
const (
	CodeSyntax                  = syntax.CodeSyntax
	CodeUnterminatedLit         = "E_UNTERMINATED_LITERAL"
	CodeUnknownName             = "E_UNKNOWN_NAME"
	CodeDuplicateName           = "E_DUPLICATE_NAME"
	CodeAssignImmutable         = "E_ASSIGN_IMMUTABLE"
	CodeReadBeforeInit          = "E_READ_BEFORE_INIT"
	CodeUnknownAction           = "E_UNKNOWN_ACTION"
	CodeUnknownArgument         = "E_UNKNOWN_ARGUMENT"
	CodeDuplicateArgument       = "E_DUPLICATE_ARGUMENT"
	CodeMissingArgument         = "E_MISSING_ARGUMENT"
	CodePositionalArg           = "E_POSITIONAL_ARGUMENT"
	CodeArgumentType            = "E_ARGUMENT_TYPE"
	CodeTypeAlias               = "E_TYPE_ALIAS"
	CodeNativeJSONNull          = "E_NATIVE_JSON_NULL"
	CodeNestedNativeList        = "E_NESTED_NATIVE_LIST"
	CodeLiteralRequired         = "E_LITERAL_REQUIRED"
	CodeEnumMember              = "E_ENUM_MEMBER"
	CodeReturnType              = "E_RETURN_TYPE"
	CodeMainOutputMode          = "E_MAIN_OUTPUT_MODE"
	CodeMissingReturn           = "E_MISSING_RETURN"
	CodeYieldContext            = "E_YIELD_CONTEXT"
	CodeTargetValueShape        = "E_TARGET_VALUE_SHAPE"
	CodeUnsupportedTarget       = "E_UNSUPPORTED_TARGET"
	CodePlatformVersionMetadata = "E_PLATFORM_VERSION_METADATA"
	CodeImportCycle             = "E_IMPORT_CYCLE"
	CodeWorkspaceAccess         = "E_WORKSPACE_ACCESS"
	CodeSetupBinding            = "E_SETUP_BINDING"
	CodeSchemaMismatch          = "E_SCHEMA_MISMATCH"
	CodeLegacySyntax            = syntax.CodeLegacySyntax

	// Warnings
	CodeShadowing                 = "W_SHADOWING"
	CodeUnverifiedType            = "W_UNVERIFIED_TYPE"
	CodeUnverifiedForm            = "W_UNVERIFIED_FORM"
	CodeUnusedSetup               = "W_UNUSED_SETUP"
	CodePlatformVersionUnverified = "W_PLATFORM_VERSION_UNVERIFIED"
)

// Diagnostic represents a compiler or language-service diagnostic.
type Diagnostic struct {
	Code     string      `json:"code"`
	Severity Severity    `json:"severity"`
	Span     source.Span `json:"span"`
	Message  string      `json:"message"`
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %s: %s (%s)", d.Severity, d.Code, d.Message, d.Span)
}
