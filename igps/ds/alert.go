/*
Defines the Alert type.
*/

package ds

import (
	"encoding/json"
	"time"
)

// Name of the Datastore Alert entity
const ENameAlert = "Alr"

// Alert type.
type Alert struct {
	// Car Device key ID.
	CarDevID int64 `datastore:"cdid"`

	// Personal Mobile Device key ID.
	PersMobDevID int64 `datastore:"pdid"`

	// Timestamp
	Created time.Time `datastore:"t"`

	// ------------------------------------------------------------------------------
	// Derived/computed fields

	// ID field of the Alert's key.
	KeyID int64 `datastore:"-"`

	// Name of the car device.
	CarDevName string `datastore:"-"`

	// Name of the personal mobile device.
	PersMobDevName string `datastore:"-"`
}

// Encode encodes the Alert into a []byte using JSON.
func (a *Alert) Encode() []byte {
	b, err := json.Marshal(a) // This can't really fail...
	if err != nil {
		panic(err)
	}
	return b
}

// Decode decodes the Alert from a JSON []byte.
func (a *Alert) Decode(b []byte) {
	err := json.Unmarshal(b, a)
	if err != nil {
		panic(err)
	}
}
