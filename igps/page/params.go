/*
Params struct which is the basic wrapper for all parameters available for templates are defined here.
*/

package page

import (
	"appengine"
	"appengine/user"
	"html/template"
	"igps/cache"
	"igps/ds"
	"net/http"
	"strings"
	"time"
)

// Type of the value passed when executing templates.
type Params struct {
	// Slice of all pages (for menu/navigation)
	Pages []*Page

	// Map of pages, mapped from Path
	PathPageMap map[string]*Page

	// Map of pages, mapped from Name
	NamePageMap map[string]*Page

	// The http request
	Request *http.Request

	// Tells if Mobile client is detected and Mobile variant should be rendered
	Mobile bool

	// Page being rendered
	Page *Page

	// Render start time: creation time of the Params
	Start time.Time

	// AppEngine context
	AppCtx appengine.Context

	// Error while creating Params or executing page Logic
	Err error

	// Logged in user
	User *user.User

	// Login URL if there is no logged in user
	LoginURL string

	// Logout URL if a user is logged in
	LogoutURL string

	// Account of the logged in user
	Account *ds.Account

	// Optional info message, if present, will be rendered at the top of the page content.
	InfoMsg interface{}

	// Optional error message, if present, will be rendered at the top of the page content.
	ErrorMsg interface{}

	// Optional important message, if present, will be rendered at the top of the page content.
	ImportantMsg interface{}

	// Page related custom data
	Custom map[string]interface{}
}

// NewParams returns a new initialized Params
func NewParams(r *http.Request, page *Page) *Params {
	now := time.Now()
	c := appengine.NewContext(r)

	p := Params{Pages: Pages, PathPageMap: PathPageMap, NamePageMap: NamePageMap,
		Request: r, Mobile: isMobile(r), Page: page, Start: now, AppCtx: c,
		Custom: make(map[string]interface{})}

	p.User = user.Current(c)

	if p.User == nil {
		p.LoginURL, p.Err = user.LoginURL(c, r.URL.String())
	} else {
		p.LogoutURL, p.Err = user.LogoutURL(c, r.URL.String())
		if p.Err != nil {
			// Log if error, but this is not a show-stopper:
			c.Errorf("Error getting logout URL: %s", p.Err.Error())
		}
		p.Account, p.Err = cache.GetAccount(p.Request, c, p.User)
	}
	if p.Err != nil {
		c.Errorf("ERROR: %s", p.Err.Error())
	}

	return &p
}

// isMobile tells if the client is a mobile.
// Currently it only detects Android OS as mobile client.
func isMobile(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "Android")
}

// RenderDuration returns the render duration (elapsed time since the creation of the Params),
// truncated to milliseconds.
func (p *Params) RenderDuration() time.Duration {
	return time.Since(p.Start) / time.Millisecond * time.Millisecond
}

// FormatDateTime formats the specified date+time using the format "2006-01-02 15:04:05"
// This is a method of Params because it uses the Account setting to format the date/time in the user's location/timezone.
func (p *Params) FormatDateTime(t time.Time) template.HTML {
	if p.Account != nil {
		t = t.In(p.Account.Location())
	}
	return template.HTML(t.Format(`<span class="datePart">06-01-02</span> 15:04:05`))
}

// ParseTime parses a formatting string and returns the time value it represents.
// This is a method of Params because it uses the Account setting to parse the date/time in the user's location/timezone.
func (p *Params) ParseTime(layout, value string) (time.Time, error) {
	if p.Account != nil {
		return time.ParseInLocation(layout, value, p.Account.Location())
	}
	return time.Parse(layout, value)
}
