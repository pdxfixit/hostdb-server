package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/VividCortex/mysqlerr"
	"github.com/go-sql-driver/mysql"
	"github.com/pdxfixit/hostdb"
)

var mariadb *sql.DB

func checkMariadb() bool {

	// if the database is in AWS, all we need to do, is create the table
	if checkTable() == true {
		return true
	}

	if err := createTable(); err != nil {
		log.Println("creating the table failed.")
		log.Println(err.Error())
	} else {
		return true
	}

	// for other situations
	if err := mariadb.Ping(); err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			switch err.Number {
			case mysqlerr.ER_ACCESS_DENIED_ERROR: // 1045
				log.Println("Attempting to create database, then will retry.")
				if setupError := setupDatabase(); setupError != nil {
					log.Println(setupError.Error())
					return false
				}

				log.Println("Retrying...")
				return checkMariadb()
			default:
				log.Println(fmt.Sprintf("checkMariadb: %v: %v", err.Number, err.Message))
				return false
			}
		} else {
			log.Println(err.Error())
			return false
		}
	}

	return true

}

func checkTable() bool {

	var count int

	statement := "SELECT COUNT(*) FROM `hostdb` LIMIT 1"

	debugMessage(statement)

	if err := mariadb.QueryRow(statement).Scan(&count); err != nil {
		log.Println(err.Error())
		return false
	}

	if count >= 0 {
		return true
	}

	return false

}

func createTable() error {

	bytes, err := ioutil.ReadFile("mariadb/create-table.sql")
	if err != nil {
		log.Println("could not read file mariadb/create-table.sql")
		return err
	}

	commands := strings.Split(string(bytes), ";")

	for _, command := range commands {
		cmd := strings.TrimSpace(command)
		if cmd != "" {
			debugMessage(cmd)
			if _, err := mariadb.Exec(cmd); err != nil {
				return err
			}
		}
	}

	log.Println("table created")

	return nil

}

func deleteMariadbRow(id string) error {

	// check for an existing ID
	record, err := getMariadbRow(id)
	if err != nil {
		return err
	}

	if record.ID == "" {
		return errors.New("record not found")
	}

	if id != record.ID {
		return errors.New("weird, record IDs didn't match")
	}

	statement := "DELETE FROM `hostdb` WHERE `id` = "

	debugMessage(fmt.Sprintf("%s%v", statement, id))

	res, err := mariadb.Exec(fmt.Sprintf("%s?", statement), id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("zero records deleted")
	}

	return nil

}

func getRowIds(clauses hostdb.MariadbWhereClauses) (recordIDs []string, err error) {

	whereSQL, values, err := clauses.Stringify()

	statement := fmt.Sprintf("SELECT `id` FROM `hostdb` %v", whereSQL)

	debugMessage(statement)

	rows, err := mariadb.Query(statement, values...)
	if err != nil {
		return nil, err
	}
	defer closer(rows)

	for rows.Next() {

		var id string

		if err = rows.Scan(&id); err != nil {
			return nil, err
		}

		recordIDs = append(recordIDs, id)

	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return recordIDs, nil

}

func getMariadbCatalog(item string, frequencyCount bool, filter string) (items map[string]int, err error) {

	// if the data location is anything other than table,
	// we will need to use the JSON_ functions
	items = map[string]int{}

	// for each record type
	for _, dataLocation := range config.API.V0.QueryParams[item] {

		var field string

		if dataLocation.Table != "" {
			// look for the key in the table itself
			field = dataLocation.Table
		} else if dataLocation.Context != "" {
			// look for the key in the record context
			field = fmt.Sprintf("json_value(context, '$%s') ", dataLocation.Context)
		} else if dataLocation.Data != "" {
			// look for the key in the record data
			// todo: consider using a different json_ function, like _contains or _search, so that we can extract data from vrops and AWS records
			field = fmt.Sprintf("json_value(data, '$%s') ", dataLocation.Data)
		} else {
			// this param isn't supported after all; ignore it
			continue
		}

		// SELECT
		var selectArgument string
		if frequencyCount {
			selectArgument = fmt.Sprintf("%s, COUNT(*)", field)
		} else {
			selectArgument = field
		}

		// WHERE
		clauses := hostdb.MariadbWhereClauses{
			Groups: []hostdb.MariadbWhereGrouping{
				{
					Clauses: []hostdb.MariadbWhereClause{
						{
							Relativity: "AND",
							Key:        []string{field},
							Operator:   "IS NOT NULL",
							Value:      []string{},
						},
					},
				},
			},
		}

		if filter != "" {
			clauses.Groups[0].Clauses = append(clauses.Groups[0].Clauses, hostdb.MariadbWhereClause{
				Relativity: "AND",
				Key:        []string{field},
				Operator:   "RLIKE",
				Value:      []string{filter[1 : len(filter)-1]},
			})
		}

		whereSQL, values, err := clauses.Stringify()

		statement := fmt.Sprintf("SELECT DISTINCT %s FROM `hostdb` %v GROUP BY %s", selectArgument, whereSQL, field)

		debugMessage(statement)

		rows, err := mariadb.Query(statement, values...)
		if err != nil {
			return nil, err
		}
		defer closer(rows)

		for rows.Next() {

			var i string
			var n int

			if frequencyCount {
				if err = rows.Scan(&i, &n); err != nil {
					return nil, err
				}
			} else {
				if err = rows.Scan(&i); err != nil {
					return nil, err
				}
			}

			if _, ok := items[i]; !ok {
				items[i] = n
			}

		}

		if err = rows.Err(); err != nil {
			return nil, err
		}
	}

	return items, nil

}

func getMariadbRow(id string) (record hostdb.Record, err error) {

	var contextString string

	statement := "SELECT `id`, `type`, `hostname`, `ip`, `timestamp`, `committer`, `context`, `data`, `hash` FROM `hostdb` WHERE `id` = "

	debugMessage(fmt.Sprintf("%s%v", statement, id))

	err = mariadb.QueryRow(fmt.Sprintf("%s?", statement), id).Scan(
		&record.ID,
		&record.Type,
		&record.Hostname,
		&record.IP,
		&record.Timestamp,
		&record.Committer,
		&contextString,
		&record.Data,
		&record.Hash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return hostdb.Record{}, nil
		}

		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("DB ERROR: %v: %v", err.Number, err.Message))
			return hostdb.Record{}, err
		}

		// TODO: if err.ErrorResponse == "invalid connection", re-establish connection and retry
		log.Println(fmt.Sprintf("getting id (%v) failed: %v", id, err.Error()))
		return hostdb.Record{}, err
	}

	if err := json.Unmarshal([]byte(contextString), &record.Context); err != nil {
		log.Println("failed to unmarshal context into a map")
		return hostdb.Record{}, err
	}

	return record, nil

}

func getMariadbRows(clauses hostdb.MariadbWhereClauses, limit hostdb.MariadbLimit) (records map[string]hostdb.Record, foundRows int, err error) {

	whereSQL, values, err := clauses.Stringify()
	if err != nil {
		return nil, 0, err
	}

	limitSQL := limit.Stringify()

	statement := fmt.Sprintf("SELECT `id`, `type`, `hostname`, `ip`, `timestamp`, `committer`, `context`, `data`, `hash` FROM `hostdb` %s %s", whereSQL, limitSQL)

	debugMessage(statement)

	rows, err := mariadb.Query(statement, values...)
	if err != nil {
		return nil, 0, err
	}
	defer closer(rows)

	// get total number of records
	// http://www.mysqlperformanceblog.com/2007/08/28/to-sql_calc_found_rows-or-not-to-sql_calc_found_rows/
	totalRecordsStatement := fmt.Sprintf("SELECT COUNT(*) FROM `hostdb` %s", whereSQL)

	debugMessage(totalRecordsStatement)

	if err = mariadb.QueryRow(totalRecordsStatement, values...).Scan(&foundRows); err != nil {
		return nil, 0, err
	}

	records = map[string]hostdb.Record{}

	for rows.Next() {

		var record hostdb.Record
		var contextString string

		if err = rows.Scan(
			&record.ID,
			&record.Type,
			&record.Hostname,
			&record.IP,
			&record.Timestamp,
			&record.Committer,
			&contextString,
			&record.Data,
			&record.Hash,
		); err != nil {
			return nil, 0, err
		}

		// unmarshal the context string into a map
		if err := json.Unmarshal([]byte(contextString), &record.Context); err != nil {
			log.Println("failed to unmarshal context into a map")
			return nil, 0, err
		}

		records[record.ID] = record

	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, foundRows, nil

}

func getMariadbVersion() (version string, err error) {

	if err = mariadb.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}

	return version, nil

}

// get the total number of records
func getTotalRecords() (count int, err error) {

	if err = mariadb.QueryRow("SELECT COUNT(*) FROM `hostdb`").Scan(&count); err != nil {
		return 0, err
	}

	return

}

// get the timestamp for the newest record
func getNewestTimestamp() (timestamp string, err error) {

	if err = mariadb.QueryRow("SELECT `timestamp` FROM `hostdb` ORDER BY `timestamp` DESC LIMIT 1").Scan(&timestamp); err != nil {
		return "", err
	}

	return

}

// get the timestamp for the oldest record
func getOldestTimestamp() (timestamp string, err error) {

	if err = mariadb.QueryRow("SELECT `timestamp` FROM `hostdb` ORDER BY `timestamp` ASC LIMIT 1").Scan(&timestamp); err != nil {
		return "", err
	}

	return

}

// get the recent timestamps for each of the committers
func getRecentCommitterTimestamps() (lastSeen map[string]string, err error) {

	lastSeen = make(map[string]string)

	rows, err := mariadb.Query("SELECT `committer`, MAX(`timestamp`) FROM hostdb WHERE `committer` IS NOT NULL GROUP BY `committer`")
	if err != nil {
		return nil, err
	}
	defer closer(rows)

	for rows.Next() {
		var committer, timestamp string

		if err = rows.Scan(
			&committer,
			&timestamp,
		); err != nil {
			return nil, err
		}

		lastSeen[committer] = timestamp
	}

	return

}

// create some global, generic database objects
func loadMariadb() (err error) {

	log.Println(fmt.Sprintf("[NOTICE] Testing connection to: %v@tcp(%v:%v)/%v?%v", config.Mariadb.User, config.Mariadb.Host, config.Mariadb.Port, config.Mariadb.DB, marshalParams()))

	mariadb, err = sql.Open("mysql",
		fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?%v", config.Mariadb.User, config.Mariadb.Pass, config.Mariadb.Host, config.Mariadb.Port, config.Mariadb.DB, marshalParams()))
	if err != nil {
		return err
	}
	//defer closer(mariadb)

	if checkMariadb() == false {
		if err = setupDatabase(); err != nil {
			log.Println("setting up the database failed")
			return err
		}
	}

	// http://techblog.en.klab-blogs.com/archives/31093990.html
	maxConnections := 20
	mariadb.SetMaxOpenConns(maxConnections)
	mariadb.SetMaxIdleConns(maxConnections)
	mariadb.SetConnMaxLifetime(time.Duration(maxConnections) * time.Second)

	return nil

}

func marshalParams() (params string) {

	for _, v := range config.Mariadb.Params {
		if len(params) > 1 {
			params += "&"
		}

		params += v
	}

	return params

}

func saveMariadbRow(record hostdb.Record) error {

	// failsafe
	if record.ID == "" {
		record.ID = getUUID("hdb")
	}

	statementString := "REPLACE INTO `hostdb` (`id`, `type`, `hostname`, `ip`, `timestamp`, `committer`, `context`, `data`, `hash`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

	debugMessage(statementString)

	statement, err := mariadb.Prepare(statementString)
	if err != nil {
		log.Println(fmt.Sprintf("save prepare failed: %v", statementString))
		return err
	}

	// marshal the context map into a string
	contextString, err := json.Marshal(record.Context)
	if err != nil {
		log.Println("failed to marshal context")
		return err
	}

	values := []string{
		record.ID,
		record.Type,
		record.Hostname,
		record.IP,
		record.Timestamp,
		record.Committer,
		fmt.Sprintf("%v", contextString),
		fmt.Sprintf("%v", record.Data),
		record.Hash,
	}

	debugMessage(values)

	if _, err = statement.Exec(
		record.ID,
		record.Type,
		record.Hostname,
		record.IP,
		record.Timestamp,
		record.Committer,
		contextString,
		record.Data,
		record.Hash,
	); err != nil {
		log.Println(fmt.Sprintf("save exec failed: %v", values))
		return err
	}

	// sanity
	//rowsAffected, err := result.RowsAffected()
	//if err != nil {
	//	log.Println("failed to get the number of rows affected in a save")
	//	return err
	//}
	//if rowsAffected > 1 {
	//	return errors.New(fmt.Sprintf("%v rows were affected, more than expected.", rowsAffected))
	//}

	return nil

}

// prepare an INSERT statement from a slice of records
func saveMariadbRows(records []hostdb.Record) error {

	if len(records) < 1 {
		return nil
	}

	statementString := "REPLACE INTO `hostdb` (`id`,`type`,`hostname`,`ip`,`timestamp`,`committer`,`context`,`data`,`hash`) VALUES "
	const rowValues = "(?,?,?,?,?,?,?,?,?)"
	var inserts []string
	var values []interface{}

	for _, record := range records {

		// marshal the context map into a string
		contextString, err := json.Marshal(record.Context)
		if err != nil {
			log.Println("failed to marshal context")
			return err
		}

		inserts = append(inserts, rowValues)
		values = append(values,
			record.ID,
			record.Type,
			record.Hostname,
			record.IP,
			record.Timestamp,
			record.Committer,
			string(contextString),
			string(record.Data),
			record.Hash,
		)

	}

	statementString = fmt.Sprintf("%s%s", statementString, strings.Join(inserts, ","))

	debugMessage(statementString)
	debugMessage(values)

	statement, err := mariadb.Prepare(statementString)
	if err != nil {
		log.Println(fmt.Sprintf("bulk save prepare failed: %v", statementString))
		return err
	}

	if _, err := statement.Exec(values...); err != nil {
		// if error 1205, retry up to 5 times
		if err, ok := err.(*mysql.MySQLError); ok {
			if err.Number == mysqlerr.ER_LOCK_WAIT_TIMEOUT { // 1205
				log.Println("Error 1205: Lock wait timeout exceeded; restarting transaction")

				counter := 5

				for i := 0; i < counter; i++ {
					if _, err := statement.Exec(values...); err != nil {
						if err, ok := err.(*mysql.MySQLError); ok {
							if err.Number == mysqlerr.ER_LOCK_WAIT_TIMEOUT { // still 1205
								log.Printf("Error 1205: Lock wait timeout exceeded; restarting transaction (%dx)\n", i+2)
								continue
							}
						}

						// some other error
						log.Printf("bulk save exec failed: %v\n", values)
						return err
					}

					// transaction succeeded
					return nil
				}

				log.Printf("maximum number of retries reached (%d)", counter)
			}
		}

		log.Printf("bulk save exec failed: %v\n", values)
		return err
	}

	return nil

}

func setupDatabase() error {

	dsn := fmt.Sprintf("root@tcp(%v:%v)/%v", config.Mariadb.Host, config.Mariadb.Port, config.Mariadb.DB)

	debugMessage(dsn)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Println("setupDatabase: failed to create a connection")
		return err
	}

	if err = db.Ping(); err != nil {
		if err, ok := err.(*mysql.MySQLError); ok {
			log.Println(fmt.Sprintf("%v: %v", err.Number, err.Message))

			switch err.Number {
			case mysqlerr.ER_DBACCESS_DENIED_ERROR:
				return errors.New("user could not access the database")
			case mysqlerr.ER_ACCESS_DENIED_ERROR:
				return errors.New("something is wrong with the root user")
			case mysqlerr.ER_BAD_DB_ERROR:
				dsn := fmt.Sprintf("root@tcp(%v:%v)/", config.Mariadb.Host, config.Mariadb.Port)

				debugMessage(dsn)

				createDB, err := sql.Open("mysql", dsn)
				if err != nil {
					log.Println("createDB: failed to create a connection")
					return err
				}

				createDatabase := fmt.Sprintf("CREATE DATABASE %v", config.Mariadb.DB)

				debugMessage(createDatabase)

				_, createDBError := createDB.Exec(createDatabase)
				if createDBError != nil {
					log.Println("create database failed")
					return createDBError
				}

				log.Println("database created")
			default:
				return fmt.Errorf("error %v: %v", err.Number, err.Message)
			}
		} else {
			log.Println("setupDatabase: non mysqlerror")
			return err
		}
	}

	// create the app user
	createUserCommands := []string{"FLUSH PRIVILEGES",
		fmt.Sprintf("CREATE OR REPLACE USER '%v'@'%%' IDENTIFIED BY '%v'", config.Mariadb.User, config.Mariadb.Pass),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %v.* TO '%v'@'%%'", config.Mariadb.DB, config.Mariadb.User),
		"FLUSH PRIVILEGES"}
	for _, command := range createUserCommands {
		debugMessage(command)
		_, err := db.Exec(command)
		if err != nil {
			log.Println(fmt.Sprintf("Creating user failed, query: %v", command))
			return err
		}
	}

	log.Println("db privileges updated")

	if err := db.Ping(); err != nil {
		log.Println("second db ping attempt failed")
		return err
	}

	if err := db.Close(); err != nil {
		log.Println("closing the database failed")
		return err
	}

	if err = createTable(); err != nil {
		log.Println("creating table failed")
		return err
	}

	return nil

}
