package steve

// AllocSize is the initial allocation size of the buffer.
// If the requested capacity is larger than this initial size,
// then the internal buffer will grow to match the capacity
// requested as bytes are written.
const AllocSize = 512

type RingBuffer struct {
	buffer   []byte
	capacity int
	total    int
	wpos     int
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity == 0 {
		panic("NewRingBuffer: A capacity of zero is not allowed")
	}

	size := capacity
	// Only allocate the initial size of bytes at first
	if size > AllocSize {
		size = AllocSize
	}

	return &RingBuffer{
		buffer:   make([]byte, size),
		capacity: capacity,
		wpos:     0,
	}
}

func (r *RingBuffer) Write(b []byte) {
	// Do we need to consider growing the size of our buffer?
	if r.capacity != cap(r.buffer) && r.total <= r.capacity {
		// Is there room in the current buffer for this write?
		if r.total+len(b) > cap(r.buffer) {
			size := r.total + len(b)
			if size < 2*cap(b) {
				// Avoid making small allocations, go big or go home.
				size = 2 * cap(b)
				// But only allocate as much as our max capacity.
				if size > r.capacity {
					size = r.capacity
				}
			}
			b2 := make([]byte, size)
			copy(b2, r.buffer)
			r.buffer = b2
		}
	}

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

// Capacity returns the total number of bytes allocated for
// the ring buffer.
func (r *RingBuffer) Capacity() int {
	return cap(r.buffer)
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
}
