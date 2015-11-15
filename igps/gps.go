/*
Defines the GPS record type and a handler which processes incoming GPS tracking events.
*/

package igps

import (
	"appengine"
	"appengine/datastore"
	"igps/cache"
	"igps/ds"
	"igps/page/logic"
	"net/http"
	"strconv"
	"time"
)

func init() {
	http.HandleFunc("/gps", gpsHandler)
}

// gpsHandler is the handler of the requests originating from (GPS) clients
// reporting GPS coordinates, start/stop events.
func gpsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	// General logs for all requests
	c.Debugf("Location: %s;%s;%s;%s", r.Header.Get("X-AppEngine-Country"), r.Header.Get("X-AppEngine-Region"), r.Header.Get("X-AppEngine-City"), r.Header.Get("X-AppEngine-CityLatLong"))

	// If device id is invalid, we want to return sliently and not let the client know about it.
	// So first check non-user related params first, because else if the client would intentionally
	// provide an invalid param and get no error, he/she would know that the device id is invalid.

	RandID := r.FormValue("dev")
	if RandID == "" {
		c.Errorf("Missing Device ID (dev) parameter!")
		http.Error(w, "Missing Device ID (dev) parameter!", http.StatusBadRequest)
		return
	}

	// Do a check for RandID length: it is used to construct a memcache key which has a 250 bytes limit!
	if len(RandID) > 100 {
		c.Errorf("Invalid Device ID (dev) parameter!")
		http.Error(w, "Invalid Device ID (dev) parameter!", http.StatusBadRequest)
		return
	}

	gps := ds.GPS{Created: time.Now()}

	var err error

	tracker := r.FormValue("tracker")
	if tracker != "" {
		// Start-stop event
		switch tracker {
		case "start":
			gps.AreaCodes = []int64{int64(ds.EvtStart)}
		case "stop":
			gps.AreaCodes = []int64{int64(ds.EvtStop)}
		default:
			c.Errorf("Invalid tracker parameter!")
			http.Error(w, "Invalid tracker parameter!", http.StatusBadRequest)
			return
		}
	} else {
		// GPS coordinates; lat must be in range -90..90, lng must be in range -180..180
		gps.GeoPoint.Lat, err = strconv.ParseFloat(r.FormValue("lat"), 64)
		if err != nil {
			c.Errorf("Missing or invalid latitude (lat) parameter!")
			http.Error(w, "Missing or invalid latitude (lat) parameter!", http.StatusBadRequest)
			return
		}
		gps.GeoPoint.Lng, err = strconv.ParseFloat(r.FormValue("lon"), 64)
		if err != nil {
			c.Errorf("Missing or invalid longitude (lon) parameter!")
			http.Error(w, "Missing or invalid longitude (lon) parameter!", http.StatusBadRequest)
			return
		}
		if !gps.GeoPoint.Valid() {
			c.Errorf("Invalid geopoint specified by latitude (lat) and longitude (lon) parameters (valid range: [-90, 90] latitude and [-180, 180] longitude)!")
			http.Error(w, "Invalid geopoint specified by latitude (lat) and longitude (lon) parameters (valid range: [-90, 90] latitude and [-180, 180] longitude)!", http.StatusBadRequest)
			return
		}
	}

	var dev *ds.Device

	dev, err = cache.GetDevice(c, RandID)
	if err != nil {
		c.Errorf("Failed to get Device with cache.GetDevice(): %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if dev == nil {
		// Invalid RandID. Do nothing and return silently.
		return
	}
	gps.DevKeyID = dev.KeyID

	if tracker == "" && dev.Indexed() {
		gps.AreaCodes = logic.AreaCodesForGeoPt(dev.AreaSize, gps.GeoPoint.Lat, gps.GeoPoint.Lng)
	}

	var gpsKey *datastore.Key

	if dev.DelOldLogs() {
		// There is a (positive) Logs Retention for the device.
		// We could delete old records, but that costs us a lot of Datastore write ops
		// (exactly the same as saving a new record).
		// Instead I query for old records (beyond the Logs Retention),
		// and "resave" to those records (basically save a new record with an existing key).
		// This way no deletion has to be paid for.
		// Optimal solution would be to pick the oldest record, but that would require a new index (G: d, t)
		// which is a "waste", so I just use the existing index (G: d, -t). This will result in the first record
		// that is just over the Retention period.
		// Load the existing record by key (strong consistency) and check if its timestamp is truely
		// beyond the retention (because a concurrent resave might have happened).
		// Query more than 1 record (limit>1) because if the latest is just concurrently resaved,
		// we have more records without executing another query. And small operations are free (reading keys).
		t := time.Now().Add(-24 * time.Hour * time.Duration(dev.LogsRetention))
		q := datastore.NewQuery(ds.ENameGPS).
			Filter(ds.PNameDevKeyID+"=", dev.KeyID).
			Filter(ds.PNameCreated+"<", t).
			Order("-" + ds.PNameCreated).KeysOnly().Limit(7)
		keys, err := q.GetAll(c, nil)
		if err != nil {
			c.Errorf("Failed to list GPS records beyond Retention period: %v", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		for _, key := range keys {
			// Load by key (strongly consistent)
			var oldGps ds.GPS
			if err = datastore.Get(c, key, &oldGps); err != nil {
				c.Errorf("Failed to load GPS by Key: %v", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			if t.After(oldGps.Created) {
				// Good: it is still older (not resaved concurrently). We will use this!
				gpsKey = key
				break
			}
		}
	}

	if gpsKey == nil {
		// No current record to resave, create a new incomplete key for a new record:
		gpsKey = datastore.NewIncompleteKey(c, ds.ENameGPS, nil)
	}

	_, err = datastore.Put(c, gpsKey, &gps)
	if err != nil {
		c.Errorf("Failed to store GPS record: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}
