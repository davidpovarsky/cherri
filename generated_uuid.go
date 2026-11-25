package main

import (
	"strconv"

	"github.com/google/uuid"
)

// One sequence spans frontend lowering and canonical macro expansion. A label
// alone is not an identity: anonymous controls and array items reuse labels.
type generatedUUIDSequence struct {
	derived      bool
	occurrence   uint64
	workflowName func() string
}

var generatedUUIDs generatedUUIDSequence

func configureGeneratedUUIDs(derived bool, name func() string) {
	generatedUUIDs = generatedUUIDSequence{derived: derived, workflowName: name}
}

func resetGeneratedUUIDs() {
	generatedUUIDs = generatedUUIDSequence{}
}

func nextGeneratedUUID(salt string) string {
	if !generatedUUIDs.derived {
		return uuid.NewString()
	}
	generatedUUIDs.occurrence++
	var name string
	if generatedUUIDs.workflowName != nil {
		name = generatedUUIDs.workflowName()
	}
	return deterministicUUID(
		[]byte("cherri-generated-workflow-uuid-v1"),
		[]byte(name),
		[]byte(salt),
		[]byte(strconv.FormatUint(generatedUUIDs.occurrence, 10)),
	)
}
