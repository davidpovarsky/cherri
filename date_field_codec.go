package main

// WFDateFieldParameterState rejects a bare WFTextTokenAttachment for these
// two fields, even when the attachment points to a real Date producer. The
// native A/B probe in run 44696174000 retained the same reference inside a
// WFTextTokenString, matching the existing formatTimestamp Date encoder.
func canonicalDateFieldValue(identifier, key string, arg actionArgument) (any, bool) {
	if identifier != "is.workflow.actions.gettimebetweendates" ||
		(key != "WFTimeUntilFromDate" && key != "WFInput") {
		return nil, false
	}
	return paramValue(arg, String), true
}
