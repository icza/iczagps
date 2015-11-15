/*
Defines the Account type.
*/

package ds

import (
	"appengine"
	"appengine/datastore"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Name of the Datastore Account entity
const ENameAccount = "Acc"

// Account type.
type Account struct {
	// Google Account email address.
	// Can be changed upon user's request (together with UserID!).
	Email string `datastore:"gae" json:"gae"`

	// Lowercased Google Account email address.
	Lemail string `datastore:"lgae" json:"lgae"`

	// Unique permanent user ID, see appengine/user.User.ID
	// Can be changed upon user's request (together with Email!).
	UserID string `datastore:"uid" json:"uid"`

	// Contact email address
	ContactEmail string `datastore:"ce" json:"ce"`

	// Location name to derive the time.Location for date/time formatting
	LocationName string `datastore:"ln" json:"ln"`

	// Map Preview Size in format of "widthxheight", e.g. "500x500"
	MapPrevSize string `datastore:"mps" json:"mps"`

	// Mobile Map Preview Size in format of "widthxheight", e.g. "500x500"
	MobMapPrevSize string `datastore:"mmps" json:"mmps"`

	// Mobile Map Image format.
	MobMapImgFormat string `datastore:"mmif" json:"mmif"`

	// Mobile Page width in pixels.
	MobPageWidth int `datastore:"mpw" json:"mpw"`

	// Logs Page Size
	LogsPageSize int `datastore:"lps" json:"lps"`

	// Timestamp
	Created time.Time `datastore:"t" json:"t"`

	// ------------------------------------------------------------------------------
	// Derived/computed fields

	// ID field of the Account's key.
	KeyID int64 `datastore:"-"`

	// Cached location to use for date/time formatting
	location *time.Location `datastore:"-" json:"-"`
}

// Encode encodes the Account into a []byte using JSON.
func (a *Account) Encode() []byte {
	b, err := json.Marshal(a) // This can't really fail...
	if err != nil {
		panic(err)
	}
	return b
}

// Decode decodes the Account from a JSON []byte.
func (a *Account) Decode(b []byte) {
	err := json.Unmarshal(b, a)
	if err != nil {
		panic(err)
	}
}

// GetLocation tries to load the location specified by the LocationName.
func (a *Account) Location() *time.Location {
	if a.location == nil {
		a.location, _ = getCachedLocation(a.LocationName)
	}
	return a.location
}

// GetLogsPageSize returns the Logs Page Size.
func (a *Account) GetLogsPageSize() int {
	// Default value:
	if a.LogsPageSize == 0 {
		return 15
	}
	return a.LogsPageSize
}

// GetMapPrevSize returns the width and height of the map preview.
func (a *Account) GetMapPrevSize() (width, height int) {
	return parseSize(a.MapPrevSize)
}

// GetMobMapPrevSize returns the width and height of the mobile map preview.
func (a *Account) GetMobMapPrevSize() (width, height int) {
	return parseSize(a.MobMapPrevSize)
}

// parseSize parses the width and height of the map preview size string.
func parseSize(prevSize string) (width, height int) {
	// Default value:
	width, height = 500, 500
	if prevSize == "" {
		return
	}

	parts := strings.Split(prevSize, "x")
	if len(parts) != 2 {
		return // Should never happen...
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return // Should never happen...
	}
	height, err = strconv.Atoi(parts[1])
	if err != nil {
		return // Should never happen...
	}

	return
}

// GetMobMapImgFormat returns the Mobile Map image format.
func (a *Account) GetMobMapImgFormat() string {
	// Default value:
	if a.MobMapImgFormat == "" {
		return "jpg"
	}
	return a.MobMapImgFormat
}

// GetKey returns a newly created complete Key of the Account.
func (a *Account) GetKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, ENameAccount, "", a.KeyID, nil)
}

//

// A cache of loaded locations to avoid loading them each time
var nameLocationMap = make(map[string]*time.Location)

// Mutex used to synchronize access to the nameLocationMap via the getCachedLocation() function.
var namelocmapMutex sync.Mutex

// getCachedLocation returns a Location for the specified name.
// Loaded locations are cached in the nameLocationMap map and returned if queried again.
func getCachedLocation(name string) (*time.Location, error) {
	// Synchronize access because the cache-map is shared!
	namelocmapMutex.Lock()
	defer func() {
		namelocmapMutex.Unlock()
	}()

	if loc, ok := nameLocationMap[name]; ok {
		return loc, nil
	}

	loc, err := time.LoadLocation(name)
	if err == nil {
		nameLocationMap[name] = loc
	}
	return loc, err
}
