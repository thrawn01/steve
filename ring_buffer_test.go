package steve_test

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thrawn01/steve"
)

//goland:noinspection SpellCheckingInspection
func TestNewRingBuffer(t *testing.T) {
	rb := steve.NewRingBuffer(10)

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
	assert.Equal(t, 21, rb.Offset())
}

func TestRingBufferGrowToCapacity(t *testing.T) {
	rb := steve.NewRingBuffer(steve.AllocSize * 2)

	// Initial allocation should not be the requested capacity.
	assert.Equal(t, steve.AllocSize, rb.Capacity())

	// Ensure everything works before growing the buffer
	rb.Write([]byte("Hello, World"))
	data, offset := rb.ReadOffset(0)
	assert.Equal(t, "Hello, World", string(data))
	assert.Equal(t, 12, offset)

	// Write up to the current allocation size
	rb.Write(randomAlpha(steve.AllocSize - 12))
	// Capacity should not have changed
	assert.Equal(t, steve.AllocSize, rb.Capacity())

	// Write more than the current allocation
	rb.Write(randomAlpha(steve.AllocSize))

	// Capacity should be double
	assert.Equal(t, steve.AllocSize*2, rb.Capacity())

	// This should read the entire buffer
	data, offset = rb.ReadOffset(0)
	assert.Equal(t, "Hello, World", string(data[:12]))
	assert.Equal(t, steve.AllocSize*2, offset)

	// Should continue to cycle through the ring
	r := randomAlpha(steve.AllocSize)
	rb.Write(r)
	data, offset = rb.ReadOffset(offset)
	assert.Equal(t, r, data)
	assert.Equal(t, steve.AllocSize*2, rb.Capacity())
}

func TestRingBufferGrowEffectively(t *testing.T) {
	rb := steve.NewRingBuffer(steve.AllocSize * 10)

	// Initial allocation should not be the requested capacity.
	assert.Equal(t, steve.AllocSize, rb.Capacity())

	// Write twice the current allocation
	rb.Write(randomAlpha(steve.AllocSize * 2))

	// Should have grown to twice the size of the current capacity
	assert.Equal(t, steve.AllocSize*4, rb.Capacity())

	// Write over that allocation
	rb.Write(randomAlpha(steve.AllocSize * 3))
	assert.Equal(t, steve.AllocSize*6, rb.Capacity())

	// Write beyond our total capacity
	rb.Write(randomAlpha(steve.AllocSize * 10))

	// We should be at the total requested capacity
	assert.Equal(t, steve.AllocSize*10, rb.Capacity())
}

func TestEmptyBuffer(t *testing.T) {
	assert.Panics(t, func() {
		steve.NewRingBuffer(0)
	})
}

//func randomAlpha(size int) []byte {
//	buf := make([]byte, size)
//	unicodeRanges := fuzz.UnicodeRanges{
//		{First: 'a', Last: 'z'},
//		{First: '0', Last: '9'},
//	}
//	ff := fuzz.New().
//		Funcs(unicodeRanges.CustomStringFuzzFunc()).
//		NilChance(0).
//		NumElements(size, size)
//
//	ff.Fuzz(&buf)
//	fmt.Printf("buf: %s\n", buf)
//	return buf
//}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomAlpha(size int) []byte {
	randomBytes := make([]byte, size)
	charsetLength := big.NewInt(int64(len(charset)))

	for i := 0; i < size; i++ {
		randomIndex, _ := rand.Int(rand.Reader, charsetLength)
		randomBytes[i] = charset[randomIndex.Int64()]
	}

	return randomBytes
}
