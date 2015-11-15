var prevRow, prevRowClass;

/**
 * Highlights the table row TR which contains the specified element (which may
 * be the TR itself).
 * 
 * When called multiple times, the row highlighted previously will be "restored"
 * so this function only highlights 1 row at the most.
 */
function highlightRow(el) {
	if (prevRow)
		prevRow.className = prevRowClass;

	while (true) {
		if (el == null)
			return;
		if (el.tagName == "TR")
			break;
		el = el.parentNode;
	}

	prevRow = el;
	prevRowClass = el.className;

	el.className = "highlight";
}

/**
 * Registers an onkeypress function at the specified HTML element/tag to call the specified fv function
 * if enter is pressed.
 */
function registerEnter(tag, fv) {
	tag.onkeypress = function(e) {
		if (!e)
			e = window.event;
		var keyCode = e.keyCode || e.which;
		if (keyCode == '13') {
			fv();
		}
		return true;
	}
}

/**
 * Converts the specified HTML code to text by stripping off the HTML tags and formatting.
 */
function htmlToText(html) {
	var temp = document.createElement("div");
	temp.innerHTML = html;
	return temp.innerText;
}
