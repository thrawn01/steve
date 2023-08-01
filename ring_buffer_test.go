package steve

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

//goland:noinspection SpellCheckingInspection
func TestNewRingBuffer(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Write([]byte("Hello"))

	data, offset := rb.ReadOffset(0)
	assert.Equal(t, []byte("Hello"), data)

	// Given no more data has been written that would overflow
	// the buffer. Our read should always return "World"
	data, offset = rb.ReadOffset(0)
	assert.Equal(t, []byte("Hello"), data)

	// No new data has been written, so no data after the last
	// read offset is available
	data, offset = rb.ReadOffset(offset)
	assert.Equal(t, "", string(data))
	assert.Equal(t, 5, offset)

	// Write up to the current capacity of the buffer (10)
	rb.Write([]byte(" Worl"))

	// Attempt to read the remainder of the written buffer
	data, offset = rb.ReadOffset(5)
	assert.Equal(t, " Worl", string(data))
	assert.Equal(t, 10, offset)

	// Write one more byte such that we overflow the end of the ring
	// and begin writing to the beginning of the ring again.
	rb.Write([]byte("d"))

	// Attempt to read such that our read wraps around the ring until
	// we reach the current Write position.
	data, offset = rb.ReadOffset(5)
	assert.Equal(t, " World", string(data))
	assert.Equal(t, 11, offset)

	// Reading from the start will return the entire contents of the
	// buffer starting from the last written position.
	data, offset = rb.ReadOffset(0)
	assert.Equal(t, "ello World", string(data))
	assert.Equal(t, 11, offset)

	// Fill the buffer such that we overwrite the current buffer contents.
	rb.Write([]byte("0123456789"))
	assert.Equal(t, []byte("9012345678"), rb.Bytes())

	// Should return the slice we just wrote even though we overflowed
	// the ring.
	assert.Equal(t, 11, offset)
	data, offset = rb.ReadOffset(offset)
	assert.Equal(t, "0123456789", string(data))
	assert.Equal(t, 21, offset)

	// Nothing new has been written, so nothing is returned
	data, offset = rb.ReadOffset(offset)
	assert.Equal(t, "", string(data))
	assert.Equal(t, 21, offset)

	// Re-reading the 11th offset produces the same result
	data, offset = rb.ReadOffset(11)
	assert.Equal(t, "0123456789", string(data))
	assert.Equal(t, 21, offset)

	// Attempting to read past the current written offset
	// returns nothing and the current written offset
	data, offset = rb.ReadOffset(52342309)
	assert.Equal(t, "", string(data))
	assert.Equal(t, 21, offset)
}
