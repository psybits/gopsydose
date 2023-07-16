package drugdose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"database/sql"

	"github.com/hasura/go-graphql-client"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

// TODO: Encryption should be done by default unless specified not to by the user from the settings
// But first the official implementation for encryption has to be done in the sqlite module

// TODO: Some basic tests need to be written

// TODO: Functions need comments.

const loggingTableName string = "userLogs"
const userSetTableName string = "userSettings"
const altNamesSubsTableName string = "substanceNames"
const altNamesRouteTableName string = "routeNames"
const altNamesUnitsTableName string = "unitsNames"
const altNamesConvUnitsTableName string = "convUnitsNames"

// When this number is set as the reference ID for remembering
// a particular input, it means that it's now "forgotten"
// and there should be no attempts to "remember" any inputs.
// This is related to the RememberConfig() and ForgetConfig() functions.
const ForgetInputConfigMagicNumber string = "0"

func exitProgram() {
	printName("exitProgram()", "Exiting")
	os.Exit(1)
}

func errorCantCloseDB(filePath string, err error) {
	printName("errorCantCloseDB()", "Can't close DB file:", filePath+":", err)
	exitProgram()
}

func errorCantCreateDB(filePath string, err error) {
	printName("errorCantCreateDB()", "Error creating drug info DB file:", filePath+":", err)
	exitProgram()
}

func errorCantOpenDB(filePath string, err error) {
	printName("errorCantOpenDB()", "Error opening DB:", filePath+":", err)
	exitProgram()
}

// If err is not nil, starts a transaction rollback and returns the error
// through errChannel.
//
// Returns true if there's an error, false otherwise.
func handleErrRollback(err error, tx *sql.Tx, errChannel chan error) bool {
	const printN string = "handleErrRollback()"

	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			errChannel <- errors.New(sprintName(printN, err2))
			return true
		}
		errChannel <- errors.New(sprintName(printN, err))
		return true
	}
	return false
}

type UserLog struct {
	StartTime int64
	Username  string
	EndTime   int64
	DrugName  string
	Dose      float32
	DoseUnits string
	DrugRoute string
}

type UserLogsError struct {
	UserLogs []UserLog
	Err      error
}

type DrugInfo struct {
	DrugName      string
	DrugRoute     string
	Threshold     float32
	LowDoseMin    float32
	LowDoseMax    float32
	MediumDoseMin float32
	MediumDoseMax float32
	HighDoseMin   float32
	HighDoseMax   float32
	DoseUnits     string
	OnsetMin      float32
	OnsetMax      float32
	OnsetUnits    string
	ComeUpMin     float32
	ComeUpMax     float32
	ComeUpUnits   string
	PeakMin       float32
	PeakMax       float32
	PeakUnits     string
	OffsetMin     float32
	OffsetMax     float32
	OffsetUnits   string
	TotalDurMin   float32
	TotalDurMax   float32
	TotalDurUnits string
	TimeOfFetch   int64
}

type SyncTimestamps struct {
	LastTimestamp int64
	LastUser      string
	Lock          sync.Mutex
}

func xtrastmt(col string, logical string) string {
	return logical + " " + col + " = ?"
}

func checkIfExistsDB(db *sql.DB, ctx context.Context,
	col string, table string, driver string,
	path string, xtrastmt []string, values ...interface{}) bool {

	const printN string = "checkIfExistsDB()"

	stmtstr := "select " + col + " from " + table + " where " + col + " = ?"
	if xtrastmt != nil {
		for i := 0; i < len(xtrastmt); i++ {
			stmtstr = stmtstr + " " + xtrastmt[i]
		}
	}

	// NOTE: this doesn't cause an SQL injection, because we're not taking
	// 'col' and 'table' from an user input.
	stmt, err := db.PrepareContext(ctx, stmtstr)
	if err != nil {
		printName(printN, "SQL error in prepare for check if exists:", err)
		return false
	}
	defer stmt.Close()
	var got string

	err = stmt.QueryRowContext(ctx, values...).Scan(&got)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		printName(printN, "received weird error:", err)
		return false
	}

	return true
}

// Ping verifies a connection to the database is still alive,
// establishing a connection if necessary.
//
// db - open database connection
//
// ctx - context to be passed to PingContext()
func (cfg Config) PingDB(db *sql.DB, ctx context.Context) {
	err := db.PingContext(ctx)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
}

// Uses the value of Timeout from the settings file to create a WithTimeout
// context. If no errors are found, it then returns the context to be used
// where it's needed.
func (cfg Config) UseConfigTimeout() (context.Context, context.CancelFunc, error) {
	if cfg.Timeout == "" || cfg.Timeout == "none" {
		return nil, nil, errors.New("Timeout value is empty.")
	}

	gotDuration, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), gotDuration)
	return ctx, cancel, nil
}

// Open a database connection using the Config struct.
//
// After calling this function, don't forget to run: defer db.Close()
//
// db being the name of the returned *sql.DB variable
//
// ctx - context to be passed to PingDB(), first passing through WithTimeout()
func (cfg Config) OpenDBConnection(ctx context.Context) *sql.DB {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}

	cfg.PingDB(db, ctx)
	return db
}

// CheckDBFileStruct returns true if the file structure is already created,
// false otherwise. Checks whether the db directory and minimum amount of files
// exist with the proper names in it. This is currently only useful for sqlite.
// If Config.DBDriver is not set to "sqlite3" it will return false.
func (cfg Config) CheckDBFileStruct() bool {
	const printN string = "CheckDBFileStruct()"

	if cfg.DBDriver != "sqlite3" {
		return false
	}

	dbFileLocat := cfg.DBSettings[cfg.DBDriver].Path

	_, err := os.Stat(dbFileLocat)
	if err == nil {
		printNameVerbose(cfg.VerbosePrinting, printN, dbFileLocat+": Exists")
	} else if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			printName(printN, dbFileLocat+": Doesn't seem to exist:", err)
			return false
		} else {
			printName(printN, err)
			return false
		}
	}

	return true
}

// InitDBFileStructure creates the basic file structure for the database.
// This should be run only once! The function calls CheckDBFileStruct,
// so there's no need to do it manually before calling it!
// This is currently only useful for sqlite.
// If Config.DBDriver is not set to "sqlite3" it will return.
func (cfg Config) InitDBFileStructure() {
	const printN string = "InitDBFileStructure()"

	if cfg.DBDriver != "sqlite3" {
		return
	}

	ret := cfg.CheckDBFileStruct()
	if ret == true {
		return
	}

	dirOnly := path.Dir(cfg.DBSettings[cfg.DBDriver].Path)

	err := os.Mkdir(dirOnly, 0700)
	if err != nil {
		printName(printN, "Error creating directory for DB:", err)
		exitProgram()
	}

	dbFileLocat := cfg.DBSettings[cfg.DBDriver].Path

	file, err := os.Create(dbFileLocat)
	if err != nil {
		errorCantCreateDB(dbFileLocat, err)
	}

	err = file.Close()
	if err != nil {
		errorCantCloseDB(dbFileLocat, err)
	}

	printName(printN, "Initialised the DB file structure.")
}

// RemoveSingleDrugInfoDB removes all entries of a single drug from the local
// info DB, instead of deleting the whole DB. For example if there's a need to
// clear all information about dosage and timing for a specific drug if it's
// old or incorrect.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - go routine channel which returns any errors
//
// drug - name of drug to be removed from source table
func (cfg Config) RemoveSingleDrugInfoDB(db *sql.DB, ctx context.Context,
	errChannel chan error, drug string) {
	const printN string = "RemoveSingleDrugInfoDB()"

	drug = cfg.MatchAndReplace(db, ctx, drug, "substance")

	ret := checkIfExistsDB(db, ctx,
		"drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		errChannel <- errors.New(sprintName(printN, "No such drug in info database: ", drug))
		return
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, err))
		return
	}

	stmt, err := tx.Prepare("delete from " + cfg.UseSource +
		" where drugName = ?")
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "tx.Prepare(): ", err))
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(drug)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "stmt.Exec(): ", err))
		return
	}

	err = tx.Commit()
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "tx.Commit(): ", err))
		return
	}

	printName(printN, "Data removed from info DB successfully:", drug, "; source: ", cfg.UseSource)

	errChannel <- nil
}

// Function which generates and returns a query for looking up table
// names in the database.
// If tableName is empty, query returns all tables in the database.
// If tableName is not empty, query returns a specific table if it exists.
func (cfg Config) getTableNamesQuery(tableName string) string {
	var queryStr string
	andTable := ""
	if cfg.DBDriver == "sqlite3" {
		if tableName != "" {
			andTable = " AND name = '" + tableName + "'"
		}
		queryStr = "SELECT name FROM sqlite_schema WHERE type='table'" + andTable
	} else if cfg.DBDriver == "mysql" {
		if tableName != "" {
			andTable = " AND table_name = '" + tableName + "'"
		}
		dbName := strings.Split(cfg.DBSettings[cfg.DBDriver].Path, "/")
		queryStr = "SELECT table_name FROM information_schema.tables WHERE table_schema = '" +
			dbName[1] + "'" + andTable
	}
	return queryStr
}

// CheckDBTables returns true if a table exists in the database with the name
// tableName. It returns false in case of error or if the table isn't found.
// If tableName is empty it will search for all tables in the database.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// tableName - name of table to check if it exists
func (cfg Config) CheckDBTables(db *sql.DB, ctx context.Context, tableName string) bool {
	const printN string = "CheckDBTables()"

	queryStr := cfg.getTableNamesQuery(tableName)
	rows, err := db.QueryContext(ctx, queryStr)
	if err != nil {
		printName(printN, err)
		return false
	}
	defer rows.Close()

	var tableList []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			printName(printN, err)
			return false
		}
		tableList = append(tableList, name)
	}

	return len(tableList) != 0
}

// CleanDB deletes all tables in the database. This is why it's a good idea
// to separate the data for drug logs from anything else. Create a separate
// database for that data only!
//
// Returns false on error.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) CleanDB(db *sql.DB, ctx context.Context) error {
	const printN string = "CleanDB()"

	queryStr := cfg.getTableNamesQuery("")
	rows, err := db.QueryContext(ctx, queryStr)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}
	defer rows.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New(sprintName(printN, "db.BeginTx(): ", err))
	}

	if cfg.VerbosePrinting == true {
		printNameNoNewline(printN, "Removing tables: ")
	}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return errors.New(sprintName(printN, err))
		}

		if cfg.VerbosePrinting == true {
			fmt.Print(name + ", ")
		}

		_, err = tx.Exec("drop table " + name)
		if err != nil {
			return errors.New(sprintName(printN, "tx.Exec(): ", err))
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.New(sprintName(printN, "tx.Commit(): ", err))
	}

	if cfg.VerbosePrinting == true {
		fmt.Println()
	}
	printName(printN, "All tables removed from DB.")

	return nil
}

// CleanInfo removes the currently configured info table. For example if source
// is set to "psychonautwiki" it will delete the table with the same name as
// the source, containing all data like dosages and timings. All user dosages
// aren't touched since they're not apart of the drug general information.
//
// Returns false on error.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) CleanInfo(db *sql.DB, ctx context.Context) error {
	const printN string = "CleanInfo()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New(sprintName(printN, "db.BeginTx(): ", err))
	}

	_, err = tx.Exec("drop table " + cfg.UseSource)
	if err != nil {
		return errors.New(sprintName(printN, "tx.Exec(): ", err))
	}

	err = tx.Commit()
	if err != nil {
		return errors.New(sprintName(printN, "tx.Commit(): ", err))
	}

	printName(printN, "The info table: "+cfg.UseSource+"; removed from DB.")

	return nil
}

// CleanNames removes the main names tables and the currently configured ones
// as well. Names are "alternative names" like "weed" for "cannabis" and etc.
// Main names are global, they apply to all sources. Currently configured ones
// are source specific and are chosen based on the currently used source.
// This means, that any old names generated for another source aren't removed.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// replaceOnly - remove only replace tables, keep the global ones intact
func (cfg Config) CleanNames(db *sql.DB, ctx context.Context, replaceOnly bool) error {
	const printN string = "CleanNames()"

	tableSuffix := "_" + cfg.UseSource
	tableNames := [8]string{altNamesSubsTableName,
		altNamesRouteTableName,
		altNamesUnitsTableName,
		altNamesConvUnitsTableName,
		altNamesSubsTableName + tableSuffix,
		altNamesRouteTableName + tableSuffix,
		altNamesUnitsTableName + tableSuffix,
		altNamesConvUnitsTableName + tableSuffix}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	startCount := 0
	if replaceOnly == true {
		startCount = 4
	}
	printNameNoNewline(printN, "Removing tables: ")
	for i := startCount; i < len(tableNames); i++ {
		fmt.Print(tableNames[i] + ", ")

		_, err = tx.Exec("drop table " + tableNames[i])
		if err != nil {
			return errors.New(sprintName(printN, "tx.Exec(): ", err))
		}
	}
	fmt.Println()

	err = tx.Commit()
	if err != nil {
		return errors.New(sprintName(printN, "tx.Commit(): ", err))
	}

	printName(printN, "All tables removed from DB.")

	return nil
}

// AddToInfoDB uses subs[] to fill up the currently configured source table
// in the database. subs[] has to be filled prior to calling the function.
// This is usually achieved by fetching data from a source using it's API.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// subs - all substances of type DrugInfo to go through to add to source table
func (cfg Config) AddToInfoDB(db *sql.DB, ctx context.Context, errChannel chan error, subs []DrugInfo) {
	const printN string = "AddToInfoDB()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, err))
		return
	}

	stmt, err := tx.Prepare("insert into " + cfg.UseSource +
		" (drugName, drugRoute, " +
		"threshold, " +
		"lowDoseMin, lowDoseMax, " +
		"mediumDoseMin, mediumDoseMax, " +
		"highDoseMin, highDoseMax, " +
		"doseUnits, " +
		"onsetMin, onsetMax, onsetUnits, " +
		"comeUpMin, comeUpMax, comeUpUnits, " +
		"peakMin, peakMax, peakUnits, " +
		"offsetMin, offsetMax, offsetUnits, " +
		"totalDurMin, totalDurMax, totalDurUnits, " +
		"timeOfFetch) " +
		"values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "tx.Prepare(): ", err))
		return
	}

	defer stmt.Close()
	for i := 0; i < len(subs); i++ {
		subs[i].DoseUnits = cfg.MatchAndReplace(db, ctx, subs[i].DoseUnits, "units")
		_, err = stmt.Exec(subs[i].DrugName,
			subs[i].DrugRoute,
			subs[i].Threshold,
			subs[i].LowDoseMin,
			subs[i].LowDoseMax,
			subs[i].MediumDoseMin,
			subs[i].MediumDoseMax,
			subs[i].HighDoseMin,
			subs[i].HighDoseMax,
			subs[i].DoseUnits,
			subs[i].OnsetMin,
			subs[i].OnsetMax,
			subs[i].OnsetUnits,
			subs[i].ComeUpMin,
			subs[i].ComeUpMax,
			subs[i].ComeUpUnits,
			subs[i].PeakMin,
			subs[i].PeakMax,
			subs[i].PeakUnits,
			subs[i].OffsetMin,
			subs[i].OffsetMax,
			subs[i].OffsetUnits,
			subs[i].TotalDurMin,
			subs[i].TotalDurMax,
			subs[i].TotalDurUnits,
			time.Now().Unix())
		if err != nil {
			errChannel <- errors.New(sprintName(printN, "stmt.Exec():", err))
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "tx.Commit():", err))
		return
	}

	errChannel <- nil
}

// InitInfoDB creates the table for the currently configured source if it
// doesn't exist. It will use the source's name to set the table's name.
// It's called "Info", because it stores general information like dosages and
// timings for every route of administration.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) InitInfoDB(db *sql.DB, ctx context.Context) error {
	const printN string = "InitDrugDB()"

	ret := cfg.CheckDBTables(db, ctx, cfg.UseSource)
	if ret {
		return nil
	}

	caseInsensitive := " "
	if cfg.DBDriver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	initDBsql := "create table " + cfg.UseSource + " (drugName varchar(255)" + caseInsensitive + "not null," +
		"drugRoute varchar(255)" + caseInsensitive + "not null," +
		"threshold real," +
		"lowDoseMin real," +
		"lowDoseMax real," +
		"mediumDoseMin real," +
		"mediumDoseMax real," +
		"highDoseMin real," +
		"highDoseMax real," +
		"doseUnits text" + caseInsensitive + "," +
		"onsetMin real," +
		"onsetMax real," +
		"onsetUnits text" + caseInsensitive + "," +
		"comeUpMin real," +
		"comeUpMax real," +
		"comeUpUnits text" + caseInsensitive + "," +
		"peakMin real," +
		"peakMax real," +
		"peakUnits text" + caseInsensitive + "," +
		"offsetMin real," +
		"offsetMax real," +
		"offsetUnits text" + caseInsensitive + "," +
		"totalDurMin real," +
		"totalDurMax real," +
		"totalDurUnits text" + caseInsensitive + "," +
		"timeOfFetch bigint not null," +
		"primary key (drugName, drugRoute));"

	_, err := db.ExecContext(ctx, initDBsql)
	if err != nil {
		return errors.New(sprintName(printN, "db.ExecContext(): ", err))
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+cfg.UseSource+"' table for drug info in database.")

	return nil
}

// InitLogDB creates the table for all user drug logs if it doesn't exist.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) InitLogDB(db *sql.DB, ctx context.Context) error {
	const printN string = "InitLogDB()"

	ret := cfg.CheckDBTables(db, ctx, loggingTableName)
	if ret {
		return nil
	}

	caseInsensitive := " "
	if cfg.DBDriver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	initDBsql := "create table " + loggingTableName + " (timeOfDoseStart bigint not null," +
		"username varchar(255) not null," +
		"timeOfDoseEnd bigint not null," +
		"drugName text" + caseInsensitive + "not null," +
		"dose real not null," +
		"doseUnits text" + caseInsensitive + "not null," +
		"drugRoute text" + caseInsensitive + "not null," +
		"primary key (timeOfDoseStart, username));"

	_, err := db.ExecContext(ctx, initDBsql)
	if err != nil {
		return errors.New(sprintName(printN, "db.ExecContext(): ", err))
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: 'userLogs' table in database.")

	return nil
}

// InitUserSetDB creates the table for all user settings if it doesn't exist.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) InitUserSetDB(db *sql.DB, ctx context.Context) error {
	const printN string = "InitUserSetDB()"

	ret := cfg.CheckDBTables(db, ctx, userSetTableName)
	if ret {
		return nil
	}

	initDBsql := "create table " + userSetTableName + " (username varchar(255) not null," +
		"useIDForRemember bigint not null," +
		"primary key (username));"

	_, err := db.ExecContext(ctx, initDBsql)
	if err != nil {
		return errors.New(sprintName(printN, "db.ExecContext(): ", err))
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: 'userSettings' table in database.")

	return nil
}

// InitAltNamesDB creates all alternative names tables if they don't exist.
// Alternative names are names like "weed" instead of "cannabis" and etc.
// There are global tables which are used for any source. There are also source
// specific names which "replace" the global names.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// replace - if true, creates the tables for the currently configured source
// only, meaning for alt names specific to the source
func (cfg Config) InitAltNamesDB(db *sql.DB, ctx context.Context, replace bool) error {
	const printN string = "InitAltNamesDB()"

	tableSuffix := ""
	if replace {
		tableSuffix = "_" + cfg.UseSource
	}

	subsExists := false
	routesExists := false
	unitsExists := false
	convUnitsExists := false

	ret := cfg.CheckDBTables(db, ctx, altNamesSubsTableName+tableSuffix)
	if ret {
		subsExists = true
	}

	ret = cfg.CheckDBTables(db, ctx, altNamesRouteTableName+tableSuffix)
	if ret {
		routesExists = true
	}

	ret = cfg.CheckDBTables(db, ctx, altNamesUnitsTableName+tableSuffix)
	if ret {
		unitsExists = true
	}

	ret = cfg.CheckDBTables(db, ctx, altNamesConvUnitsTableName+tableSuffix)
	if ret {
		convUnitsExists = true
	}

	if subsExists && routesExists && unitsExists && convUnitsExists {
		return nil
	}

	caseInsensitive := " "
	if cfg.DBDriver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	var err error
	if !subsExists {
		initDBsql := "create table " + altNamesSubsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.ExecContext(ctx, initDBsql)
		if err != nil {
			return errors.New(sprintName(printN, "db.ExecContext(): ", err))
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesSubsTableName+tableSuffix+"' table in database.")
	}

	if !routesExists {
		initDBsql := "create table " + altNamesRouteTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.ExecContext(ctx, initDBsql)
		if err != nil {
			return errors.New(sprintName(printN, "db.ExecContext(): ", err))
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesRouteTableName+tableSuffix+"' table in database.")
	}

	if !unitsExists {
		initDBsql := "create table " + altNamesUnitsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.ExecContext(ctx, initDBsql)
		if err != nil {
			return errors.New(sprintName(printN, "db.ExecContext(): ", err))
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesUnitsTableName+tableSuffix+"' table in database.")
	}

	if !convUnitsExists {
		initDBsql := "create table " + altNamesConvUnitsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.ExecContext(ctx, initDBsql)
		if err != nil {
			return errors.New(sprintName(printN, "db.ExecContext(): ", err))
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesConvUnitsTableName+tableSuffix+"' table in database.")
	}

	return nil
}

// InitAllDBTables creates all tables needed to run the program properly.
// This function should be sufficient on it's own for most use cases.
// Even if the function is called every time the program is started, it should
// not be an issue, since all called functions first check if the tables they're
// trying to create already exist.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) InitAllDBTables(db *sql.DB, ctx context.Context) error {
	const printN string = "InitAllDBTables()"

	err := cfg.InitInfoDB(db, ctx)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	err = cfg.InitLogDB(db, ctx)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	err = cfg.InitUserSetDB(db, ctx)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	err = cfg.InitAltNamesDB(db, ctx, false)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	err = cfg.InitAltNamesDB(db, ctx, true)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Ran through all tables for initialisation.")

	return nil
}

// InitAllDB initializes the DB file structure if needed and all tables.
// It will stop the program if it encounters an error.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) InitAllDB(db *sql.DB, ctx context.Context) {
	const printN string = "InitAllDB()"

	if cfg.DBDriver == "sqlite3" {
		cfg.InitDBFileStructure()
	}

	err := cfg.InitAllDBTables(db, ctx)
	if err != nil {
		printName(printN, "Database didn't get initialised, because of an error, exiting: ", err)
		os.Exit(1)
	}

	if cfg.DBDriver != "mysql" && cfg.DBDriver != "sqlite3" {
		printName(printN, "No proper driver selected. Choose sqlite3 or mysql!")
		os.Exit(1)
	}
}

func (cfg Config) FetchFromSource(db *sql.DB, ctx context.Context,
	errChannel chan error, drugname string, client graphql.Client) {

	const printN string = "FetchFromSource()"

	gotsrcData := GetSourceData()
	printNameVerbose(cfg.VerbosePrinting, printN, "Using API from settings.toml:", cfg.UseSource)
	printNameVerbose(cfg.VerbosePrinting, printN, "Got API URL from sources.toml:", gotsrcData[cfg.UseSource].API_ADDRESS)

	if cfg.UseSource == "psychonautwiki" {
		errChannel2 := make(chan error)
		go cfg.FetchPsyWiki(db, ctx, errChannel2, drugname, client)
		err := <-errChannel2
		if err != nil {
			errChannel <- errors.New(sprintName(printN, "While fetching from: ", cfg.UseSource, " ; error: ", err))
			return
		}
	} else {
		errChannel <- errors.New(sprintName(printN, "No valid API selected:", cfg.UseSource))
		return
	}

	errChannel <- nil
}

func (cfg Config) AddToDoseDB(db *sql.DB, ctx context.Context, errChannel chan error,
	synct *SyncTimestamps, user string, drug string, route string,
	dose float32, units string, perc float32, printit bool) {

	const printN string = "AddToDoseDB()"

	drug = cfg.MatchAndReplace(db, ctx, drug, "substance")
	route = cfg.MatchAndReplace(db, ctx, route, "route")
	units = cfg.MatchAndReplace(db, ctx, units, "units")

	if perc != 0 {
		dose, units = cfg.ConvertUnits(db, ctx, drug, dose, perc)
		if dose == 0 || units == "" {
			errChannel <- errors.New(sprintfName(printN, "Error converting units for drug: %q"+
				" ; dose: %g ; perc: %g ; units: %q", drug, dose, perc, units))
			return
		}
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB(db, ctx,
		"drugName", cfg.UseSource,
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		xtrs[:], drug, route, units)
	if !ret {
		errChannel <- errors.New(sprintfName(printN, "Combo of Drug: %q"+
			" ; Route: %q"+
			" ; Units: %q"+
			" ; doesn't exist in local information database.", drug, route, units))
		return
	}

	var count uint32
	err, count := cfg.GetLogsCount(db, ctx, user)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, err))
		return
	}

	// get lock
	synct.Lock.Lock()

	if MaxLogsPerUserSize(count) >= cfg.MaxLogsPerUser {
		diff := count - uint32(cfg.MaxLogsPerUser)
		if cfg.AutoRemove {
			errChannel2 := make(chan error)
			go cfg.RemoveLogs(db, ctx, errChannel2, user, int(diff+1), true, 0, "none")
			gotErr := <-errChannel2
			if gotErr != nil {
				// release lock
				errChannel <- gotErr
				synct.Lock.Unlock()
				return
			}
		} else {
			errChannel <- errors.New(sprintName(printN, "User:", user,
				"has reached the maximum entries per user:", cfg.MaxLogsPerUser, "; Not logging."))
			return
		}
	}

	// release lock
	synct.Lock.Unlock()

	// Add to log db
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- err
		return
	}

	stmt, err := tx.Prepare("insert into " + loggingTableName +
		" (timeOfDoseStart, username, timeOfDoseEnd, drugName, dose, doseUnits, drugRoute) " +
		"values(?, ?, ?, ?, ?, ?, ?)")
	if handleErrRollback(err, tx, errChannel) {
		return
	}
	defer stmt.Close()

	// get lock
	synct.Lock.Lock()

	currTime := time.Now().Unix()
	if currTime == synct.LastTimestamp && user == synct.LastUser {
		time.Sleep(time.Second)
		currTime = time.Now().Unix()
	}

	_, err = stmt.Exec(currTime, user, 0, drug, dose, units, route)
	if handleErrRollback(err, tx, errChannel) {
		return
	}
	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel) {
		return
	}

	synct.LastTimestamp = currTime
	synct.LastUser = user

	// release lock
	synct.Lock.Unlock()

	if printit {
		printNameF(printN, "Logged: drug: %q ; dose: %g ; units: %q ; route: %q ; username: %q\n",
			drug, dose, units, route, user)
	}

	errChannel <- nil
}

// GetDBSize returns the total size of the database in bytes.
func (cfg Config) GetDBSize() int64 {
	const printN string = "GetDBSize()"

	if cfg.DBDriver == "sqlite3" {
		file, err := os.Open(cfg.DBSettings[cfg.DBDriver].Path)
		if err != nil {
			printName(printN, "Error opening:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		fileInfo, err := file.Stat()
		if err != nil {
			printName(printN, "Error getting stat:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		err = file.Close()
		if err != nil {
			printName(printN, "Error closing file:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		return fileInfo.Size()
	} else if cfg.DBDriver == "mysql" {
		db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
		if err != nil {
			errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
		}
		defer db.Close()

		res := strings.Split(cfg.DBSettings[cfg.DBDriver].Path, "/")
		dbName := res[1]

		dbSizeQuery := "select SUM(data_length + index_length) FROM information_schema.tables " +
			"where table_schema = ?"

		var totalSize int64

		row := db.QueryRow(dbSizeQuery, dbName)
		err = row.Scan(&totalSize)
		if err != nil {
			printName(printN, "Error getting size:", err)
			return 0
		}

		return totalSize
	}

	printName(printN, "The chosen driver is not a proper one:", cfg.DBDriver)
	return 0
}

// GetUsers returns all unique usernames
// currently present in the drug log table.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) GetUsers(db *sql.DB, ctx context.Context) []string {
	const printN string = "GetUsers()"

	var allUsers []string
	var tempUser string

	rows, err := db.QueryContext(ctx, "select distinct username from "+loggingTableName)
	if err != nil {
		printName(printN, "Query: error getting usernames:", err)
		return nil
	}

	for rows.Next() {
		err = rows.Scan(&tempUser)
		if err != nil {
			printName(printN, "Scan: error getting usernames:", err)
			return nil
		}
		allUsers = append(allUsers, tempUser)
	}

	return allUsers
}

// GetLogsCount returns total amount of logs in
// the drug log table for username set in user parameter.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// user - user to get log count for
func (cfg Config) GetLogsCount(db *sql.DB, ctx context.Context, user string) (error, uint32) {
	const printN string = "GetLogsCount()"

	var count uint32

	row := db.QueryRowContext(ctx, "select count(*) from "+loggingTableName+" where username = ?", user)
	err := row.Scan(&count)
	if err != nil {
		err = errors.New(sprintName(printN, "Error getting count:", err))
		return err, 0
	}

	return nil, count
}

// db - an open database connection
//
// userLogsErrorChannel - the goroutine channel used to return the logs and
// the error
//
// ctx - context that will be passed to the sql query function
//
// num - amount of logs to return (limit), if 0 returns all logs (without limit)
//
// id - if not 0, will return the exact log matching that id for the given user
//
// user - the user which owns the logs
//
// reverse - if true go from high values to low,
// this should return the newest logs first
//
// search - return logs only matching this string
func (cfg Config) GetLogs(db *sql.DB, userLogsErrorChannel chan UserLogsError,
	ctx context.Context, num int, id int64, user string, reverse bool,
	search string) {

	printN := "GetLogs()"

	numstr := strconv.Itoa(num)

	tempUserLogsError := UserLogsError{
		Err: nil,
	}

	var endstmt string
	if num == 0 {
		endstmt = ""
	} else {
		endstmt = " limit " + numstr
	}

	orientation := "asc"
	if reverse {
		orientation = "desc"
	}

	searchStmt := ""
	var searchArr []any
	if search != "none" && search != "" {
		search = cfg.MatchAndReplaceAll(db, ctx, search)
		searchColumns := []string{"drugName",
			"dose",
			"doseUnits",
			"drugRoute"}
		searchArr = append(searchArr, user)
		searchStmt += "and " + searchColumns[0] + " like ? "
		searchArr = append(searchArr, "%"+search+"%")
		for i := 1; i < len(searchColumns); i++ {
			searchStmt += "or " + searchColumns[i] + " like ? "
			searchArr = append(searchArr, "%"+search+"%")
		}
	}

	mainQuery := "select * from " + loggingTableName + " where username = ? " + searchStmt +
		"order by timeOfDoseStart " + orientation + endstmt
	var rows *sql.Rows
	var err error
	if id == 0 {
		if search == "none" || search == "" {
			rows, err = db.QueryContext(ctx, mainQuery, user)
		} else {
			rows, err = db.QueryContext(ctx, mainQuery, searchArr...)
		}
	} else {
		rows, err = db.QueryContext(ctx, "select * from "+loggingTableName+" where username = ? and timeOfDoseStart = ?", user, id)
	}
	if err != nil {
		tempUserLogsError.Err = err
		tempUserLogsError.UserLogs = nil
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	defer rows.Close()

	userlogs := []UserLog{}
	for rows.Next() {
		tempul := UserLog{}
		err = rows.Scan(&tempul.StartTime, &tempul.Username, &tempul.EndTime, &tempul.DrugName,
			&tempul.Dose, &tempul.DoseUnits, &tempul.DrugRoute)
		if err != nil {
			tempUserLogsError.Err = err
			tempUserLogsError.UserLogs = nil
			userLogsErrorChannel <- tempUserLogsError
			return
		}

		userlogs = append(userlogs, tempul)
	}
	err = rows.Err()
	if err != nil {
		tempUserLogsError.Err = err
		tempUserLogsError.UserLogs = nil
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	if len(userlogs) == 0 {
		tempUserLogsError.Err = errors.New(sprintName(printN, "No logs returned for user: ", user))
		tempUserLogsError.UserLogs = nil
		userLogsErrorChannel <- tempUserLogsError
		return
	}

	tempUserLogsError.Err = nil
	tempUserLogsError.UserLogs = userlogs
	userLogsErrorChannel <- tempUserLogsError
}

// PrintLogs writes all logs present in userLogs to console.
//
// userLogs - the logs slice returned from GetLogs()
//
// prefix - if true the name of the function should be shown
// when writing to console
func (cfg Config) PrintLogs(userLogs []UserLog, prefix bool) {
	var printN string
	if prefix == true {
		printN = "GetLogs()"
	} else {
		printN = ""
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		printName(printN, "LoadLocation:", err)
		return
	}

	for _, elem := range userLogs {
		printNameF(printN, "Start:\t%q (%d) < ID\n",
			time.Unix(int64(elem.StartTime), 0).In(location), elem.StartTime)
		if elem.EndTime != 0 {
			printNameF(printN, "End:\t%q (%d)\n",
				time.Unix(int64(elem.EndTime), 0).In(location), elem.EndTime)
		}
		printNameF(printN, "Drug:\t%q\n", elem.DrugName)
		printNameF(printN, "Dose:\t%g\n", elem.Dose)
		printNameF(printN, "Units:\t%q\n", elem.DoseUnits)
		printNameF(printN, "Route:\t%q\n", elem.DrugRoute)
		printNameF(printN, "User:\t%q\n", elem.Username)
		printName(printN, "=========================")
	}
}

// GetLocalInfoNames returns a slice containing all unique names of drugs
// present in the local database gotten from a source.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) GetLocalInfoNames(db *sql.DB, ctx context.Context) []string {
	const printN string = "GetLocalInfoNames()"

	rows, err := db.QueryContext(ctx, "select distinct drugName from "+cfg.UseSource)
	if err != nil {
		printName(printN, err)
		return nil
	}
	defer rows.Close()

	var drugList []string
	for rows.Next() {
		var holdName string
		err := rows.Scan(&holdName)
		if err != nil {
			printName(printN, err)
			return nil
		}

		drugList = append(drugList, holdName)
	}
	err = rows.Err()
	if err != nil {
		printName(printN, err)
		return nil
	}

	return drugList
}

// GetLocalInfo returns a slice containing all information about a drug.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// drug - drug to get information about
func (cfg Config) GetLocalInfo(db *sql.DB, ctx context.Context,
	drug string) []DrugInfo {
	printN := "GerLocalInfo()"

	drug = cfg.MatchAndReplace(db, ctx, drug, "substance")

	ret := checkIfExistsDB(db, ctx,
		"drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		printName(printN, "No such drug in info database:", drug)
		return nil
	}

	rows, err := db.QueryContext(ctx, "select * from "+cfg.UseSource+" where drugName = ?", drug)
	if err != nil {
		printName(printN, err)
		return nil
	}
	defer rows.Close()
	infoDrug := []DrugInfo{}
	for rows.Next() {
		tempdrinfo := DrugInfo{}
		err := rows.Scan(&tempdrinfo.DrugName, &tempdrinfo.DrugRoute,
			&tempdrinfo.Threshold,
			&tempdrinfo.LowDoseMin, &tempdrinfo.LowDoseMax, &tempdrinfo.MediumDoseMin,
			&tempdrinfo.MediumDoseMax, &tempdrinfo.HighDoseMin, &tempdrinfo.HighDoseMax,
			&tempdrinfo.DoseUnits, &tempdrinfo.OnsetMin, &tempdrinfo.OnsetMax,
			&tempdrinfo.OnsetUnits, &tempdrinfo.ComeUpMin, &tempdrinfo.ComeUpMax,
			&tempdrinfo.ComeUpUnits, &tempdrinfo.PeakMin, &tempdrinfo.PeakMax,
			&tempdrinfo.PeakUnits, &tempdrinfo.OffsetMin, &tempdrinfo.OffsetMax,
			&tempdrinfo.OffsetUnits, &tempdrinfo.TotalDurMin, &tempdrinfo.TotalDurMax,
			&tempdrinfo.TotalDurUnits, &tempdrinfo.TimeOfFetch)
		if err != nil {
			printName(printN, err)
			return nil
		}

		infoDrug = append(infoDrug, tempdrinfo)
	}
	err = rows.Err()
	if err != nil {
		printName(printN, err)
		return nil
	}

	return infoDrug
}

// PrintLocalInfo prints the information gotten from the source, present in the
// local database.
//
// drugInfo - slice returned from GetLocalInfo()
//
// prefix - whether to add the function name to console output
func (cfg Config) PrintLocalInfo(drugInfo []DrugInfo, prefix bool) {
	var printN string
	if prefix == true {
		printN = "GetLocalInfo()"
	} else {
		printN = ""
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		printName(printN, err)
		return
	}

	for _, elem := range drugInfo {
		printName(printN, "Source:", cfg.UseSource)
		printName(printN, "Drug:", elem.DrugName, ";", "Route:", elem.DrugRoute)
		printName(printN, "---Dosages---")
		printNameF(printN, "Threshold: %g\n", elem.Threshold)
		printName(printN, "Min\tMax\tRange")
		printNameF(printN, "%g\t%g\tLow\n", elem.LowDoseMin, elem.LowDoseMax)
		printNameF(printN, "%g\t%g\tMedium\n", elem.MediumDoseMin, elem.MediumDoseMax)
		printNameF(printN, "%g\t%g\tHigh\n", elem.HighDoseMin, elem.HighDoseMax)
		printName(printN, "Dose units:", elem.DoseUnits)
		printName(printN, "---Times---")
		printName(printN, "Min\tMax\tPeriod\tUnits")
		printNameF(printN, "%g\t%g\tOnset\t%q\n",
			elem.OnsetMin,
			elem.OnsetMax,
			elem.OnsetUnits)
		printNameF(printN, "%g\t%g\tComeup\t%q\n",
			elem.ComeUpMin,
			elem.ComeUpMax,
			elem.ComeUpUnits)
		printNameF(printN, "%g\t%g\tPeak\t%q\n",
			elem.PeakMin,
			elem.PeakMax,
			elem.PeakUnits)
		printNameF(printN, "%g\t%g\tOffset\t%q\n",
			elem.OffsetMin,
			elem.OffsetMax,
			elem.OffsetUnits)
		printNameF(printN, "%g\t%g\tTotal\t%q\n",
			elem.TotalDurMin,
			elem.TotalDurMax,
			elem.TotalDurUnits)
		printName(printN, "Time of fetch:", time.Unix(int64(elem.TimeOfFetch), 0).In(location))
		printName(printN, "====================")
	}
}

func (cfg Config) RemoveLogs(db *sql.DB, ctx context.Context, errChannel chan error,
	username string, amount int, reverse bool, remID int64, search string) {

	const printN string = "RemoveLogs()"

	stmtStr := "delete from " + loggingTableName + " where username = ?"
	if amount != 0 && remID == 0 || search != "none" {
		if search != "none" && search != "" {
			amount = 0
		}

		userLogsErrChan := make(chan UserLogsError)
		go cfg.GetLogs(db, userLogsErrChan, ctx, amount, 0, username, reverse, search)
		gotLogs := <-userLogsErrChan
		if gotLogs.Err != nil {
			errChannel <- gotLogs.Err
			return
		}

		var gotTimeOfDose []int64
		var tempTimes int64

		for i := 0; i < len(gotLogs.UserLogs); i++ {
			tempTimes = gotLogs.UserLogs[i].StartTime
			gotTimeOfDose = append(gotTimeOfDose, tempTimes)
		}

		concatTimes := ""
		for i := 0; i < len(gotTimeOfDose); i++ {
			concatTimes = concatTimes + strconv.FormatInt(gotTimeOfDose[i], 10) + ","
		}
		concatTimes = strings.TrimSuffix(concatTimes, ",")

		stmtStr = "delete from " + loggingTableName + " where timeOfDoseStart in (" + concatTimes + ") AND username = ?"
	} else if remID != 0 && search == "none" {
		xtrs := [1]string{xtrastmt("username", "and")}
		ret := checkIfExistsDB(db, ctx,
			"timeOfDoseStart", loggingTableName,
			cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
			xtrs[:], remID, username)
		if !ret {
			errChannel <- errors.New(sprintName(printN, "Log with ID:", remID, "doesn't exists."))
			return
		}

		stmtStr = "delete from " + loggingTableName + " where timeOfDoseStart = ? AND username = ?"
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- err
		return
	}

	stmt, err := tx.Prepare(stmtStr)
	if handleErrRollback(err, tx, errChannel) {
		return
	}
	defer stmt.Close()
	if remID != 0 {
		_, err = stmt.Exec(remID, username)
	} else {
		_, err = stmt.Exec(username)
	}
	if handleErrRollback(err, tx, errChannel) {
		return
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel) {
		return
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Data removed from log table in DB successfully.")

	errChannel <- nil
}

func (cfg Config) SetUserLogs(db *sql.DB, ctx context.Context, set string, id int64, username string, setValue string) bool {
	const printN string = "SetUserLogs()"

	if username == "none" {
		printName(printN, "Please specify an username!")
		return false
	}

	if set == "none" {
		printName(printN, "Please specify a set type!")
		return false
	}

	if setValue == "none" {
		printName(printN, "Please specify a value to set!")
		return false
	}

	if setValue == "now" && set == "start-time" || setValue == "now" && set == "end-time" {
		setValue = strconv.FormatInt(time.Now().Unix(), 10)
	}

	if set == "start-time" || set == "end-time" {
		if _, err := strconv.ParseInt(setValue, 10, 64); err != nil {
			printName(printN, "Error when checking if integer:", err)
			return false
		}
	}

	if set == "dose" {
		if _, err := strconv.ParseFloat(setValue, 64); err != nil {
			printName(printN, "Error when checking if float:", err)
			return false
		}
	}

	setName := map[string]string{
		"start-time": "timeOfDoseStart",
		"end-time":   "timeOfDoseEnd",
		"drug":       "drugName",
		"dose":       "dose",
		"units":      "doseUnits",
		"route":      "drugRoute",
	}

	userLogsErrChan := make(chan UserLogsError)
	var gotLogs []UserLog
	var gotErr error

	if id == 0 {
		go cfg.GetLogs(db, userLogsErrChan, ctx, 1, 0, username, true, "")
		gotUserLogsErr := <-userLogsErrChan
		gotErr = gotUserLogsErr.Err
		if gotErr != nil {
			printName(printN, gotErr)
			return false
		}

		gotLogs = gotUserLogsErr.UserLogs
		id = gotLogs[0].StartTime
	} else {
		go cfg.GetLogs(db, userLogsErrChan, ctx, 1, id, username, true, "")
		gotUserLogsErr := <-userLogsErrChan
		gotErr = gotUserLogsErr.Err
		if gotErr != nil {
			printName(printN, gotErr)
			return false
		}
		gotLogs = gotUserLogsErr.UserLogs
	}

	stmtStr := fmt.Sprintf("update "+loggingTableName+" set %s = ? where timeOfDoseStart = ?",
		setName[set])

	tx, err := db.Begin()
	if err != nil {
		printName(printN, "db.Begin():", err)
		return false
	}

	stmt, err := tx.PrepareContext(ctx, stmtStr)
	if err != nil {
		printName(printN, "tx.Prepare():", err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, setValue, id)
	if err != nil {
		printName(printN, "stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, "entry:", id, "; changed:", set, "; to value:", setValue)

	return true
}

// InitUserSettings creates the row with default settings for a user.
// These settings are kept in the database and are not global like the config
// files. All users have their own settings.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// username - the user to create default settings for
func (cfg Config) InitUserSettings(db *sql.DB, ctx context.Context, username string) bool {
	const printN string = "InitUserSettings()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		printName(printN, err)
		return false
	}

	stmt, err := tx.Prepare("insert into userSettings" +
		" (username, useIDForRemember) " +
		"values(?, ?)")
	if err != nil {
		printName(printN, err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(username, ForgetInputConfigMagicNumber)
	if err != nil {
		printName(printN, err)
		return false
	}
	err = tx.Commit()
	if err != nil {
		printName(printN, err)
		return false
	}

	printName(printN, "User settings initialized successfully!")

	return true
}

func (cfg Config) SetUserSettings(db *sql.DB, ctx context.Context, set string,
	username string, setValue string) bool {

	const printN string = "SetUserSettings()"

	ret := checkIfExistsDB(db, ctx,
		"username", "userSettings",
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		nil, username)
	if ret == false {
		cfg.InitUserSettings(db, ctx, username)
	}

	if username == "none" {
		printName(printN, "Please specify an username!")
		return false
	}

	if set == "none" {
		printName(printN, "Please specify a set type!")
		return false
	}

	if setValue == "none" {
		printName(printN, "Please specify a value to set!")
		return false
	}

	if set == "useIDForRemember" {
		if setValue == "remember" {
			userLogsErrChan := make(chan UserLogsError)
			go cfg.GetLogs(db, userLogsErrChan, ctx, 1, 0, username, true, "none")
			gotLogs := <-userLogsErrChan
			if gotLogs.Err != nil {
				printName(printN, gotLogs.Err)
				return false
			}

			setValue = strconv.FormatInt(gotLogs.UserLogs[0].StartTime, 10)
		} else {
			if _, err := strconv.ParseInt(setValue, 10, 64); err != nil {
				printName(printN, "Error when checking if integer:", setValue, ":", err)
				return false
			}
		}
	}

	stmtStr := fmt.Sprintf("update userSettings set %s = ? where username = ?", set)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		printName(printN, "db.Begin():", err)
		return false
	}

	stmt, err := tx.Prepare(stmtStr)
	if err != nil {
		printName(printN, "tx.Prepare():", err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(setValue, username)

	if err != nil {
		printName(printN, "stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, set+": setting changed to:", setValue)

	return true
}

func (cfg Config) GetUserSettings(db *sql.DB, ctx context.Context,
	set string, username string) string {

	const printN string = "GetUserSettings()"

	fmtStmt := fmt.Sprintf("select %s from userSettings where username = ?", set)
	stmt, err := db.PrepareContext(ctx, fmtStmt)
	if err != nil {
		printName(printN, "SQL error in prepare:", err)
		return ""
	}
	defer stmt.Close()

	var got string
	err = stmt.QueryRowContext(ctx, username).Scan(&got)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		printName(printN, "Received weird error:", err)
		return ""
	}

	return got
}

func (cfg Config) RememberConfig(db *sql.DB, ctx context.Context, username string) *UserLog {
	const printN string = "RememberConfig()"

	got := cfg.GetUserSettings(db, ctx, "useIDForRemember", username)
	if got == "" {
		printName(printN, "Couldn't get setting value: useIDForRemember")
		return nil
	}

	gotInt, err := strconv.ParseInt(got, 10, 64)
	if err != nil {
		printName(printN, "Couldn't convert:", got, "; to integer:", err)
		return nil
	}

	userLogsErrChan := make(chan UserLogsError)
	go cfg.GetLogs(db, userLogsErrChan, ctx, 1, gotInt, username, false, "")
	gotLogs := <-userLogsErrChan
	if gotLogs.Err != nil {
		printName(printN, gotLogs.Err)
		return nil
	}

	return &gotLogs.UserLogs[0]
}

func (cfg Config) ForgetConfig(db *sql.DB, ctx context.Context, username string) bool {
	const printN string = "ForgetConfig()"

	ret := cfg.SetUserSettings(db, ctx, "useIDForRemember", username, ForgetInputConfigMagicNumber)
	if ret == false {
		printName(printN, "Couldn't set setting: useIDForRemember")
		return false
	}

	return true
}
