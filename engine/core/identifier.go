package core

import "fmt"

var Owners []interface{}

func IdentifierAquireNewID(owner interface{}) uint32 {
	if len(Owners) == 0 {
		Owners = make([]interface{}, 100)
	}
	length := uint32(len(Owners))
	for i := uint32(0); i < length; i++ {
		// Existing free spot. Take it.
		if Owners[i] == nil {
			Owners[i] = owner
			return i
		}
	}

	// If here, no existing free slots. Need a new id, so push one.
	// This means the id will be length - 1
	Owners = append(Owners, owner)
	length = uint32(len(Owners))
	return length - 1
}

func IdentifierReleaseID(id uint32) error {
	if len(Owners) == 0 {
		err := fmt.Errorf("identifier_release_id called before initialization. identifier_aquire_new_id should have been called first. Nothing was done")
		return err
	}

	length := uint32(len(Owners))
	if id > length {
		err := fmt.Errorf("identifier_release_id: id '%d' out of range (max=%d). Nothing was done", id, length)
		return err
	}

	// Just zero out the entry, making it available for use.
	Owners[id] = nil
	return nil
}
