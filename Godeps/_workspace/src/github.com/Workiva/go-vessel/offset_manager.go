package vessel

import (
	"strconv"
	"sync"
)

// offsetManager encapsulates logic to handle offsets from the server.
type offsetManager struct {
	// The offsets of the latest seen messages
	offsets map[int32]int64
	// The offsets of the latest seen messages when a client disconnects
	readbackOffsets map[int32]int64
	lock            sync.RWMutex
}

// UpdateOffsets updates the offsets using the provided envelopes.
// Setting an offset to a lesser value than it was previously is not allowed.
func (o *offsetManager) UpdateOffsets(envelopes []*Envelope) {
	// The vessel server doesn't support offset tracking
	o.lock.Lock()
	if o.offsets == nil {
		o.lock.Unlock()
		return
	}

	for _, envelope := range envelopes {
		if envelope.Offset > o.offsets[envelope.Partition] {
			o.offsets[envelope.Partition] = envelope.Offset
		}
	}
	o.lock.Unlock()
}

// UpdateOffsetsFromStringMap updates the offsets using a map of the partitions
// as strings to offsets. Setting an offset to a lesser value than it was
// previously is not allowed.
func (o *offsetManager) UpdateOffsetsFromStringMap(offsets map[string]int64) error {
	o.lock.Lock()
	if o.offsets == nil {
		o.lock.Unlock()
		return nil
	}

	for partitionStr, offset := range offsets {
		partitionInt, err := strconv.ParseInt(partitionStr, 10, 32)
		if err != nil {
			return err
		}
		partition := int32(partitionInt)
		if offset > o.offsets[partition] {
			o.offsets[partition] = offset
		}
	}
	o.lock.Unlock()
	return nil
}

// GetOffsetStringMap returns the offsets as a mapping of partitions as strings
// to offsets.
func (o *offsetManager) GetOffsetStringMap() map[string]int64 {
	return o.offsetsToStringMap(o.offsets)
}

// GetReadbackOffsetStringMap returns the readback offsets as a mapping of
// partitions as strings to offsets
func (o *offsetManager) GetReadbackOffsetStringMap() map[string]int64 {
	return o.offsetsToStringMap(o.readbackOffsets)
}

// offsetsToStringmap is a helper function to convert a map[int32]int64 to a
// map[string]int64
func (o *offsetManager) offsetsToStringMap(offsets map[int32]int64) map[string]int64 {
	o.lock.RLock()
	var offsetsStr map[string]int64
	if offsets != nil {
		offsetsStr = make(map[string]int64, len(offsets))
		for key, value := range offsets {
			offsetsStr[strconv.FormatInt(int64(key), 10)] = value
		}
	}
	o.lock.RUnlock()
	return offsetsStr
}

// SupportsOffsetTracking returns true if the server supports offset tracking,
// false if it doesn't
func (o *offsetManager) SupportsOffsetTracking() bool {
	o.lock.RLock()
	supports := o.offsets != nil
	o.lock.RUnlock()
	return supports
}

// SetOffsets sets the offsets to the given offsets
func (o *offsetManager) SetOffsets(offsets map[int32]int64) {
	o.lock.Lock()
	o.offsets = offsets
	o.lock.Unlock()
}

// TransferOffsets sets the readback offsets to the current values of
// the offsets
func (o *offsetManager) TransferOffsets() {
	o.lock.Lock()
	o.readbackOffsets = make(map[int32]int64, len(o.offsets))
	for k, v := range o.offsets {
		o.readbackOffsets[k] = v
	}
	o.lock.Unlock()
}

// ClearReadbackOffsets clears the readback offsets
func (o *offsetManager) ClearReadbackOffsets() {
	o.lock.Lock()
	o.readbackOffsets = nil
	o.lock.Unlock()
}
