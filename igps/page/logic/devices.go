/*
Devices page logic.
*/

package logic

import (
	"appengine/datastore"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"igps/cache"
	"igps/ds"
	"igps/page"
	"strconv"
	"time"
)

func init() {
	page.NamePageMap["Devices"].Logic = devices
}

// devices is the logic implementation of the Devices page.
func devices(p *page.Params) {
	c := p.AppCtx
	fv := p.Request.PostFormValue

	// Initial values:
	p.Custom["SearchPrecision"] = 1000
	p.Custom["LogsRetention"] = 60

	// Detect form submits:
	switch {
	case fv("submitAdd") != "":
		// Add New Device form submitted!
		// Checks:
		switch {
		case !checkName(p, fv("name")):
		case !checkSearchPrecision(p, fv("searchPrecision")):
		case !checkLogsRetention(p, fv("logsRetention")):
		}
		if p.ErrorMsg == nil {
			// All data OK, save new Device
			searchPrecision, _ := strconv.ParseInt(fv("searchPrecision"), 10, 64)
			logsRetention, _ := strconv.Atoi(fv("logsRetention"))
			dev := ds.Device{fv("name"), 0, logsRetention, "", time.Now(), 0}
			dev.SetSearchPrecision(searchPrecision)
			genNewRandID(p, &dev)
			if p.Err != nil {
				return
			}
			if _, p.Err = datastore.Put(c, datastore.NewIncompleteKey(c, ds.ENameDevice, p.Account.GetKey(c)), &dev); p.Err != nil {
				return // Datastore error
			}
			p.InfoMsg = "New Device saved successfully."
			// Clear from memcache:
			cache.ClearDevListForAccKey(c, p.Account.GetKey(c))
		} else {
			// Submitted values
			p.Custom["Name"] = fv("name")
			p.Custom["SearchPrecision"] = fv("searchPrecision")
			p.Custom["LogsRetention"] = fv("logsRetention")
		}
	case fv("submitRename") != "":
		// Rename Device form submitted!
		if !checkName(p, fv("name")) {
			break
		}
		if devID, err := strconv.ParseInt(string(fv("devID")), 10, 64); err != nil {
			p.ErrorMsg = "Invalid Device!"
		} else {
			devKey := datastore.NewKey(c, ds.ENameDevice, "", devID, p.Account.GetKey(c))
			var dev ds.Device
			if err = datastore.Get(c, devKey, &dev); err != nil {
				if err == datastore.ErrNoSuchEntity {
					p.ErrorMsg = "You do not have access to the specified Device!"
				} else {
					// Real datastore error
					p.Err = err
					return
				}
			} else {
				// Proceed to rename
				dev.Name = fv("name")
				if _, p.Err = datastore.Put(c, devKey, &dev); p.Err != nil {
					return // Datastore error
				}
				p.InfoMsg = "Device renamed successfully."
				// Clear from memcache:
				cache.ClearDevListForAccKey(c, p.Account.GetKey(c))
				cache.ClearDeviceForRandID(c, dev.RandID)
				dev.KeyID = devID // This is important (device is loaded freshly and not yet set)!
				cache.CacheDevice(c, &dev)
			}
		}
	case fv("submitGenNewKey") != "":
		// Generate New Key form submitted!
		if devID, err := strconv.ParseInt(string(fv("devID")), 10, 64); err != nil {
			p.ErrorMsg = "Invalid Device!"
		} else {
			devKey := datastore.NewKey(c, ds.ENameDevice, "", devID, p.Account.GetKey(c))
			var dev ds.Device
			if err = datastore.Get(c, devKey, &dev); err != nil {
				if err == datastore.ErrNoSuchEntity {
					p.ErrorMsg = "You do not have access to the specified Device!"
				} else {
					// Real datastore error
					p.Err = err
					return
				}
			} else {
				// Proceed to generate new key
				// Store old RandID to remove it from cache if saving succeeds
				oldRandID := dev.RandID
				genNewRandID(p, &dev)
				if p.Err != nil {
					return
				}
				if _, p.Err = datastore.Put(c, devKey, &dev); p.Err != nil {
					return // Datastore error
				}
				p.InfoMsg = template.HTML("New Key generated successfully.")
				p.ImportantMsg = template.HTML("<b>Important!</b> You have to update the URL in the client application else further GPS tracking calls will be discarded!")
				cache.ClearDeviceForRandID(c, oldRandID)
				dev.KeyID = devID // This is important (device is loaded freshly and not yet set)!
				cache.CacheDevice(c, &dev)
			}
		}
	}

	q := datastore.NewQuery(ds.ENameDevice).Ancestor(p.Account.GetKey(c)).Order(ds.PNameName)

	var devices []*ds.Device
	var devKeys []*datastore.Key
	if devKeys, p.Err = q.GetAll(c, &devices); p.Err != nil {
		return
	}
	for i := range devices {
		devices[i].KeyID = devKeys[i].IntID()
	}

	p.Custom["Devices"] = devices
}

// checkName checks the specified Device name and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid).
func checkName(p *page.Params, name string) (ok bool) {
	switch {
	case len(name) == 0:
		p.ErrorMsg = template.HTML(`<span class="code">Name</span> must be specified!`)
		return false
	case len(name) > 500:
		p.ErrorMsg = template.HTML(`<span class="code">Name</span> is too long! (cannot be longer than 500 characters)`)
		return false
	}

	return true
}

// checkSearchPrecision checks the specified search precision and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid).
func checkSearchPrecision(p *page.Params, searchPrecision string) (ok bool) {
	basemsg := `Invalid <span class="code">Search Precision</span>!`

	num, err := strconv.ParseInt(searchPrecision, 10, 64)
	if err != nil {
		p.ErrorMsg = template.HTML(basemsg)
		return false
	}
	if num < 0 || num > 1000*1000 {
		p.ErrorMsg = SExecTempl(basemsg+` Value is outside of valid range (0..1,000,000): <span class="highlight">{{.}}</span>`, num)
		return false
	}
	if num%100 != 0 {
		p.ErrorMsg = SExecTempl(basemsg+` Value is not a multiple of 100: <span class="highlight">{{.}}</span>`, num)
		return false
	}

	return true
}

// checkLogsRetention checks the specified logs retention and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid).
func checkLogsRetention(p *page.Params, logsRetention string) (ok bool) {
	basemsg := `Invalid <span class="code">Logs Retention</span>!`

	num, err := strconv.Atoi(logsRetention)
	if err != nil {
		p.ErrorMsg = template.HTML(basemsg)
		return false
	}
	if num < 0 || num > 9999 {
		p.ErrorMsg = SExecTempl(basemsg+` Value is outside of valid range (0..9,999): <span class="highlight">{{.}}</span>`, num)
		return false
	}

	return true
}

// genNewID generates a new, unique device ID
func genNewRandID(p *page.Params, d *ds.Device) {
	b := make([]byte, 9) // Use 9 bytes: multiple of 3 bytes (ideal for base64 encoding so no padding '=' signs will be needed)
	if _, p.Err = rand.Read(b); p.Err != nil {
		return
	}

	RandID := base64.URLEncoding.EncodeToString(b)

	// Check if RandID is unique.
	// It will be, but once in a million years we might maybe perhaps encounter a match!
	q := datastore.NewQuery(ds.ENameDevice).Filter(ds.PNameRandID+"=", RandID).KeysOnly().Limit(1)
	var devKeys []*datastore.Key
	if devKeys, p.Err = q.GetAll(p.AppCtx, nil); p.Err != nil {
		return
	}
	if len(devKeys) > 0 {
		p.Err = fmt.Errorf("Generated device RandID already exists: %s", RandID)
		return
	}

	d.RandID = RandID
}
