/*
This file implements data access of Accounts from the Datastore
which are also cached and retrieved from the memcache is present.
*/

package cache

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
	"igps/ds"
	"net/http"
	"strings"
	"time"
)

// Expiration duration for mem-cached Accounts.
// Significance of this is that Logins are logged if Account is not found in memcache.
var cachedAccExpiration = time.Hour * 12

// GetAccount returns the Account for the specified user.
// The implementation applies caching: first memcache is checked if the Account is already stored
// which returned if so. Else the Account is read from the Datastore and put into the memcache before returning it.
//
// If the specified user has no Account, nil is returned as acc,
// and it is not considered an error (err will be nil).
//
// If the account is not found in the memcache and therefore a Datastore query is performed,
// the event will be logged as a Login record.
func GetAccount(r *http.Request, c appengine.Context, u *user.User) (acc *ds.Account, err error) {
	// First check in memcache:
	mk := prefixAccForUID + u.ID

	var item *memcache.Item
	if item, err = memcache.Get(c, mk); err == nil {
		// Found in memcache
		if len(item.Value) == 0 {
			// This means that the user has no account, but was stored in the memcache
			// to prevent query repeating.
			return nil, nil
		}

		acc = new(ds.Account)
		acc.Decode(item.Value)
		return acc, nil
	}

	// If err == memcache.ErrCacheMiss it's just not present,
	// else real Error (e.g. memcache service is down).
	if err != memcache.ErrCacheMiss {
		c.Errorf("Failed to get %s from memcache: %v", mk, err)
	}

	// Either way we have to search in Datastore:

	// Do a keys-only query and lookup by key to see consistent value.
	// Lookup by key is strongly consistent.
	q := datastore.NewQuery(ds.ENameAccount).Filter(ds.PNameUserID+"=", u.ID).KeysOnly().Limit(1)
	var accKeys []*datastore.Key
	if accKeys, err = q.GetAll(c, nil); err != nil {
		// Datastore error.
		c.Errorf("Failed to query Accounts by UserID: %v", err)
		return nil, err
	}

	// Save Login record (regardless if the user is registered)
	// TODO: Consider only saving login record if no memcache error,
	// else if memcache service is down, it would generate a login record for all requests!
	defer func() {
		loc := strings.Join([]string{r.Header.Get("X-AppEngine-Country"), r.Header.Get("X-AppEngine-Region"), r.Header.Get("X-AppEngine-City")}, ";")
		var accId int64
		if acc != nil {
			accId = acc.KeyID
		}
		login := ds.Login{u.ID, u.Email, accId, r.UserAgent(), r.RemoteAddr, loc, time.Now()}
		login.Check()
		if _, err := datastore.Put(c, datastore.NewIncompleteKey(c, ds.ENameLogin, nil), &login); err != nil {
			c.Warningf("Failed to save Login: %v", err)
		}
	}()

	if len(accKeys) == 0 {
		// User has no account, but still store an empty value in the memcache
		// to prevent query repeating:
		if err = memcache.Set(c, &memcache.Item{Key: mk, Value: []byte{}, Expiration: cachedAccExpiration}); err != nil {
			c.Warningf("Failed to set %s in memcache: %v", mk, err)
		}
		return nil, nil
	}

	acc = new(ds.Account)
	if err = datastore.Get(c, accKeys[0], acc); err != nil {
		// Datastore error.
		acc = nil
		c.Errorf("Failed to lookup Account by Key: %v", err)
		return nil, err
	}

	acc.KeyID = accKeys[0].IntID()

	// Also store it in memcache
	CacheAccount(c, acc)

	return acc, nil
}

// CacheAccount puts the specified Account into the cache (memcache).
func CacheAccount(c appengine.Context, acc *ds.Account) {
	mk := prefixAccForUID + acc.UserID

	if err := memcache.Set(c, &memcache.Item{Key: mk, Value: acc.Encode(), Expiration: cachedAccExpiration}); err != nil {
		c.Warningf("Failed to set %s in memcache: %v", mk, err)
	}
}
