package main

// makeArchiveFormatValues preserves the public archiveFormat spellings while
// encoding the IDs from docs/apple-runtime/macos-27/toolkit-registry.json.
// This is deliberately limited to the eight formats already in the public API.
var makeArchiveFormatValues = [...]struct {
	source string
	wire   string
}{
	{".zip", "zip"},
	{".tar.gz", "tar.gz"},
	{".tar.bz2", "tar.bz2"},
	{".tar.xz", "tar.xz"},
	{".tar", "tar"},
	{".gz", "gz"},
	{".cpio", "cpio"},
	{".iso", "iso"},
}

func isMakeArchiveFormatParameter(identifier string, param parameterDefinition) bool {
	return identifier == "is.workflow.actions.makezip" &&
		param.key == "WFArchiveFormat" && param.enum == "archiveFormat"
}

// canonicalArchiveFormatValue converts only literal public enum values.
// References, omitted arguments and explicit raw actions keep their own encoders.
func canonicalArchiveFormatValue(identifier string, param parameterDefinition, arg actionArgument) (string, bool) {
	if !isMakeArchiveFormatParameter(identifier, param) ||
		(arg.valueType != String && arg.valueType != RawString) {
		return "", false
	}
	var value, literal = arg.value.(string)
	if !literal {
		return "", false
	}
	for _, format := range makeArchiveFormatValues {
		if value == format.source {
			return format.wire, true
		}
	}
	return "", false
}

// archiveFormatSourceValue recognizes canonical native IDs only. Historical
// dotted native values and unknown IDs need raw fallback to preserve their bytes.
func archiveFormatSourceValue(wire string) (string, bool) {
	for _, format := range makeArchiveFormatValues {
		if wire == format.wire {
			return format.source, true
		}
	}
	return "", false
}
