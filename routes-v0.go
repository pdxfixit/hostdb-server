package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/pdxfixit/hostdb"
)

func getAPIConfig(c *gin.Context) {

	sendResponse(c, http.StatusOK, config.API.V0)

}

// POST expects no id/name in query string
// PUT should require an id/name
func saveRecord(c *gin.Context) {

	// record
	data := hostdb.Record{
		ID: c.Param("id"),
	}

	// get the raw request data
	rawData, err := c.GetRawData()
	if err != nil {
		log.Println(fmt.Sprintf(err.Error()))
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "could not get raw request data",
		})
		return
	}

	// marshal the []bytes into our struct
	err = json.Unmarshal([]byte(rawData), &data)
	if err != nil {
		log.Println(fmt.Sprintf(err.Error()))
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "could not unmarshal the data",
		})
		return
	}

	// ensure all the necessary data is there
	if err = ensureDataIsComplete(&data); err != nil {
		log.Println(fmt.Sprintf("%v", err.Error()))
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "data is not complete",
		})
		return
	}

	// SAVE
	if err := saveMariadbRow(data); err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
				Error: "something went wrong with the database when saving the record",
			})
			return
		}

		// any other type of error
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "something went wrong saving the record",
		})
		return
	}

	sendResponse(c, http.StatusCreated, hostdb.PutRecordResponse{
		ID: data.ID,
		OK: true,
	})
	return

}

// get full detail of record(s)
func getDetail(c *gin.Context) {

	// get the records collected into a response
	response := get(c)

	if c.IsAborted() {
		return
	}

	// is it an error
	if response, ok := response.(hostdb.ErrorResponse); ok {
		sendResponse(c, response.Code, hostdb.GenericError{Error: response.Message})
		return
	}

	// is it a response
	if response, ok := response.(hostdb.GetRecordsResponse); ok {
		sendResponse(c, http.StatusOK, response)
		return
	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
		Error: "unknown",
	})
	return

}

// get list of record(s), providing only the requested (or default) fields
func getList(c *gin.Context) {

	// timer
	start := time.Now()

	// get the records collected into a response
	response := get(c)

	if c.IsAborted() {
		return
	}

	// is it an error
	if response, ok := response.(hostdb.ErrorResponse); ok {
		c.AbortWithStatusJSON(response.Code, hostdb.GenericError{
			Error: response.Message,
		})
		return
	}
	if response, ok := response.(error); ok {
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: response.Error(),
		})
		return
	}

	// is it a response
	if response, ok := response.(hostdb.GetRecordsResponse); ok {
		// list view is designed to return back a limited set of data, for brevity
		// with a full dataset in the response, we'll now remove the unwanted data

		// figure out if the user has requested specific fields,
		// or if we should fallback to the defaults in the config file
		var fieldSlice []string

		if fieldsParam := c.Query("_fields"); fieldsParam != "" {

			// if the user has specified which fields they want returned
			// TODO: try to do some deep matching ...
			//  if vc_url is requested, try to find that
			if strings.Contains(fieldsParam, ",") {
				fieldSlice = strings.Split(fieldsParam, ",")
			} else {
				fieldSlice = []string{fieldsParam}
			}

		} else {

			// loop through the fields in config
			fieldSlice = config.API.V0.ListFields

		}

		// for each record, only keep requested/default fields
		collection := map[string]hostdb.Record{}

		for id, record := range response.Records {

			newRecord := hostdb.Record{}

			for _, field := range fieldSlice {

				// loop over fields in the record struct
				for i := 0; i < reflect.TypeOf(record).NumField(); i++ {

					// if the field is to be preserved
					if field == strings.ToLower(reflect.TypeOf(record).Field(i).Name) {

						// lookup field by name
						newRecordField := reflect.ValueOf(&newRecord).Elem().Field(i)
						if !newRecordField.IsValid() {
							break
						}

						// field must be exported
						if !newRecordField.CanSet() {
							log.Println(fmt.Sprintf("unable to set the field %s", field))
							break
						}

						value := reflect.ValueOf(&record).Elem().Field(i)
						newRecordField.Set(value)
						break

					}

				}

				// todo: if the requested field isn't part of the standard record struct
				// attempt to find a match in the queryparams (e.g. ?_fields=stack_name,env,image)
				//
				// the problem with this, is that GetRecordsResponse has a map of Records
				// that struct won't work in this scenario

			}

			collection[id] = newRecord

		}

		// stop the query timer
		end := time.Now()
		latency := end.Sub(start)

		sendResponse(c, http.StatusOK, hostdb.GetRecordsResponse{
			Count:     response.Count,
			QueryTime: fmt.Sprintf("%v", latency),
			Records:   collection,
		})
		return

	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
		Error: "unknown",
	})
	return
}

func get(c *gin.Context) (response interface{}) {

	// timer
	start := time.Now()

	// check if an ID has been specified
	id := c.Param("id")
	if id != "" {
		record, err := getRecord(id)
		if err != nil {
			return err
		}

		// stop the query timer
		end := time.Now()
		latency := end.Sub(start)

		return hostdb.GetRecordsResponse{
			Count:     1,
			QueryTime: fmt.Sprintf("%v", latency),
			Records:   map[string]hostdb.Record{id: record},
		}
	}

	// check for any query params
	query := c.Request.URL.Query()

	// if none, return all records
	if len(query) == 0 {
		records, foundRows, err := getMariadbRows(hostdb.MariadbWhereClauses{}, hostdb.MariadbLimit{})
		if err != nil {
			return err
		}

		// stop the query timer
		end := time.Now()
		latency := end.Sub(start)

		return hostdb.GetRecordsResponse{
			Count:     foundRows,
			QueryTime: fmt.Sprintf("%v", latency),
			Records:   records,
		}
	}

	// start processing query params
	records, foundRows, err := processQueryParams(query)
	if err != nil {
		return err
	}

	// stop the query timer
	end := time.Now()
	latency := end.Sub(start)

	// return what we've got
	return hostdb.GetRecordsResponse{
		Count:     foundRows,
		QueryTime: fmt.Sprintf("%v", latency),
		Records:   records,
	}
}

// parse the query parameters into a Where object, return a collection of records indexed by their ID
func processQueryParams(query map[string][]string) (records map[string]hostdb.Record, foundRows int, err error) {

	where := hostdb.MariadbWhereClauses{
		Groups: []hostdb.MariadbWhereGrouping{},
	}

	limit := hostdb.MariadbLimit{}

	// for each of the requested query params
	i := 0
	for requestedParam, requestedParamValue := range query {

		switch requestedParam {
		case "_limit":
			i, err := strconv.Atoi(requestedParamValue[0])
			if err != nil {
				return nil, 0, err
			}
			// if i is negative, foul
			if i < 0 {
				return nil, 0, hostdb.ErrorResponse{
					Code:    http.StatusBadRequest,
					Message: "_limit parameter must not be negative",
				}
			}
			limit.Limit = i
		case "_offset":
			i, err := strconv.Atoi(requestedParamValue[0])
			if err != nil {
				return nil, 0, err
			}
			// if i is negative, foul
			if i < 0 {
				return nil, 0, hostdb.ErrorResponse{
					Code:    http.StatusBadRequest,
					Message: "_offset parameter must not be negative",
				}
			}
			limit.Offset = i
		case "_search", "!_search":
			// sloppy search
			for _, val := range requestedParamValue {
				if len(val) < 1 {
					continue
				}

				likeOperator := "LIKE"
				nullOperator := "IS NOT NULL"
				relativity := "OR"
				if requestedParam[0:1] == "!" {
					// detect a negative assertion
					requestedParam = requestedParam[1:]
					likeOperator = "NOT LIKE"
					nullOperator = "IS NULL"
					relativity = "AND"
				}

				where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
					Clauses: []hostdb.MariadbWhereClause{
						{
							Relativity: relativity,
							Key:        []string{fmt.Sprintf("json_search(data, 'one', '%%%v%%')", val)},
							Operator:   nullOperator,
							Value:      []string{},
						}, {
							Relativity: relativity,
							Key:        []string{fmt.Sprintf("json_search(context, 'one', '%%%v%%')", val)},
							Operator:   nullOperator,
							Value:      []string{},
						}, {
							Relativity: relativity,
							Key:        []string{"hostname"},
							Operator:   likeOperator,
							Value:      []string{fmt.Sprintf("%%%s%%", val)},
						}, {
							Relativity: relativity,
							Key:        []string{"ip"},
							Operator:   likeOperator,
							Value:      []string{fmt.Sprintf("%%%s%%", val)},
						}, {
							Relativity: relativity,
							Key:        []string{"type"},
							Operator:   likeOperator,
							Value:      []string{fmt.Sprintf("%%%s%%", val)},
						}, {
							Relativity: relativity,
							Key:        []string{"committer"},
							Operator:   likeOperator,
							Value:      []string{fmt.Sprintf("%%%s%%", val)},
						},
					},
				})
			}
		default:
			var keys, values []string
			var negativeAssertion bool
			var paramMatch = false
			var param map[string]hostdb.APIv0QueryParam

			if requestedParam[len(requestedParam)-2:] == "[]" {
				// attempt to detect an array of checkboxes (foo[] => foo)
				requestedParam = requestedParam[0 : len(requestedParam)-2]
			} else if requestedParam[0:1] == "!" {
				// detect a negative assertion
				requestedParam = requestedParam[1:]
				negativeAssertion = true
			}

			// check if this param is supported
			for paramName, queryParam := range config.API.V0.QueryParams {
				if paramName == requestedParam {
					paramMatch = true
					param = queryParam
					break
				}
			}

			// foul if a requested param isn't supported
			// params with leading underscores are special/fancy and exempt
			if !paramMatch && requestedParam[0:1] != "_" {
				return nil, 0, hostdb.ErrorResponse{
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("unsupported query param '%s'", requestedParam),
				}
			}

			// prepare the key/field for the WHERE clause
			for _, recordType := range param {
				var key string

				// key
				if recordType.Table != "" {
					// look for the key in the table itself
					key = recordType.Table
				} else if recordType.Context != "" {
					// look for the key in the record context
					key = fmt.Sprintf("json_value(context, '$%s') ", recordType.Context)
				} else if recordType.Data != "" {
					// look for the key in the record data
					key = fmt.Sprintf("json_value(data, '$%s') ", recordType.Data)
				} else {
					// this param isn't supported after all; ignore it
					continue
				}

				exist := false
				for _, k := range keys {
					if k == key {
						exist = true
					}
				}

				if !exist {
					keys = append(keys, key)
				}
			}

			if len(keys) < 1 {
				continue
			}

			// prepare the value(s) for the WHERE clause
			operator := "="

			if negativeAssertion {
				operator = "!="
			}

			for _, value := range requestedParamValue {
				if strings.Contains(value, ",") {
					split := strings.Split(value, ",")
					values = append(values, split...)
				} else {
					if len(value) > 0 {
						// regex is supported with /bounding slashes/
						if value[len(value)-1:] == "/" && value[0:1] == "/" {
							values = append(values, value[1:len(value)-1])
							if negativeAssertion {
								operator = "NOT RLIKE"
							} else {
								operator = "RLIKE"
							}
						} else {
							values = append(values, value)
						}
					}
				}
			}

			if len(values) < 1 {
				if negativeAssertion {
					operator = "IS NULL"
				} else {
					operator = "IS NOT NULL"
				}
			} else if len(values) > 1 {
				if negativeAssertion {
					operator = "IS NOT IN"
				} else {
					operator = "IN"
				}
			}

			// put it all together
			if len(keys) > 0 && operator != "" {
				where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
					Clauses: []hostdb.MariadbWhereClause{
						{
							Relativity: "AND",
							Key:        keys,
							Operator:   operator,
							Value:      values,
						},
					},
				})
			}
		}

		i++

	}

	// get records from the db
	records, foundRows, err = getMariadbRows(where, limit)
	if err != nil {
		return nil, 0, err
	}

	return records, foundRows, nil

}

// given an id, return a HostDB record
func getRecord(id string) (record hostdb.Record, err error) {

	record, err = getMariadbRow(id)
	if err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))
			return hostdb.Record{},
				hostdb.ErrorResponse{
					Code:    http.StatusInternalServerError,
					Message: "getting the record from the database failed",
				}
		}

		// all other errors
		log.Println(err.Error())

		return hostdb.Record{},
			hostdb.ErrorResponse{
				Code:    http.StatusInternalServerError,
				Message: "somewhere, something went wrong",
			}
	} else if record.ID == "" {
		return hostdb.Record{},
			hostdb.ErrorResponse{
				Code:    http.StatusUnprocessableEntity,
				Message: "record not found",
			}
	}

	return record, nil

}

// get a catalog of thing(s)
func getCatalog(c *gin.Context) {

	// setup
	var frequencyCount bool
	var filter string

	// timer
	start := time.Now()

	// check if an item has been specified
	item := c.Param("item")
	if item == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.GenericError{Error: "no item specified"})
		return
	} else if !validQueryParam(item) { // check if it's a valid item
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, hostdb.GenericError{Error: "that item is not familiar"})
		return
	}

	query := c.Request.URL.Query()

	if len(query) >= 1 {
		for key, values := range query {
			switch key {
			case "count":
				if values[0] != "0" && strings.ToLower(values[0]) != "false" {
					frequencyCount = true
				}
			case "filter":
				if values[0][:1] != "/" || values[0][len(values[0])-1:] != "/" {
					c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.GenericError{Error: "invalid regex encapsulation"})
					return
				}
				filter = values[0]
			default:
				continue // unsupported query parameter
			}
		}
	}

	catalog, err := getMariadbCatalog(item, frequencyCount, filter)
	if err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{Error: "getting the record from the database failed"})
			return
		}

		// all other errors
		log.Println(err.Error())

		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{Error: "somewhere, something went wrong"})
		return

	} else if len(catalog) < 1 {
		c.AbortWithStatusJSON(http.StatusNotFound, hostdb.GenericError{Error: "catalog not found"})
		return
	}

	if c.IsAborted() {
		return
	}

	// stop the query timer
	end := time.Now()
	latency := end.Sub(start)

	// get the records collected into a response
	if frequencyCount {
		sendResponse(c, http.StatusOK, hostdb.GetCatalogQuantityResponse{
			Count:     len(catalog),
			QueryTime: fmt.Sprintf("%v", latency),
			Catalog:   catalog,
		})
	} else {
		items := make([]string, 0, len(catalog))
		for i := range catalog {
			items = append(items, i)
		}

		sendResponse(c, http.StatusOK, hostdb.GetCatalogResponse{
			Count:     len(catalog),
			QueryTime: fmt.Sprintf("%v", latency),
			Catalog:   items,
		})
	}

	return

}

// check if a queryParam / item is valid. Returns true/false.
func validQueryParam(item string) bool {

	// check if this param is supported
	for paramName := range config.API.V0.QueryParams {
		if paramName == item {
			return true
		}
	}

	return false

}

// delete a single record
func deleteRecord(c *gin.Context) {

	// verify id
	id := c.Param("id")
	if id == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.GenericError{
			Error: "no id provided",
		})
		return
	}

	// check for the record
	record, err := getMariadbRow(id)
	if err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
				Error: "could not get record from the database",
			})
			return
		}

		// all other errors
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "delete failed",
		})
		return
	}

	// if we didn't get a record back, return a 422
	if record.ID == "" {
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	}

	// DELETE
	if err := deleteMariadbRow(id); err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
				Error: "deleting the record failed",
			})
			return
		}

		// all other errors
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.GenericError{
			Error: "delete failed",
		})
		return
	}

	sendResponse(c, http.StatusOK, gin.H{
		"id":      id,
		"deleted": true,
	})
	return

}

// post many records at once
func postBulk(c *gin.Context) {

	var bulk hostdb.RecordSet
	var replacements []hostdb.Record

	// get the raw request data
	rawData, err := c.GetRawData()
	if err != nil {
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "failed to get request data",
		})
		return
	}

	// marshal the []bytes into our struct
	if err = json.Unmarshal([]byte(rawData), &bulk); err != nil {
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "did not conform to expected standards",
		})
		return
	}

	//
	// validation
	//

	// ensure we have a type, and that it contains no spaces
	if bulk.Type == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "no type provided",
		})
		return
	} else if strings.Contains(bulk.Type, " ") {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "type cannot contain a space character",
		})
		return
	}

	// ensure we have a timestamp
	if bulk.Timestamp == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "no timestamp provided",
		})
		return
	}

	// is the timestamp valid
	timestamp, err := time.Parse("2006-01-02 15:04:05", bulk.Timestamp)
	if timestamp.IsZero() {
		bulk.Timestamp = time.Now().UTC().Format("2006-01-02 15:04:05")
	}

	// ensure we have context
	if bulk.Context == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "no context provided",
		})
		return
	}

	// ensure we have a committer
	if bulk.Committer == "" {
		bulk.Committer = fmt.Sprintf("%v: %v", c.Request.RemoteAddr, c.Request.UserAgent())
	}

	// ensure each of the required context fields are present for this type of record
	for recordType := range config.API.V0.ContextFields {
		if strings.Contains(bulk.Type, recordType) {
			for _, k := range config.API.V0.ContextFields[bulk.Type] {
				if _, ok := bulk.Context[k]; !ok {
					c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
						OK:    false,
						Error: fmt.Sprintf("missing context value for %s", k),
					})
					return
				}
			}
			break
		}
	}

	// ensure we have a data payload for each record
	for _, record := range bulk.Records {
		if record.Data == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, hostdb.PostRecordsResponse{
				OK:    false,
				Error: "data payload/element is missing from one or more records",
			})
			return
		}
	}

	// prepare a collection of existing records, based on type and bulk.Context
	var collection map[string]hostdb.Record

	where := hostdb.MariadbWhereClauses{
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "AND",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{bulk.Type},
					},
				},
			},
		},
	}

	// build the WHERE clauses for this bulk.Type
	if strings.Contains(bulk.Type, "aws") {
		where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
			Clauses: []hostdb.MariadbWhereClause{
				{
					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["aws-region"]["aws"].Context)},
					Operator:   "",
					Value:      []string{bulk.Context["aws-region"].(string)},
				}, {
					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["aws-account-id"]["aws"].Context)},
					Operator:   "",
					Value:      []string{bulk.Context["aws-account-id"].(string)},
				},
			},
		})
	} else if strings.Contains(bulk.Type, "oneview") { // filter by oneview_url
		where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
			Clauses: []hostdb.MariadbWhereClause{
				{
					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["oneview_url"]["oneview"].Context)},
					Operator:   "",
					Value:      []string{bulk.Context["oneview_url"].(string)},
				},
			},
		})
	} else if bulk.Type == "openstack" { // ensure we only get records for this tenant
		where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
			Clauses: []hostdb.MariadbWhereClause{
				{
					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["tenant"]["openstack"].Context)},
					Operator:   "=",
					Value:      []string{bulk.Context["tenant_name"].(string)},
				},
			},
		})
	} else if strings.Contains(bulk.Type, "ucs") { // filter by ucs_url
		where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
			Clauses: []hostdb.MariadbWhereClause{
				{
					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["ucs_url"]["ucs"].Context)},
					Operator:   "",
					Value:      []string{bulk.Context["ucs_url"].(string)},
				},
			},
		})
	} else if bulk.Type == "vrops-vmware" { // filter by vc_url
		where.Groups[0].Clauses[0].Operator = "LIKE"
		where.Groups[0].Clauses[0].Value = []string{fmt.Sprintf("%s%%", where.Groups[0].Clauses[0].Value[0])}
		where.Groups = append(where.Groups, hostdb.MariadbWhereGrouping{
			Clauses: []hostdb.MariadbWhereClause{
				{

					Relativity: "AND",
					Key:        []string{fmt.Sprintf("json_value(context, '$%s')", config.API.V0.QueryParams["vc_url"]["vrops-vmware"].Context)},
					Operator:   "=",
					Value:      []string{bulk.Context["vc_url"].(string)},
				},
			},
		})
	}

	// attempt to retrieve existing records
	if len(where.Groups[0].Clauses) > 0 {
		collection, _, err = getMariadbRows(where, hostdb.MariadbLimit{})
		if err != nil {
			log.Println(err.Error())
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
				OK:    false,
				Error: "Couldn't get existing records before applying bulk record request.",
			})
			return
		}
	}

	// loop over the new records in the request
	for _, record := range bulk.Records {

		// get any missing data from the bulk record set
		if record.Type == "" {
			record.Type = bulk.Type
		}

		if record.Timestamp == "" {
			record.Timestamp = bulk.Timestamp
		}

		if record.Committer == "" {
			record.Committer = bulk.Committer
		}

		// smoosh context
		for key, val := range bulk.Context {
			if record.Context == nil {
				record.Context = map[string]interface{}{}
			}

			if _, ok := record.Context[key]; ok {
				if record.Context[key] == "" {
					// if the context key is present, but empty
					record.Context[key] = val
				}
			} else {
				// if the context key is absent
				record.Context[key] = val
			}
		}

		// hash data payload
		record.Hash, err = hashPayload(record.Data)
		if err != nil {
			log.Println(err.Error())
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
				OK:    false,
				Error: "hashing the data failed",
			})
			return
		}

		// ensure data consistency before finding a match
		if err = ensureDataIsComplete(&record); err != nil {
			log.Println(err.Error())
			c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
				OK:    false,
				Error: "attempting to enforce data consistency failed",
			})
			return
		}

		existing := hostdb.Record{}
		if record.ID == "" {

			// if there are no records to check against
			// then this must be a new record
			if len(collection) < 1 {
				record.ID = getUUID("hdb")
			}

			for _, v := range collection {
				// attempt to match a record
				switch bulk.Type {
				case "aws-bucket":
					var newRecord, oldRecord struct {
						ID string `json:"Name"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-database":
					var newRecord, oldRecord struct {
						ID string `json:"DbiResourceId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-directconnect":
					var newRecord, oldRecord struct {
						ID string `json:"VirtualInterfaceId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-hostedzone":
					var newRecord, oldRecord struct {
						ID string `json:"Id"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-image":
					var newRecord, oldRecord struct {
						ID string `json:"ImageId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-keypair":
					var newRecord, oldRecord struct {
						ID string `json:"KeyName"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-securitygroup":
					var newRecord, oldRecord struct {
						ID string `json:"GroupId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-subnet":
					var newRecord, oldRecord struct {
						ID string `json:"SubnetId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "aws-vpc":
					var newRecord, oldRecord struct {
						ID string `json:"VpcId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "oneview-enclosure", "oneview-enclosure_group", "oneview-ethernet_network", "oneview-fc_network", "oneview-fcoe_network", "oneview-interconnect", "oneview-interconnect_type", "oneview-logical_enclosure", "oneview-logical_interconnect", "oneview-logical_interconnect_group", "oneview-network_set", "oneview-scope", "oneview-server_hardware", "oneview-server_hardware_type", "oneview-server_profile", "oneview-server_profile_template", "oneview-storage_pool", "oneview-storage_system", "oneview-storage_volume", "oneview-storage_volume_attachment", "oneview-storage_volume_template", "oneview-task", "oneview-uplink_set":
					var newRecord, oldRecord struct {
						ID string `json:"uri"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "openstack": // match on openstack guid
					var newRecord, oldRecord struct {
						ID string `json:"id"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "ucs-cpu", "ucs-fabric_interconnect", "ucs-memory", "ucs-pci", "ucs-storage", "ucs-vhba", "ucs-vic", "ucs-vnic":
					var newRecord, oldRecord struct {
						ID string `json:"dn"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "ucs-disk", "ucs-psu":
					var newRecord, oldRecord struct {
						ID string `json:"serial"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				case "vrops-vmware":
					var newRecord, oldRecord struct {
						ID string `json:"resourceId"`
					}

					if err := loadRecords(record.Data, &newRecord, v.Data, &oldRecord); err != nil {
						log.Println(err.Error())
						c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
							OK:    false,
							Error: "failed to unmarshal record data",
						})
						return
					}

					if newRecord.ID != oldRecord.ID {
						continue // keep looking for a match
					}
				default:
					// if we don't have a better way of identifying existing records, fall back on hostname (this really *is* awful. i'm so ashamed.)
					if record.Hostname != v.Hostname {
						continue // keep looking for a match
					}
				}

				// since we've found a match in the database, give record the id
				record.ID = v.ID
				existing = v

				// don't need to keep loo{k|p}ing, we found a match
				break
			}

		} else { // if id is present, attempt match to existing

			existing, err = getMariadbRow(record.ID)
			if err != nil {
				log.Println(err.Error())
				c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
					OK:    false,
					Error: fmt.Sprintf("failed to get database record, id = %v", record.ID),
				})
				return
			}

		}

		// if we don't have a record id by now, generate a new one
		if record.ID == "" {
			record.ID = getUUID("hdb")
		}

		// if the record has changed, replace it in the database
		if anythingChanged(record, existing) {
			replacements = append(replacements, record)
		}

		// remove this id from the records to be deleted
		delete(collection, record.ID)
	}

	// save all the records
	if err := saveMariadbRows(replacements); err != nil {
		log.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "failed to insert/replace records",
		})
		return
	}

	// delete all ids that remain in the collection
	deleteFail := false
	for id := range collection {
		if err = deleteMariadbRow(id); err != nil {
			log.Println(err.Error())
			deleteFail = true // keep trying
		}
	}

	if deleteFail {
		c.AbortWithStatusJSON(http.StatusInternalServerError, hostdb.PostRecordsResponse{
			OK:    false,
			Error: "Deleting one or more records failed. Please check the HostDB server logs.",
		})
		return
	}

	sendResponse(c, http.StatusOK, hostdb.PostRecordsResponse{
		OK:    true,
		Error: fmt.Sprintf("%d record(s) processed", len(bulk.Records)),
	})

}

func loadRecords(newRecord []byte, newRecordStruct interface{}, existingRecord []byte, existingRecordStruct interface{}) error {
	// get the new record openstack id
	if err := json.Unmarshal(newRecord, &newRecordStruct); err != nil {
		return err
	}

	// get the existing record openstack id
	if err := json.Unmarshal(existingRecord, &existingRecordStruct); err != nil {
		return err
	}

	return nil
}

// take in a map of records, and return headers and lines for printing
func renderData(records map[string]hostdb.Record) (headers []string, lines []map[string]string, err error) {
	headers = []string{"ID", "Type", "Hostname", "IP Address", "Last Updated"}
	var extraHeaders []string

	for _, record := range records {
		line := map[string]string{}

		line["ID"] = record.ID
		line["Type"] = record.Type
		line["Hostname"] = record.Hostname
		line["IP Address"] = record.IP
		line["Last Updated"] = record.Timestamp

		// convert context values to strings
		for k, v := range record.Context {
			// skip over some fields that really aren't helpful
			switch k {
			case "os_auth_url",
				"tenant_id":
				continue
			}

			key := strings.ToLower(getDisplayText(k))

			found := false
			for _, h := range extraHeaders {
				if key == h {
					found = true
				}
			}

			if found == false {
				extraHeaders = append(extraHeaders, key)
			}

			line[key] = fmt.Sprintf("%v", v)
		}

		// print metadata
		if strings.ToLower(record.Type) == "openstack" {
			// metadata struct
			var metadata struct {
				Metadata map[string]string `json:"metadata"`
			}

			if err := json.Unmarshal(record.Data, &metadata); err != nil {
				return nil, nil, err
			}

			// convert metadata to strings
			for k, v := range metadata.Metadata {
				// skip some metadata fields that really aren't that helpful
				switch k {
				case "datacenter",
					"tenant.name":
					continue
				}

				found := false
				for _, h := range extraHeaders {
					if k == h {
						found = true
					}
				}

				if found == false {
					extraHeaders = append(extraHeaders, k)
				}

				line[k] = fmt.Sprintf("%v", v)
			}
		}

		// last but not least
		line["data"] = fmt.Sprintf("%s", record.Data)

		lines = append(lines, line)
	}

	// sort the extra headers, add them on
	sort.Strings(extraHeaders)
	headers = append(headers, extraHeaders...)
	headers = append(headers, "data")

	return headers, lines, nil
}

// return results as a CSV
func outputCSV(c *gin.Context) {

	// check for any query params
	query := c.Request.URL.Query()

	// if no query parameters, return an error
	if len(query) == 0 {
		c.HTML(http.StatusBadRequest, "error.html", "no search terms provided")
		return
	}

	// start processing query params
	records, _, err := processQueryParams(query)
	if err != nil {
		if err, ok := err.(hostdb.ErrorResponse); ok {
			c.HTML(err.Code, "error.html", err.Message)
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		}
		return
	}

	header, lines, err := renderData(records)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		return
	}

	// remove the data header
	if header[len(header)-1] == "data" {
		header = header[:len(header)-1]
	}

	// output to disk
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	if err := w.Write(header); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		return
	}

	for _, line := range lines {
		var outputRow []string

		for _, key := range header {

			if key == "data" {
				continue // don't display the data in the CSV
			}

			if line[key] == "" {
				line[key] = ""
			}

			outputRow = append(outputRow, line[key])

		}

		if err := w.Write(outputRow); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", err.Error())
			return
		}
	}

	w.Flush()

	if err := w.Error(); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", err.Error())
		return
	}

	// CSV output
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=metadata-"+time.Now().Format("20060102150405")+".csv")
	c.Data(http.StatusOK, "text/csv", b.Bytes())

}
