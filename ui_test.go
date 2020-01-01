package main

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailableFields(t *testing.T) {

	result, err := availableFields()
	if err != nil {
		t.Fatal(err.Error())
	}

	assert.Equal(t, "_search", result[" Text Search"])
	assert.Equal(t, "app", result["Application List"])
	assert.Equal(t, "datacenter", result["Datacenter"])
	assert.Equal(t, "description", result["Description"])
	assert.Equal(t, "env", result["Environment"])
	assert.Equal(t, "flavor", result["Openstack Flavor"])
	assert.Equal(t, "flavor_id", result["Openstack Flavor ID"])
	assert.Equal(t, "hostname", result["Hostname"])
	assert.Equal(t, "image", result["Openstack Image"])
	assert.Equal(t, "ip", result["IP Address"])
	assert.Equal(t, "owner", result["Owner List"])
	assert.Equal(t, "puppet", result["Puppet Class List"])
	assert.Equal(t, "status", result["Status"])
	assert.Equal(t, "tenant_id", result["Openstack Tenant ID"])
	assert.Equal(t, "type", result["Type"])
	assert.Equal(t, "vc_name", result["vCenter Name"])
	assert.Equal(t, "vc_url", result["vCenter URL"])

}

func TestDisplayUI(t *testing.T) {

	w := makeTestGetRequest(t,
		"/",
		false,
		nil,
	)

	assert.Regexp(t, "<title>Infrastructure Metadata</title>", w.Body, "UI page title")

	for key, values := range w.Header() {
		if key == "Content-Type" {
			assert.Equal(t, "text/html; charset=utf-8", values[0], "UI Content-Type")
		}
	}
}

func TestGetDisplayText(t *testing.T) {

	testMap := map[string]string{
		"app":               "Application List",
		"aws-account-alias": "AWS Account Alias",
		"aws-account-id":    "AWS Account ID",
		"aws-account-name":  "AWS Account Name",
		"aws-region":        "AWS Region",
		"datacenter":        "Datacenter",
		"description":       "Description",
		"env":               "Environment",
		"flavor":            "Openstack Flavor",
		"flavor_id":         "Openstack Flavor ID",
		"hostname":          "Hostname",
		"image":             "Openstack Image",
		"id":                "ID",
		"ip":                "IP Address",
		"owner":             "Owner List",
		"puppet":            "Puppet Class List",
		"stack":             "Openstack Instance",
		"stack_name":        "Openstack Instance",
		"status":            "Status",
		"tenant":            "Openstack Tenant Name",
		"tenant_name":       "Openstack Tenant Name",
		"tenant_id":         "Openstack Tenant ID",
		"type":              "Type",
		"vc_name":           "vCenter Name",
		"vc_url":            "vCenter URL",
		"_search":           "Text Search",
	}

	for key, text := range testMap {
		assert.Equal(t, text, getDisplayText(key))
	}

}

func TestRenderPagination(t *testing.T) {

	limit := 3

	// populate with records
	header, lines, err := renderData(generateTestRecords(limit))
	if err != nil {
		t.Fatal(err.Error())
	}

	test := displayResults{
		Count:  37,
		Limit:  limit,
		Offset: 15,
		Query: map[string][]string{
			"test":    {"yes"},
			"_offset": {"15"},
		},
		Header: header,
		Lines:  lines,
	}

	html, err := renderPagination(test)
	if err != nil {
		t.Fatal(err.Error())
	}

	assert.Equal(t, template.HTML(`
      <div class="row">
        <div class="col-sm-12 ta-sm-c pb4-sm">
          <nav aria-label="Page navigation" style="display: inline-block;">
            <ul class="pagination">

              <li class="page-item">
                <a class="page-link" href="/?_offset=12&test=yes" aria-label="Previous">
                  <span aria-hidden="true">&laquo;</span>
                  <span class="sr-only">Previous</span>
                </a>
              </li>

			  <li class="page-item">
			    <a class="page-link" href="/?_offset=0&test=yes">1</a>
			  </li>
			  <li class="page-item disabled">
			    <a class="page-link" href="#">...</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=9&test=yes">4</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=12&test=yes">5</a>
			  </li>
			  <li class="page-item active">
			    <a class="page-link" href="/?_offset=15&test=yes">6</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=18&test=yes">7</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=21&test=yes">8</a>
			  </li>
			  <li class="page-item disabled">
			    <a class="page-link" href="#">...</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=36&test=yes">13</a>
			  </li>
              <li class="page-item">
                <a class="page-link" href="/?_offset=18&test=yes" aria-label="Next">
                  <span aria-hidden="true">&raquo;</span>
                  <span class="sr-only">Next</span>
                </a>
              </li>

            </ul>
          </nav>
        </div>
      </div>
`), html)

	// test regex escaping
	regexTest := displayResults{
		Count:  37,
		Limit:  limit,
		Offset: 15,
		Query: map[string][]string{
			"test":    {"/y/"},
			"_offset": {"15"},
		},
		Header: header,
		Lines:  lines,
	}
	regexHTML, err := renderPagination(regexTest)
	if err != nil {
		t.Fatal(err.Error())
	}

	assert.Equal(t, template.HTML(`
      <div class="row">
        <div class="col-sm-12 ta-sm-c pb4-sm">
          <nav aria-label="Page navigation" style="display: inline-block;">
            <ul class="pagination">

              <li class="page-item">
                <a class="page-link" href="/?_offset=12&test=%2Fy%2F" aria-label="Previous">
                  <span aria-hidden="true">&laquo;</span>
                  <span class="sr-only">Previous</span>
                </a>
              </li>

			  <li class="page-item">
			    <a class="page-link" href="/?_offset=0&test=%2Fy%2F">1</a>
			  </li>
			  <li class="page-item disabled">
			    <a class="page-link" href="#">...</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=9&test=%2Fy%2F">4</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=12&test=%2Fy%2F">5</a>
			  </li>
			  <li class="page-item active">
			    <a class="page-link" href="/?_offset=15&test=%2Fy%2F">6</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=18&test=%2Fy%2F">7</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=21&test=%2Fy%2F">8</a>
			  </li>
			  <li class="page-item disabled">
			    <a class="page-link" href="#">...</a>
			  </li>
			  <li class="page-item">
			    <a class="page-link" href="/?_offset=36&test=%2Fy%2F">13</a>
			  </li>
              <li class="page-item">
                <a class="page-link" href="/?_offset=18&test=%2Fy%2F" aria-label="Next">
                  <span aria-hidden="true">&raquo;</span>
                  <span class="sr-only">Next</span>
                </a>
              </li>

            </ul>
          </nav>
        </div>
      </div>
`), regexHTML)

}

func TestRenderQuery(t *testing.T) {

	test := map[string][]string{
		"test":   {"yes"},
		"number": {"9"},
		"list":   {"one", "two", "three"},
	}

	str := renderQuery(test)

	assert.Equal(t, template.URL("list=%5Bone%2Ctwo%2Cthree%5D&number=9&test=yes"), str)

}
