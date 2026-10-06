package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/source"
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
	CodeSyntax            = "E_SYNTAX"
	CodeUnterminatedLit   = "E_UNTERMINATED_LITERAL"
	CodeUnknownName       = "E_UNKNOWN_NAME"
	CodeDuplicateName     = "E_DUPLICATE_NAME"
	CodeAssignImmutable   = "E_ASSIGN_IMMUTABLE"
	CodeReadBeforeInit    = "E_READ_BEFORE_INIT"
	CodeUnknownAction     = "E_UNKNOWN_ACTION"
	CodeUnknownArgument   = "E_UNKNOWN_ARGUMENT"
	CodeDuplicateArgument = "E_DUPLICATE_ARGUMENT"
	CodeMissingArgument   = "E_MISSING_ARGUMENT"
	CodePositionalArg     = "E_POSITIONAL_ARGUMENT"
	CodeArgumentType      = "E_ARGUMENT_TYPE"
	CodeLiteralRequired   = "E_LITERAL_REQUIRED"
	CodeEnumMember        = "E_ENUM_MEMBER"
	CodeReturnType        = "E_RETURN_TYPE"
	CodeMissingReturn     = "E_MISSING_RETURN"
	CodeYieldContext      = "E_YIELD_CONTEXT"
	CodeTargetValueShape  = "E_TARGET_VALUE_SHAPE"
	CodeUnsupportedTarget = "E_UNSUPPORTED_TARGET"
	CodeImportCycle       = "E_IMPORT_CYCLE"
	CodeWorkspaceAccess   = "E_WORKSPACE_ACCESS"
	CodeSetupBinding      = "E_SETUP_BINDING"
	CodeSchemaMismatch    = "E_SCHEMA_MISMATCH"
	CodeLegacySyntax      = "E_LEGACY_SYNTAX"

	// Warnings
	CodeShadowing       = "W_SHADOWING"
	CodeUnverifiedType  = "W_UNVERIFIED_TYPE"
	CodeUnverifiedForm  = "W_UNVERIFIED_FORM"
	CodeUnusedSetup     = "W_UNUSED_SETUP"
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
