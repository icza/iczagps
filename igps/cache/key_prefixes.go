/*
Defines constants for key prefixes used in the memcache.
*/

package cache

// Memcache key prefixes must be unique and none can be a prefix of another!

const (
	// Memcache key prefix for Account for User ID.
	prefixAccForUID = "accForUID:"

	// Memcache key prefix for Device for RandID
	prefixDevForRandID = "devForRandID:"

	// Memcache key prefix for Device list for an Account Key
	prefixDevListForAccKey = "devListForAccKey:"
)
