package main

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pdxfixit/hostdb"
)

type details struct {
	DisplayText string
	QueryParam  string
}

type displayResults struct {
	Count  int
	Limit  int
	Offset int
	Query  map[string][]string
	Header []string
	Lines  []map[string]string
}

type vropsResourceProperties struct {
	ResourceID string          `json:"resourceId"`
	Property   []vropsProperty `json:"property"`
}

type vropsProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func availableFields() (fields map[string]string, err error) {

	fields = map[string]string{
		" Text Search": "_search",
	}

	for paramName := range config.API.V0.QueryParams {
		if paramName == "id" || paramName == "test" {
			continue
		}

		text := getDisplayText(paramName)

		fields[text] = paramName
	}

	return fields, nil

}

func displayUI(c *gin.Context) {

	// check for any query params
	query := c.Request.URL.Query()

	// if no query parameters, return the search form
	if len(query) == 0 {
		c.HTML(http.StatusOK, "search.html", nil)
		return
	}

	// set the default number of records per page
	if len(query["_limit"]) == 0 {
		query["_limit"] = append(query["_limit"], fmt.Sprintf("%d", config.API.V0.DefaultLimit))
	}

	// start processing query params
	records, foundRows, err := processQueryParams(query)
	if err != nil {
		if err, ok := err.(hostdb.ErrorResponse); ok {
			c.HTML(err.Code, "error.html", err.Message)
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		}
		return
	}

	limit := 0
	if len(query["_limit"]) > 0 {
		limit, err = strconv.Atoi(query["_limit"][0])
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", err.Error())
			return
		}
	}

	offset := 0
	if len(query["_offset"]) > 0 {
		offset, err = strconv.Atoi(query["_offset"][0])
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", err.Error())
			return
		}
	}

	// prepare a slice of the field names, to be used as a header
	header, lines, err := renderData(records)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		return
	}

	results := displayResults{
		Count:  foundRows,
		Limit:  limit,
		Offset: offset,
		Query:  query,
		Header: header,
		Lines:  lines,
	}

	c.HTML(http.StatusOK, "results.html", results)

}

func getDisplayText(key string) string {

	switch key {
	case "id":
		return "ID"
	case "type":
		return "Type"
	case "_search":
		return "Text Search"
	default:
		for _, dataStructure := range config.API.V0.QueryParams[key] {
			if dataStructure.DisplayName == "" {
				return key
			}

			return dataStructure.DisplayName // we just need one
		}
	}

	return key // shouldn't ever get here tho

}

func renderPagination(result displayResults) (html template.HTML, err error) {

	if len(result.Lines) > result.Limit {
		return template.HTML(""), nil
	}

	delta := 2
	currentPage := (result.Offset / result.Limit) + 1
	str := ""
	totalPages := int(math.Ceil(float64(result.Count) / float64(result.Limit)))
	var pageNumbers []int

	// figure out which page numbers we'll want to show
	left := result.Offset - (result.Limit * delta)
	if left < 0 {
		left = 0
	}

	right := result.Offset + (result.Limit * (delta + 1))

	for i := 1; i <= totalPages; i++ {
		if i == 1 || i == totalPages || i*result.Limit > left && i*result.Limit <= right {
			pageNumbers = append(pageNumbers, i)
		}
	}

	// STOP DELETING ME -- I'M GOOD FOR DEBUGGING
	//str = str + fmt.Sprintf(`<pre>count: %d .... pages: %d .... limit: %d .... offset: %d .... length: %d .... page: %d</pre>`, result.Count, totalPages, result.Limit, result.Offset, len(result.Records), currentPage)

	// START
	str = str + `
      <div class="row">
        <div class="col-sm-12 ta-sm-c pb4-sm">
          <nav aria-label="Page navigation" style="display: inline-block;">
            <ul class="pagination">
`

	// LEFT NAV
	leftDisabled := ""
	leftOffset := result.Offset - result.Limit
	if result.Offset <= 0 {
		leftDisabled = " disabled"
		leftOffset = 0
	}
	if leftOffset < 0 {
		leftOffset = 0
	}
	leftQuery := result.Query
	leftQuery["_offset"] = []string{fmt.Sprintf("%d", leftOffset)}
	str = str + fmt.Sprintf(`
              <li class="page-item%s">
                <a class="page-link" href="/?%s" aria-label="Previous">
                  <span aria-hidden="true">&laquo;</span>
                  <span class="sr-only">Previous</span>
                </a>
              </li>
`, leftDisabled, renderQuery(leftQuery))

	// ITEMS
	l := 0
	for _, i := range pageNumbers {
		active := ""
		offset := result.Limit * (i - 1)

		if offset == result.Offset {
			active = " active"
		}

		if l != 0 {
			if i-l == 2 {
				linkOffset := result.Limit * l
				if linkOffset == result.Offset {
					active = " active"
				}
				query := result.Query
				query["_offset"] = []string{fmt.Sprintf("%d", linkOffset)}
				str = str + fmt.Sprintf(`
			  <li class="page-item%s">
			    <a class="page-link" href="/?%s">%d</a>
			  </li>`, active, renderQuery(query), l+1)
			} else if i-l != 1 {
				str = str + `
			  <li class="page-item disabled">
			    <a class="page-link" href="#">...</a>
			  </li>`
			}
		}

		query := result.Query
		query["_offset"] = []string{fmt.Sprintf("%d", offset)}
		str = str + fmt.Sprintf(`
			  <li class="page-item%s">
			    <a class="page-link" href="/?%s">%d</a>
			  </li>`, active, renderQuery(query), i)

		l = i
	}

	// RIGHT NAV
	rightDisabled := ""
	rightOffset := result.Offset + result.Limit
	if currentPage == totalPages {
		rightDisabled = " disabled"
		rightOffset = result.Offset
	}
	if rightOffset < 0 {
		rightOffset = 0
	}
	rightQuery := result.Query
	rightQuery["_offset"] = []string{fmt.Sprintf("%d", rightOffset)}
	str = str + fmt.Sprintf(`
              <li class="page-item">
                <a class="page-link%s" href="/?%s" aria-label="Next">
                  <span aria-hidden="true">&raquo;</span>
                  <span class="sr-only">Next</span>
                </a>
              </li>
`, rightDisabled, renderQuery(rightQuery))

	// END
	str = str + `
            </ul>
          </nav>
        </div>
      </div>
`

	return template.HTML(str), nil

}

func renderQuery(query map[string][]string) template.URL {

	u := url.URL{}
	q := u.Query()

	for key, values := range query {
		value := ""

		if len(values) > 1 {
			value = fmt.Sprintf("[%s]", strings.Join(values, ","))
		} else {
			value = values[0]
		}

		q.Set(key, value)
	}

	return template.URL(q.Encode())

}
