/*
This file implements data access of Device Key IDs from the Datastore
which are also cached and retrieved from the memcache is present.
*/

package cache

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"igps/ds"
)

// GetDevice returns the Device associated with the specified RandID.
// The implementation applies caching: first memcache is checked if the Device is already stored
// which is returned if so. Else the Device is loaded from the Datastore and it is put into the memcache
// before returning it.
//
// If there is no Device for the specified RandID, nil is returned as the Device,
// and it is not considered an error (err will be nil).
func GetDevice(c appengine.Context, RandID string) (dev *ds.Device, err error) {
	// First check in memcache:
	mk := prefixDevForRandID + RandID

	var item *memcache.Item
	if item, err = memcache.Get(c, mk); err == nil {
		// Found in memcache
		if len(item.Value) == 0 {
			// This means that the RandID is invalid, but was stored in the memcache
			// to prevent query repeating.
			return nil, nil
		}

		dev = new(ds.Device)
		dev.Decode(item.Value)
		return dev, nil
	}

	// If err == memcache.ErrCacheMiss it's just not present,
	// else real Error (e.g. memcache service is down).
	if err != memcache.ErrCacheMiss {
		c.Errorf("Failed to get %s from memcache: %v", mk, err)
	}

	// Either way we have to search in Datastore:

	// Do a keys-only query and lookup by key to see consistent value.
	// Lookup by key is strongly consistent.

	q := datastore.NewQuery(ds.ENameDevice).Filter(ds.PNameRandID+"=", RandID).KeysOnly().Limit(1)
	var devKeys []*datastore.Key
	if devKeys, err = q.GetAll(c, nil); err != nil {
		// Datastore error.
		c.Errorf("Failed to query Device by RandID: %v", err)
		return nil, err
	}

	if len(devKeys) == 0 {
		// Invalid RandID, but still store an empty value in the memcache
		// to prevent query repeating:
		if err = memcache.Set(c, &memcache.Item{Key: mk, Value: []byte{}}); err != nil {
			c.Warningf("Failed to set %s in memcache: %v", mk, err)
		}
		return nil, nil
	}

	// 'ts all good, valid RandID:
	dev = new(ds.Device)
	if err = datastore.Get(c, devKeys[0], dev); err != nil {
		// Datastore error.
		dev = nil
		c.Errorf("Failed to lookup Device by Key: %v", err)
		return nil, err
	}

	dev.KeyID = devKeys[0].IntID()

	// Also store it in memcache
	CacheDevice(c, dev)

	return dev, nil
}

// CacheDevice puts the specified Device into the cache (memcache).
func CacheDevice(c appengine.Context, dev *ds.Device) {
	mk := prefixDevForRandID + dev.RandID

	if err := memcache.Set(c, &memcache.Item{Key: mk, Value: dev.Encode()}); err != nil {
		c.Warningf("Failed to set %s in memcache: %v", mk, err)
	}
}

// ClearDeviceForRandID clears the cached Device for the specified RandID.
func ClearDeviceForRandID(c appengine.Context, RandID string) {
	mk := prefixDevForRandID + RandID

	if err := memcache.Delete(c, mk); err != nil {
		c.Warningf("Failed to delete %s from memcache: %v", mk, err)
	}
}
