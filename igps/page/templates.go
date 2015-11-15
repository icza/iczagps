/*
All HTML templates are parsed and initialized here.
Also custom functions for templates are defined here.
*/

package page

import (
	"html/template"
	"time"
)

// All HTML templates
var Htmls *template.Template = template.Must(template.New("base").Funcs(map[string]interface{}{
	"Odd": Odd,
	"Add": Add,
	"Now": time.Now,
}).ParseGlob("igps/page/html/*.html"))

// Odd tells if the specified number is odd (cannot be divided by 2).
func Odd(i int) bool {
	return i%2 != 0
}

// Add adds 2 numbers and reuturns the result.
func Add(i, j int) int {
	return i + j
}
