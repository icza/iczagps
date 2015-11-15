/*
Page struct and list of pages are defiend here.
Also the handler serving all the pages are here.
*/

package page

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"
)

const (
	NO_LOGIN  = false
	REQ_LOGIN = true
)

const (
	NOT_VISIBLE = false
	VISIBLE     = true
)

const (
	NOT_ERROR = false
	IS_ERROR  = true
)

// Type of the value passed when executing templates.
type Page struct {
	// Unique name of the page which can be used to refer to this page by.
	Name string

	// Path of the page under which it is accessible
	Path string

	// Title of the Page
	Title string

	// Tells if this page requires a logged in user
	ReqLogin bool

	// Page logic function
	Logic func(params *Params)

	// Name of the page template
	TemplName string

	// Tells if the page should be visible in the menu
	Visible bool

	// Tells if this page is an error page
	Error bool
}

// Link returns an HTML link (<a>) pointing to the page.
func (p *Page) Link() template.HTML {
	return template.HTML(fmt.Sprintf(`<a href="%s">%s</a>`, p.Path, html.EscapeString(p.Title)))
}

// Slice of all pages
var Pages = []*Page{
	// "Normal" pages
	&Page{"Home", "/", "Home", NO_LOGIN, nil, "home.html", VISIBLE, NOT_ERROR},
	&Page{"Devices", "/devices", "Devices", REQ_LOGIN, nil, "devices.html", VISIBLE, NOT_ERROR},
	&Page{"Logs", "/logs", "Logs", REQ_LOGIN, nil, "logs.html", VISIBLE, NOT_ERROR},
	&Page{"Alerts", "/alerts", "Alerts", REQ_LOGIN, nil, "alerts.html", VISIBLE, NOT_ERROR},
	&Page{"Settings", "/settings", "Settings", REQ_LOGIN, nil, "settings.html", VISIBLE, NOT_ERROR},
	&Page{"TermsAndPolicy", "/termsandpolicy", "Terms and Policy", NO_LOGIN, nil, "terms_and_policy.html", VISIBLE, NOT_ERROR},
	&Page{"Register", "/register", "Register", NO_LOGIN, nil, "register.html", NOT_VISIBLE, NOT_ERROR},

	// Error pages
	&Page{"NotFound", "/notfound", "Page Not Found :-(", NO_LOGIN, nil, "err_not_found.html", NOT_VISIBLE, IS_ERROR},
	&Page{"NotLoggedIn", "/notloggedin", "Not Logged In", NO_LOGIN, nil, "err_not_logged_in.html", NOT_VISIBLE, IS_ERROR},
	&Page{"NoAccount", "/noaccount", "No IczaGPS Account", NO_LOGIN, nil, "err_no_account.html", NOT_VISIBLE, IS_ERROR},
	&Page{"InternalError", "/internalerror", "Internal Error %#$@~", NO_LOGIN, nil, "err_internal.html", NOT_VISIBLE, IS_ERROR},
}

// Map of pages, mapped from Path
var PathPageMap map[string]*Page = make(map[string]*Page, len(Pages))

// Map of pages, mapped from Name
var NamePageMap map[string]*Page = make(map[string]*Page, len(Pages))

func init() {
	for _, page := range Pages {
		// First some checks:
		if _, ok := PathPageMap[page.Path]; ok {
			panic("Page Path is not unique!")
		}
		if _, ok := NamePageMap[page.Name]; ok {
			panic("Page Name is not unique!")
		}

		PathPageMap[page.Path] = page
		NamePageMap[page.Name] = page
	}

	http.HandleFunc("/", pageHandler)
}

// pageHandler is a handler mapped to the "/" pattern and it serves all pages
func pageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)

	var p *Params
	{
		// Create an inner block so the page variable will be block-scoped.
		// I do this because page may change (in case of error) so I don't accidentally refer to this initial page.
		page := PathPageMap[r.URL.Path]
		if page == nil {
			page = NamePageMap["NotFound"]
			w.WriteHeader(http.StatusNotFound)
		}
		p = NewParams(r, page)
	}

	c := p.AppCtx

	// General logs for all requests
	c.Debugf("Location: %s;%s;%s;%s", r.Header.Get("X-AppEngine-Country"), r.Header.Get("X-AppEngine-Region"), r.Header.Get("X-AppEngine-City"), r.Header.Get("X-AppEngine-CityLatLong"))
	// Form parameters
	r.ParseForm()
	if len(r.Form) > 0 {
		c.Debugf("Form: %v", r.Form) // Request.Form contains both URL and Post form parameters
	}

	if p.Err == nil && p.Page.ReqLogin {
		// Login required
		if p.User == nil {
			p.Page = NamePageMap["NotLoggedIn"]
			w.WriteHeader(http.StatusUnauthorized)
			Htmls.ExecuteTemplate(w, p.Page.TemplName, p)
			return
		}
		// A user is logged in, check account
		if p.Account == nil {
			p.Page = NamePageMap["NoAccount"]
			w.WriteHeader(http.StatusForbidden)
			Htmls.ExecuteTemplate(w, p.Page.TemplName, p)
			return
		}
	}

	if p.Err == nil && p.Page.Logic != nil {
		// Run page logic protected so we can serve our nice InternalError page in case the logic panics:
		runLogic(p)
	}

	if p.Err != nil {
		c.Errorf("%s", p.Err)
		p.Page = NamePageMap["InternalError"]
		w.WriteHeader(http.StatusInternalServerError)
		Htmls.ExecuteTemplate(w, p.Page.TemplName, p)
		return
	}

	// Log page logic response messages
	if p.InfoMsg != nil {
		c.Debugf("Params.InfoMsg: %v", p.InfoMsg)
	}
	if p.ErrorMsg != nil {
		c.Debugf("Params.ErrorMsg: %v", p.ErrorMsg)
	}
	if p.ImportantMsg != nil {
		c.Debugf("Params.ImportantMsg: %v", p.ImportantMsg)
	}

	Htmls.ExecuteTemplate(w, p.Page.TemplName, p)
}

// runLogic executes the page logic function protected: it catches panic calls
// and restores normal operation in which case stores an error into Params.Err.
func runLogic(p *Params) {
	defer func() {
		if x := recover(); x != nil {
			p.AppCtx.Errorf("%s: %s", x, debug.Stack())
			p.Err = fmt.Errorf("Page Logic paniced: %s", x)
		}
	}()

	p.Page.Logic(p)
}
