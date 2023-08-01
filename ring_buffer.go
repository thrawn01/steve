package steve

type RingBuffer struct {
	buffer   []byte
	capacity int
	total    int
	wpos     int
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		// TODO: Don't allocate the entire max capacity all at once.
		buffer:   make([]byte, capacity),
		capacity: capacity,
		wpos:     0,
	}
}

func (r *RingBuffer) Write(b []byte) {
	r.total += len(b)
	for _, v := range b {
		r.buffer[r.wpos] = v
		r.wpos = (r.wpos + 1) % r.capacity
	}
}

// Bytes will return the entire buffer for the ring.
// The byte slice returned is a reference to the actual
// internal buffer.
func (r *RingBuffer) Bytes() []byte {
	return r.buffer
}

// Offset will return the current written offset which
// can be used to start reading at the end of the current
// buffer.
func (r *RingBuffer) Offset() int {
	return r.total
}

func (r *RingBuffer) ReadOffset(offset int) ([]byte, int) {
	// If the offset is the same or outside the bounds
	// of the total written, then return empty bytes
	// and the current total.
	if offset >= r.total {
		return []byte(""), r.total
	}

	// Given the requested offset, calculate where in
	// the current ring the position should be.
	pos := offset % r.capacity

	// If the offset requested is the offset of a previous ring,
	// we don't have that data anymore. In this case, we return the
	// entire buffer contents starting from the current Write position.
	// OR
	// If our read position is the same as the current Write position, this
	// means we are a full ring cycle behind and need to read the entire ring.
	if offset < (r.total-r.capacity) || pos == r.wpos {
		data := make([]byte, r.capacity)
		// Copy bytes from the current Write position until the end of the buffer
		copy(data, r.buffer[r.wpos:r.capacity])
		// Read from the beginning of the buffer until the last Write position.
		copy(data[r.capacity-r.wpos:], r.buffer[:r.wpos])
		return data, r.total
	}

	if r.wpos < pos {
		data := make([]byte, r.capacity-pos+r.wpos)
		// Copy remaining bytes until the end of the buffer
		copy(data, r.buffer[pos:r.capacity])
		// Read from the beginning of the buffer until the last Write position.
		copy(data[r.capacity-pos:], r.buffer[:r.wpos])
		return data, offset + len(data)
	}

	data := make([]byte, r.wpos-pos)
	copy(data, r.buffer[pos:r.wpos])
	return data, offset + len(data)

	//if pos < r.rpos || pos >= r.wpos {
	//	data := make([]byte, r.wpos-r.rpos)
	//	copy(data, r.buffer[r.rpos:r.wpos])
	//	//offset = r.wpos
	//	return data, offset + len(data)
	//}
	//
	//data := make([]byte, r.wpos-pos)
	//copy(data, r.buffer[pos:r.wpos])
	//return data, r.wpos
}
