package azure

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFixEndiannessUUID(t *testing.T) {
	var toFix = "60D7F925-4C67-DF44-A144-A3FE111ECDE3"
	var expected = ("25F9D760-674C-44DF-A144-A3FE111ECDE3")
	var result = fixEndiannessUUID(toFix)
	assert.Equal(t, result, expected)
}

func TestDoubleFixShouldProduceSameString(t *testing.T) {
	var toFix = "60D7F925-4C67-DF44-A144-A3FE111ECDE3"
	var tmp = fixEndiannessUUID(toFix)
	var result = fixEndiannessUUID(tmp)
	assert.Equal(t, result, toFix)
}

func TestFixEndiannessUUIDFailsOnInvalidUUID(t *testing.T) {
	assert.Panics(t, func() {
		var toFix = "60D7F925-4C67-DF44-A144-A3FE111ECDE3XXXX"
		_ = fixEndiannessUUID(toFix)
	}, "Calling with invalid UUID should panic")

}

func TestFixEndiannessUUIDFailsOnInvalidUUID2(t *testing.T) {
	assert.Panics(t, func() {
		var toFix = "60D7-F925-4C67-DF44-A144-A3FE-111E-CDE3-XXXX"
		_ = fixEndiannessUUID(toFix)
	}, "Calling with invalid UUID should panic")

}

func TestReverseBytes(t *testing.T) {
	assert.Equal(t, "CDAB", reverseBytes("ABCD"))
}
