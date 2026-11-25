package backend

// ScopedPageCallback describes an inline, compile-time page body. The canonical
// paginator emits its helpers through EmitAction and invokes Body exactly once
// in the validated page region. It is not an escaping/runtime closure.
type ScopedPageCallback struct {
	InputName  string
	EmitAction func(appleIdentifier string, parameters map[string]any) error
	Body       func(page Reference) error
}

// ScopedPaginationSession reuses canonical argument binding and native codecs
// for a page callback. Ordinary pagination still uses Session.EmitResolvedCall.
type ScopedPaginationSession interface {
	EmitScopedPaginateCall(call ResolvedCall, callback ScopedPageCallback) (string, error)
}
