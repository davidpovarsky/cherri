package lower

// SetUUIDGenerator changes only generated identities. Imported provenance is
// consumed first by GenerateUUID, and the default remains cryptographic random.
func (l *Lowerer) SetUUIDGenerator(generator func() string) {
	l.uuidGenerator = generator
}
