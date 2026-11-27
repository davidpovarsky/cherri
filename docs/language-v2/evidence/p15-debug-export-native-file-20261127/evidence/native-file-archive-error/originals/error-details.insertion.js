// BEGIN bounded NSError diagnostic addition; no execution or action changes.
function diagnosticErrorText(target, selector, budget) {
    try {
        const value = obj(target, selector);
        if (isNil(value)) return {status: "NIL"};
        if (!nativeIsA(value, "NSString")) return {status: "UNSUPPORTED_TYPE"};
        const units = Number(value.length);
        const limit = selector === "domain" || selector === "className" ? 128 : 512;
        if (!Number.isInteger(units) || units < 0) return {status: "INVALID_LENGTH"};
        if (units > limit) return {status: "OMITTED_TEXT_BOUND"};
        const text = str(value);
        if (typeof text !== "string" || text.length > limit) return {status: "INVALID_STRING"};
        if (/(?:password|passwd|secret|authorization|cookie|(?:access|refresh|api)[ _-]?(?:token|key))\s*[:=]|Bearer\s+\S+|-----BEGIN [^-]*PRIVATE KEY-----|[a-z][a-z0-9+.-]*:\/\/[^\/\s]*@/i.test(text))
            return {status: "OMITTED_CREDENTIAL_LIKE_TEXT"};
        const bytes = Number($(text).dataUsingEncoding($.NSUTF8StringEncoding).length);
        if (!Number.isInteger(bytes) || bytes < 0 || bytes > 2048 || budget.bytes + bytes > 4096)
            return {status: "OMITTED_UTF8_BOUND"};
        budget.bytes += bytes;
        return {status: "OBSERVED", value: text, utf8Bytes: bytes};
    } catch (_) { return {status: "READ_FAILED"}; }
}
function diagnosticErrorFields(error, budget) {
    return {
        localizedDescription: diagnosticErrorText(error, "localizedDescription", budget),
        localizedFailureReason: diagnosticErrorText(error, "localizedFailureReason", budget),
        localizedRecoverySuggestion: diagnosticErrorText(error, "localizedRecoverySuggestion", budget)
    };
}
function diagnosticUnderlyingError(error, budget) {
    try {
        const info = obj(error, "userInfo");
        if (isNil(info)) return {status: "NIL_USER_INFO"};
        if (!nativeIsA(info, "NSDictionary")) return {status: "UNSUPPORTED_USER_INFO_TYPE"};
        // Read only this public Foundation key; never enumerate or serialize userInfo.
        const underlying = info.objectForKey($("NSUnderlyingError"));
        if (isNil(underlying)) return {status: "NIL"};
        if (!nativeIsA(underlying, "NSError")) return {status: "UNSUPPORTED_TYPE"};
        const code = Number(underlying.code);
        if (!Number.isSafeInteger(code)) return {status: "INVALID_CODE"};
        return {
            status: "OBSERVED",
            nativeClass: diagnosticErrorText(underlying, "className", budget),
            domain: diagnosticErrorText(underlying, "domain", budget),
            code: code,
            fields: diagnosticErrorFields(underlying, budget),
            furtherUnderlyingErrorRead: false
        };
    } catch (_) { return {status: "READ_FAILED"}; }
}
function observedErrorWithDiagnostic(error) {
    // These three original fields retain the exact original access and conversion.
    const original = {nativeClass: cls(error), domain: str(error.domain), code: Number(error.code)};
    try {
        if (!nativeIsA(error, "NSError")) {
            original.diagnostic = {status: "UNSUPPORTED_ERROR_TYPE"};
            return original;
        }
        const budget = {bytes: 0};
        const fields = diagnosticErrorFields(error, budget);
        const underlying = diagnosticUnderlyingError(error, budget);
        const detail = {status: "BOUNDED_DIAGNOSTIC_ONLY", fields: fields, underlyingError: underlying,
                        underlyingDepthLimit: 1, textUTF16Limit: 512, textUTF8Limit: 2048,
                        allTextUTF8Limit: 4096, observedTextUTF8Bytes: budget.bytes};
        const bytes = Number($(JSON.stringify(detail)).dataUsingEncoding($.NSUTF8StringEncoding).length);
        original.diagnostic = Number.isInteger(bytes) && bytes >= 0 && bytes <= 8192 ?
            detail : {status: "OMITTED_METADATA_BYTES_BOUND"};
    } catch (_) { original.diagnostic = {status: "READ_FAILED"}; }
    return original;
}
// END bounded NSError diagnostic addition.
