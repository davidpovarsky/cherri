package modules

// StandardLibrarySource returns the byte-exact generated snapshot of the preserved
// upstream stdfunc.cherri source. It is available to direct service, LSP and
// compiler consumers without process initialization or host-install reads.
func StandardLibrarySource() string { return standardLibrarySource }
