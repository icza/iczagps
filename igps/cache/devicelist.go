/*
This file implements data access of Device List from the Datastore
which are also cached and retrieved from the memcache is present.
*/

package cache

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"encoding/json"
	"igps/ds"
	"strconv"
)

// GetDevListForAccKey returns the Device list (only name and KeyID is populatd) for the specified Account.
// The implementation applies caching: first memcache is checked if the Device list is already stored
// which is returned if so. Else the Device list is read from the Datastore and the list is put into the memcache
// before returning it.
//
// If there is no Device for the specified Account, nil is returned as the devices,
// and it is not considered an error (err will be nil).
func GetDevListForAccKey(c appengine.Context, accKey *datastore.Key) (devices []*ds.Device, err error) {
	// First check in memcache:
	mk := prefixDevListForAccKey + strconv.FormatInt(accKey.IntID(), 10)

	var item *memcache.Item
	if item, err = memcache.Get(c, mk); err == nil {
		// Found in memcache
		var devices []*ds.Device
		err = json.Unmarshal(item.Value, &devices)
		if err != nil {
			c.Errorf("Invalid DevList value stored in memcache: %s", item.Value)
			return nil, err
		}
		return devices, nil
	}

	// If err == memcache.ErrCacheMiss it's just not present,
	// else real Error (e.g. memcache service is down).
	if err != memcache.ErrCacheMiss {
		c.Errorf("Failed to get %s from memcache: %v", mk, err)
	}

	// Either way we have to search in Datastore:

	q := datastore.NewQuery(ds.ENameDevice).Ancestor(accKey).Order(ds.PNameName)

	var devKeys []*datastore.Key
	if devKeys, err = q.GetAll(c, &devices); err != nil {
		// Datastore error.
		c.Errorf("Failed to query Device list by ancestor: %v", err)
		return nil, err
	}
	for i := range devices {
		devices[i].KeyID = devKeys[i].IntID()
	}

	// Also store it in memcache
	cacheDevListForAccKey(c, accKey, devices)

	return devices, nil
}

// cacheDevListForAccKey puts the specified Device list into the cache (memcache).
func cacheDevListForAccKey(c appengine.Context, accKey *datastore.Key, devices []*ds.Device) {
	mk := prefixDevListForAccKey + strconv.FormatInt(accKey.IntID(), 10)

	data, err := json.Marshal(devices) // This can't really fail
	if err != nil {
		c.Errorf("Failed to encode device list to JSON: %v", err)
	}

	if err = memcache.Set(c, &memcache.Item{Key: mk, Value: data}); err != nil {
		c.Warningf("Failed to set %s in memcache: %v", mk, err)
	}
}

// ClearDevListForAccKey clears the cached Device list for the specified Account Key.
func ClearDevListForAccKey(c appengine.Context, accKey *datastore.Key) {
	mk := prefixDevListForAccKey + strconv.FormatInt(accKey.IntID(), 10)
	if err := memcache.Delete(c, mk); err != nil {
		c.Warningf("Failed to delete %s from memcache: %v", mk, err)
	}
}
