/*
Defines the GPS device type.
*/

package ds

import (
	"encoding/json"
	"time"
	"fmt"
)

// Name of the Datastore Device entity
const ENameDevice = "Dev"

// Device type.
type Device struct {
	// Device name
	Name string `datastore:"nm" json:"nm"`

	// AreaSize for location searches in meters. 0 means not indexed (not searchable).
	AreaSize int64 `datastore:"as" json:"as"`

	// Number of days to keep GPS log records for this device. 0 means forever (never to delete them).
	LogsRetention int `datastore:"lr" json:"lr"`

	// Random Unique Device ID: expected in incoming URLs to identify the device.
	// Not permanent, can be changed (regenerated).
	RandID string `datastore:"rid" json:"-"`

	// Timestamp
	Created time.Time `datastore:"t" json:"-"`

	// ------------------------------------------------------------------------------
	// Derived/computed fields

	// ID field of the Device's key.
	KeyID int64 `datastore:"-"`
}

// Indexed tells if GPS records o this device are indexed and can be searched by location.
func (d *Device) Indexed() bool {
	return d.AreaSize > 0
}

// SearchPrecision returns the search precision in meters.
func (d *Device) SearchPrecision() int64 {
	return d.AreaSize / 2
}

// SetSearchPrecision sets the search precision in meters.
func (d *Device) SetSearchPrecision(sp int64) {
	d.AreaSize = sp * 2
}

// DelOldLogs tells if old GSP records (beyond the LogsRetention) are not to be kept (not important).
func (d *Device) DelOldLogs() bool {
	return d.LogsRetention > 0
}

// SearchPrecisionString returns the Search Precision in a human readable format.
func (d *Device) SearchPrecisionString() string {
	if d.Indexed() {
		return fmt.Sprintf("%d m", d.SearchPrecision())
	}
	return "not indexed"
}

// LogsRetentionString returns the Logs Retention in a human readable format.
func (d *Device) LogsRetentionString() string {
	if d.DelOldLogs() {
		var plural string 
		if d.LogsRetention != 1 {
			plural = "s"
		}  
		return fmt.Sprintf("%d day%s", d.LogsRetention, plural)
	}
	return "forever"
}

// Encode encodes the Device into a []byte using JSON.
func (d *Device) Encode() []byte {
	b, err := json.Marshal(d) // This can't really fail...
	if err != nil {
		panic(err)
	}
	return b
}

// Decode decodes the Device from a JSON []byte.
func (d *Device) Decode(b []byte) {
	err := json.Unmarshal(b, d)
	if err != nil {
		panic(err)
	}
}
