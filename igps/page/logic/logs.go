/*
Logs page logic.
*/

package logic

import (
	"appengine"
	"appengine/datastore"
	"bytes"
	"fmt"
	"html/template"
	"igps/cache"
	"igps/ds"
	"igps/page"
	"strconv"
	"strings"
	"time"
)

func init() {
	page.NamePageMap["Logs"].Logic = logs
}

const timeLayout = "06-01-02 15:04:05"

// logs is the logic implementation of the Logs page.
func logs(p *page.Params) {
	c := p.AppCtx

	// First get devices
	var devices []*ds.Device
	if devices, p.Err = cache.GetDevListForAccKey(c, p.Account.GetKey(c)); p.Err != nil {
		return
	}
	p.Custom["Devices"] = devices

	fv := p.Request.FormValue

	p.Custom["Before"] = fv("before")
	p.Custom["After"] = fv("after")
	p.Custom["SearchLoc"] = fv("loc")

	if fv("devID") == "" {
		// No device chosen yet
		return
	}

	var err error

	var devID int64
	if devID, err = strconv.ParseInt(string(fv("devID")), 10, 64); err != nil {
		p.ErrorMsg = "Invalid Device! Please select a Device from the list below."
		return
	}
	// Check if device is owned by the user:
	var dev *ds.Device
	for _, d := range devices {
		if d.KeyID == devID {
			dev = d
			p.Custom["Device"] = d
			break
		}
	}

	if dev == nil {
		p.ErrorMsg = "You do not have access to the specified Device! Please select a Device from the list below."
		return
	}

	// Parse filters:
	var before time.Time
	if fv("before") != "" {
		if before, err = p.ParseTime(timeLayout, strings.TrimSpace(fv("before"))); err != nil {
			p.ErrorMsg = template.HTML(`Invalid <span class="highlight">Before</span>!`)
			return
		}
		// Add 1 second to the parsed time because fraction of a second is not parsed but exists,
		// so this new time will also include records which has the same time up to the second part and has millisecond part too.
		before = before.Add(time.Second)
	}
	var after time.Time
	if fv("after") != "" {
		if after, err = p.ParseTime(timeLayout, strings.TrimSpace(fv("after"))); err != nil {
			p.ErrorMsg = template.HTML(`Invalid <span class="highlight">After</span>!`)
			return
		}
	}
	var searchLoc appengine.GeoPoint
	areaCode := int64(-1)
	if dev.Indexed() && fv("loc") != "" {
		// GPS coordinates; lat must be in range -90..90, lng must be in range -180..180
		baseErr := template.HTML(`Invalid <span class="highlight">Location</span>!`)

		var coords = strings.Split(strings.TrimSpace(fv("loc")), ",")
		if len(coords) != 2 {
			p.ErrorMsg = baseErr
			return
		}
		searchLoc.Lat, err = strconv.ParseFloat(coords[0], 64)
		if err != nil {
			p.ErrorMsg = baseErr
			return
		}
		searchLoc.Lng, err = strconv.ParseFloat(coords[1], 64)
		if err != nil {
			p.ErrorMsg = baseErr
			return
		}
		if !searchLoc.Valid() {
			p.ErrorMsg = template.HTML(`Invalid <span class="highlight">Location</span> specified by latitude and longitude! Valid range: [-90, 90] latitude and [-180, 180] longitude`)
			return
		}
		areaCode = AreaCodeForGeoPt(dev.AreaSize, searchLoc.Lat, searchLoc.Lng)
	}
	var page int

	cursorsString := fv("cursors")
	var cursors = strings.Split(cursorsString, ";")[1:] // Split always returns at least 1 element (and we use semicolon separator before cursors)

	// Form values
	if fv("page") == "" {
		page = 1
	} else {
		page, err = strconv.Atoi(fv("page"))
		if err != nil || page < 1 {
			page = 1
		}
		if page > len(cursors) { // If page is provided, so are (should be) the cursors
			page = len(cursors)
		}
	}

	switch {
	case fv("submitFirstPage") != "":
		page = 1
	case fv("submitPrevPage") != "":
		if page > 1 {
			page--
		}
	case fv("submitNextPage") != "":
		page++
	}

	pageSize := p.Account.GetLogsPageSize()

	if ps := fv("pageSize"); ps != "" && ps != strconv.Itoa(pageSize) {
		// Page size has been changed (on Settings page), drop cursors, render page 1
		page = 1
		cursorsString = ""
		cursors = make([]string, 0, 1)
	}

	// 'ts all good, proceed with the query:
	q := datastore.NewQuery(ds.ENameGPS).Filter(ds.PNameDevKeyID+"=", devID)
	if !before.IsZero() {
		q = q.Filter(ds.PNameCreated+"<", before)
	}
	if !after.IsZero() {
		q = q.Filter(ds.PNameCreated+">", after)
	}
	if areaCode >= 0 {
		q = q.Filter(ds.PNameAreaCodes+"=", areaCode)
	}
	q = q.Order("-" + ds.PNameCreated).Limit(pageSize)

	var records = make([]*ds.GPS, 0, pageSize)

	// If there is a cursor, set it.
	// Page - cursor index mapping:     cursors[page-2]
	//     1st page: no cursor, 2nd page: cursors[0], 3nd page: cursors[1], ...
	if page > 1 && page <= len(cursors)+1 {
		var cursor datastore.Cursor
		if cursor, p.Err = datastore.DecodeCursor(cursors[page-2]); p.Err != nil {
			return
		}
		q = q.Start(cursor)
	}

	// Iterate over the results:
	t := q.Run(c)
	for {
		r := new(ds.GPS)
		_, err := t.Next(r)
		if err == datastore.Done {
			break
		}
		if err != nil {
			// Datastore error
			p.Err = err
			return
		}
		records = append(records, r)
		r.Dd = -1 // For now, will be set if applicable
		if r.Track() {
			// Check the previous (in time) record and calculate distance.
			// If previous is not a Track, check the one before that etc.
			for i := len(records) - 2; i >= 0; i-- {
				if prev := records[i]; prev.Track() {
					prev.Dd = Distance(r.GeoPoint.Lat, r.GeoPoint.Lng, prev.GeoPoint.Lat, prev.GeoPoint.Lng)
					prev.Dt = prev.Created.Sub(r.Created)
					break
				}
			}
		}
	}

	if len(records) == 0 {
		// End of list reached, disable Next page button:
		p.Custom["EndOfList"] = true
	}

	if page == 1 || page > len(cursors) {
		// Get updated cursor and store it for next page:
		var cursor datastore.Cursor
		if cursor, p.Err = t.Cursor(); p.Err != nil {
			return
		}
		cursorString := cursor.String()
		if page == 1 {
			// If new records were inserted, they appear on the first page in which case
			// the cursor for the 2nd page changes (and all other cursors will change).
			// In this case drop all the cursors:
			if len(cursors) > 0 && cursors[0] != cursorString {
				cursorsString = ""
				cursors = make([]string, 0, 1)
			}
		} else {
			// When end of list is reached, the same cursor will be returned
			if len(records) == 0 && page == len(cursors)+1 && cursors[page-2] == cursorString {
				// Add 1 extra, empty page, but not more.
				if page > 2 && cursors[page-3] == cursorString {
					// An extra, empty page has already been added, do not add more:
					page--
				}
			}
		}

		if page > len(cursors) {
			cursors = append(cursors, cursorString)
			cursorsString += ";" + cursorString
		}
	}

	// Calculate labels: '1'..'9' then 'A'...
	for i, lbl := len(records)-1, '1'; i >= 0; i-- {
		if r := records[i]; r.Track() {
			r.Label = lbl
			if lbl == '9' {
				lbl = 'A' - 1
			}
			lbl++
		}
	}

	p.Custom["CursorList"] = cursors
	p.Custom["Cursors"] = cursorsString

	p.Custom["Page"] = page
	p.Custom["PageSize"] = pageSize
	p.Custom["RecordOffset"] = (page-1)*pageSize + 1
	p.Custom["Records"] = records
	if p.Mobile {
		p.Custom["MapWidth"], p.Custom["MapHeight"] = p.Account.GetMobMapPrevSize()
		p.Custom["MapImgFormat"] = p.Account.GetMobMapImgFormat()
	} else {
		p.Custom["MapWidth"], p.Custom["MapHeight"] = p.Account.GetMapPrevSize()
	}
	p.Custom["APIKey"] = "AIzaSyCEU_tZ1n0-mMg4woGKIfPqdbi0leSKvjg"
	p.Custom["AllMarkers"] = allMarkers(records)

	if len(records) == 0 {
		if page == 1 {
			if before.IsZero() && after.IsZero() && areaCode < 0 {
				p.Custom["PrintNoRecordsForDev"] = true
			} else {
				p.Custom["PrintNoMatchForFilters"] = true
			}
		} else {
			p.Custom["PrintNoMoreRecords"] = true
		}
	}
}

// allMarkers returns the URL fragment containing all the markers of the specified GPS records to be appended
// to a static maps URL.
//
// Static maps documentation: // https://developers.google.com/maps/documentation/staticmaps/
func allMarkers(records []*ds.GPS) string {
	// Returned string will be around 1 KB, allocated reasonable buffer:
	b := bytes.NewBuffer(make([]byte, 0, 1280))

	// MARKERS

	var prev *ds.GPS
	for idx, r := range records {
		if !r.Track() {
			prev = r
			continue
		}

		// Determine color. Default marker color: blue;
		// First records after a Start event: green
		// Last records before a Stop event: red
		clr := "blue"
		if prev != nil && prev.Evt() == ds.EvtStop {
			clr = "red"
		}
		// Check next
		if idx < len(records)-1 && records[idx+1].Evt() == ds.EvtStart {
			clr = "green"
		}

		fmt.Fprintf(b, "&markers=color:%s|label:%c|%f,%f", clr, r.Label, r.GeoPoint.Lat, r.GeoPoint.Lng)

		prev = r
	}

	// PATHS

	i := 0
	for _, r := range records {
		if !r.Track() {
			i = 0
			continue
		}

		if i == 0 {
			b.WriteString("&path=")
		} else if i > 0 {
			b.WriteString("|")
		}
		fmt.Fprintf(b, "%f,%f", r.GeoPoint.Lat, r.GeoPoint.Lng)
		i++
	}

	return b.String()
}
