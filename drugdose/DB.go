package drugdose

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"database/sql"

	// MySQL driver needed for sql module

	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

// Encryption should be done by default unless specified not to by the user from the settings
// But first the official implementation for encryption has to be done in the sqlite module

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
const ForgetInputConfigMagicNumber string = "9999999999"

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

type UserLog struct {
	StartTime int64
	Username  string
	EndTime   int64
	DrugName  string
	Dose      float32
	DoseUnits string
	DrugRoute string
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

func xtrastmt(col string, logical string) string {
	return logical + " " + col + " = ?"
}

func checkIfExistsDB(col string, table string, driver string,
	path string, xtrastmt []string, values ...interface{}) bool {

	const printN string = "checkIfExistsDB()"

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	stmtstr := "select " + col + " from " + table + " where " + col + " = ?"
	if xtrastmt != nil {
		for i := 0; i < len(xtrastmt); i++ {
			stmtstr = stmtstr + " " + xtrastmt[i]
		}
	}

	// NOTE: this doesn't cause an SQL injection, because we're not taking 'col' and 'table' from an user input.
	stmt, err := db.Prepare(stmtstr)
	if err != nil {
		printName(printN, "SQL error in prepare for check if exists:", err)
		return false
	}
	defer stmt.Close()
	var got string

	err = stmt.QueryRow(values...).Scan(&got)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		printName(printN, "received weird error:", err)
		return false
	}

	return true
}

// InitDBFileStructure creates the basic file structure for the database.
// This should be run only once!
func (cfg Config) InitDBFileStructure() bool {
	const printN string = "InitDBFileStructure()"

	ret := cfg.checkDBFileStruct()
	if ret == true {
		return true
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

	return true
}

// checkDBFileStruct Returns true if the file structure is already created,
// false otherwise. Checks whether the db directory and minimum amount of files
// exist with the proper names in it.
func (cfg Config) checkDBFileStruct() bool {
	const printN string = "checkDBFileStruct()"

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

// RemoveSingleDrugInfoDB Remove all entries of a single drug from the local info DB, instead of deleting the whole DB.
func (cfg Config) RemoveSingleDrugInfoDB(drug string) bool {
	const printN string = "RemoveSingleDrugInfoDB()"

	drug = cfg.MatchAndReplace(drug, "substance")

	ret := checkIfExistsDB("drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		printName(printN, "No such drug in info database:", drug)
		return false
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	stmt, err := tx.Prepare("delete from " + cfg.UseSource +
		" where drugName = ?")
	if err != nil {
		printName(printN, "tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	_, err = stmt.Exec(drug)
	if err != nil {
		printName(printN, "stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, "Data removed from info DB successfully.")

	return true
}

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

func (cfg Config) CheckDBTables(tableName string) bool {
	const printN string = "CheckDBTables()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	queryStr := cfg.getTableNamesQuery(tableName)
	rows, err := db.Query(queryStr)
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

func (cfg Config) CleanDB() bool {
	const printN string = "CleanDB()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	queryStr := cfg.getTableNamesQuery("")
	rows, err := db.Query(queryStr)
	if err != nil {
		printName(printN, err)
		return false
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	if cfg.VerbosePrinting == true {
		printNameNoNewline(printN, "Removing tables: ")
	}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			printName(printN, err)
			return false
		}

		if cfg.VerbosePrinting == true {
			fmt.Print(name + ", ")
		}

		_, err = tx.Exec("drop table " + name)
		if err != nil {
			fmt.Println()
			printName(printN, "tx.Exec():", err)
			return false
		}
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	if cfg.VerbosePrinting == true {
		fmt.Println()
	}
	printName(printN, "All tables removed from DB.")

	return true
}

// Removes the currently configured info table.
func (cfg Config) CleanInfo() bool {
	const printN string = "CleanInfo()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	_, err = tx.Exec("drop table " + cfg.UseSource)
	if err != nil {
		printName(printN, "tx.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, "The info table: "+cfg.UseSource+"; removed from DB.")

	return true
}

// Removes the main names tables and the currently configured ones as well.
// This means, that any old names generated for another source aren't removed.
func (cfg Config) CleanNames() bool {
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

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	printNameNoNewline(printN, "Removing tables: ")
	for i := 0; i < len(tableNames); i++ {
		fmt.Print(tableNames[i] + ", ")

		_, err = tx.Exec("drop table " + tableNames[i])
		if err != nil {
			fmt.Println()
			printName(printN, "tx.Exec():", err)
			return false
		}
	}
	fmt.Println()

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, "All tables removed from DB.")

	return true
}

func (cfg Config) AddToInfoDB(subs []DrugInfo) bool {
	const printN string = "AddToInfoDB()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
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
		printName(printN, "tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	for i := 0; i < len(subs); i++ {
		subs[i].DoseUnits = cfg.MatchAndReplace(subs[i].DoseUnits, "units")
		_, err = stmt.Exec(
			subs[i].DrugName,
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
			printName(printN, "stmt.Exec():", err)
			return false
		}
	}
	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	return true
}

func (cfg Config) InitDrugDB() bool {
	const printN string = "InitDrugDB()"

	ret := cfg.CheckDBTables(cfg.UseSource)
	if ret {
		return true
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

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

	_, err = db.Exec(initDBsql)
	if err != nil {
		printName(printN, initDBsql+":", err)
		return false
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+cfg.UseSource+"' table for drug info in database.")

	return true
}

func (cfg Config) InitLogDB() bool {
	const printN string = "InitLogDB()"

	ret := cfg.CheckDBTables(loggingTableName)
	if ret {
		return true
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

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

	_, err = db.Exec(initDBsql)
	if err != nil {
		printName(printN, initDBsql+":", err)
		return false
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: 'userLogs' table in database.")

	return true
}

func (cfg Config) InitUserSetDB() bool {
	const printN string = "InitUserSetDB()"

	ret := cfg.CheckDBTables(userSetTableName)
	if ret {
		return true
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	initDBsql := "create table " + userSetTableName + " (username varchar(255) not null," +
		"useIDForRemember bigint not null," +
		"primary key (username));"

	_, err = db.Exec(initDBsql)
	if err != nil {
		printName(printN, initDBsql+":", err)
		return false
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Created: 'userSettings' table in database.")

	return true
}

func (cfg Config) InitAltNamesDB(replace bool) bool {
	const printN string = "InitAltNamesDB()"

	tableSuffix := ""
	if replace {
		tableSuffix = "_" + cfg.UseSource
	}

	subsExists := false
	routesExists := false
	unitsExists := false
	convUnitsExists := false

	ret := cfg.CheckDBTables(altNamesSubsTableName + tableSuffix)
	if ret {
		subsExists = true
	}

	ret = cfg.CheckDBTables(altNamesRouteTableName + tableSuffix)
	if ret {
		routesExists = true
	}

	ret = cfg.CheckDBTables(altNamesUnitsTableName + tableSuffix)
	if ret {
		unitsExists = true
	}

	ret = cfg.CheckDBTables(altNamesConvUnitsTableName + tableSuffix)
	if ret {
		convUnitsExists = true
	}

	if subsExists && routesExists && unitsExists && convUnitsExists {
		return true
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	caseInsensitive := " "
	if cfg.DBDriver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	if !subsExists {
		initDBsql := "create table " + altNamesSubsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.Exec(initDBsql)
		if err != nil {
			printName(printN, initDBsql+":", err)
			return false
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesSubsTableName+tableSuffix+"' table in database.")
	}

	if !routesExists {
		initDBsql := "create table " + altNamesRouteTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.Exec(initDBsql)
		if err != nil {
			printName(printN, initDBsql+":", err)
			return false
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesRouteTableName+tableSuffix+"' table in database.")
	}

	if !unitsExists {
		initDBsql := "create table " + altNamesUnitsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.Exec(initDBsql)
		if err != nil {
			printName(printN, initDBsql+":", err)
			return false
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesUnitsTableName+tableSuffix+"' table in database.")
	}

	if !convUnitsExists {
		initDBsql := "create table " + altNamesConvUnitsTableName + tableSuffix +
			" (localName varchar(255)" + caseInsensitive + "not null," +
			"alternativeName varchar(255)" + caseInsensitive + "not null," +
			"primary key (localName, alternativeName));"

		_, err = db.Exec(initDBsql)
		if err != nil {
			printName(printN, initDBsql+":", err)
			return false
		}

		printNameVerbose(cfg.VerbosePrinting, printN, "Created: '"+altNamesConvUnitsTableName+tableSuffix+"' table in database.")
	}

	return true
}

func (cfg Config) InitAllDBTables() bool {
	const printN string = "InitAllDBTables()"

	ret := cfg.InitDrugDB()
	if !ret {
		return false
	}

	ret = cfg.InitLogDB()
	if !ret {
		return false
	}

	ret = cfg.InitUserSetDB()
	if !ret {
		return false
	}

	ret = cfg.InitAltNamesDB(false)
	if !ret {
		return false
	}

	ret = cfg.InitAltNamesDB(true)
	if !ret {
		return false
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Ran through all tables for initialisation.")

	return true
}

func (cfg Config) AddToDoseDB(user string, drug string, route string,
	dose float32, units string, perc float32, printit bool) bool {

	const printN string = "AddToDoseDB()"

	drug = cfg.MatchAndReplace(drug, "substance")
	route = cfg.MatchAndReplace(route, "route")
	units = cfg.MatchAndReplace(units, "units")

	if perc != 0 {
		dose, units = cfg.ConvertUnits(drug, dose, perc)
		if dose == 0 || units == "" {
			printName(printN, "Error converting units for drug:", drug,
				"; dose:", dose, "; perc:", perc, "; units:", units)
			return false
		}
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB("drugName", cfg.UseSource,
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		xtrs[:], drug, route, units)
	if !ret {
		printName(printN, "Combo of Drug("+drug+
			"), Route("+route+
			") and Units("+units+
			") doesn't exist in local information database.")
		return false
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("select count(*) from "+loggingTableName+" where username = ?", user).Scan(&count)
	if err != nil {
		printName(printN, "Error when counting user logs for user:", user)
		printName(printN, err)
		return false
	}

	if MaxLogsPerUserSize(count) >= cfg.MaxLogsPerUser {
		diff := count - int(cfg.MaxLogsPerUser)
		if cfg.AutoRemove {
			cfg.RemoveLogs(user, diff+1, true, 0, "none")
		} else {
			printName(printN, "User:", user, "has reached the maximum entries per user:", cfg.MaxLogsPerUser,
				"; Not logging.")
			return false
		}
	}

	// Add to log db
	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	stmt, err := tx.Prepare("insert into " + loggingTableName +
		" (timeOfDoseStart, username, timeOfDoseEnd, drugName, dose, doseUnits, drugRoute) " +
		"values(?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		printName(printN, err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now().Unix(), user, 0, drug, dose, units, route)
	if err != nil {
		printName(printN, err)
		return false
	}
	err = tx.Commit()
	if err != nil {
		printName(printN, err)
		return false
	}

	if printit {
		printNameF(printN, "Logged: drug: %q ; dose: %g ; units: %q ; route: %q ; username: %q\n",
			drug, dose, units, route, user)
		printName(printN, "Dose logged successfully!")
	}

	return true
}

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

func (cfg Config) GetUsers() []string {
	const printN string = "GetUsers()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var allUsers []string
	var tempUser string

	rows, err := db.Query("select distinct username from " + loggingTableName)
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

func (cfg Config) GetLogsCount(user string) int {
	const printN string = "GetLogsCount()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var count int

	row := db.QueryRow("select count(*) from "+loggingTableName+" where username = ?", user)
	err = row.Scan(&count)
	if err != nil {
		printName(printN, "Error getting count:", err)
		return 0
	}

	return count
}

// outputChannel - the goroutine channel used to return the logs
//
// num - amount of logs to return (limit)
//
// id - if not 0, will return the exact log matching that id for the given user
//
// user - the user which owns the logs
//
// all - ignores num and returns all logs if id is not set
//
// reverse - go from high values to low
//
// printit - print the logs
//
// search - return logs only matching this string
//
// prefix - whether the name of the function should be shown when writing to console
func (cfg Config) GetLogs(outputChannel chan []UserLog, num int, id int64,
	user string, all bool, reverse bool, printit bool,
	search string, prefix bool) {

	var printN string
	if prefix == true {
		printN = "GetLogs()"
	} else {
		printN = ""
	}

	numstr := strconv.Itoa(num)

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var endstmt string
	if all {
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
		search = cfg.MatchAndReplaceAll(search)
		searchColumns := []string{"drugName",
			"dose",
			"doseUnits",
			"drugRoute"}
		searchArr = append(searchArr, user)
		searchStmt += "and " + searchColumns[0] + " like ? "
		searchArr = append(searchArr, "%" + search + "%")
		for i := 1; i < len(searchColumns); i++ {
			searchStmt += "or " + searchColumns[i] + " like ? "
			searchArr = append(searchArr, "%" + search + "%")
		}
	}

	mainQuery := "select * from "+loggingTableName+" where username = ? "+searchStmt+
		"order by timeOfDoseStart "+orientation+endstmt
	var rows *sql.Rows
	if id == 0 {
		if search == "none" || search == "" {
			rows, err = db.Query(mainQuery, user)
		} else {
			rows, err = db.Query(mainQuery, searchArr...)
		}
	} else {
		rows, err = db.Query("select * from "+loggingTableName+" where username = ? and timeOfDoseStart = ?", user, id)
	}
	if err != nil {
		printName(printN, "Query:", err)
		outputChannel <- nil
		return
	}
	defer rows.Close()

	userlogs := []UserLog{}
	for rows.Next() {
		tempul := UserLog{}
		err = rows.Scan(&tempul.StartTime, &tempul.Username, &tempul.EndTime, &tempul.DrugName,
			&tempul.Dose, &tempul.DoseUnits, &tempul.DrugRoute)
		if err != nil {
			printName(printN, "Scan:", err)
			outputChannel <- nil
			return
		}

		location, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			printName(printN, "LoadLocation:", err)
			outputChannel <- nil
			return
		}

		if printit {
			printNameF(printN, "Start:\t%q (%d) < ID\n",
				time.Unix(int64(tempul.StartTime), 0).In(location), tempul.StartTime)
			if tempul.EndTime != 0 {
				printNameF(printN, "End:\t%q (%d)\n",
					time.Unix(int64(tempul.EndTime), 0).In(location), tempul.EndTime)
			}
			printNameF(printN, "Drug:\t%q\n", tempul.DrugName)
			printNameF(printN, "Dose:\t%g\n", tempul.Dose)
			printNameF(printN, "Units:\t%q\n", tempul.DoseUnits)
			printNameF(printN, "Route:\t%q\n", tempul.DrugRoute)
			printNameF(printN, "User:\t%q\n", tempul.Username)
			printName(printN, "=========================")
		}

		userlogs = append(userlogs, tempul)
	}
	err = rows.Err()
	if err != nil {
		printName(printN, "rows.Err():", err)
		outputChannel <- nil
		return
	}
	if len(userlogs) == 0 {
		outputChannel <- nil
		return
	}

	outputChannel <- userlogs
}

func (cfg Config) GetLocalInfoNames() []string {
	const printN string = "GetLocalInfoNames()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	rows, err := db.Query("select distinct drugName from " + cfg.UseSource)
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

func (cfg Config) GetLocalInfo(drug string, printit bool, prefix bool) []DrugInfo {
	var printN string
	if prefix == true {
		printN = "GetLocalInfo()"
	} else {
		printN = ""
	}

	drug = cfg.MatchAndReplace(drug, "substance")

	ret := checkIfExistsDB("drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		printName(printN, "No such drug in info database:", drug)
		return nil
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	rows, err := db.Query("select * from "+cfg.UseSource+" where drugName = ?", drug)
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
		location, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			printName(printN, err)
		}

		if printit {
			printName(printN, "Source:", cfg.UseSource)
			printName(printN, "Drug:", tempdrinfo.DrugName, ";", "Route:", tempdrinfo.DrugRoute)
			printName(printN, "---Dosages---")
			printNameF(printN, "Threshold: %g\n", tempdrinfo.Threshold)
			printName(printN, "Min\tMax\tRange")
			printNameF(printN, "%g\t%g\tLow\n", tempdrinfo.LowDoseMin, tempdrinfo.LowDoseMax)
			printNameF(printN, "%g\t%g\tMedium\n", tempdrinfo.MediumDoseMin, tempdrinfo.MediumDoseMax)
			printNameF(printN, "%g\t%g\tHigh\n", tempdrinfo.HighDoseMin, tempdrinfo.HighDoseMax)
			printName(printN, "Dose units:", tempdrinfo.DoseUnits)
			printName(printN, "---Times---")
			printName(printN, "Min\tMax\tPeriod\tUnits")
			printNameF(printN, "%g\t%g\tOnset\t%q\n",
				tempdrinfo.OnsetMin,
				tempdrinfo.OnsetMax,
				tempdrinfo.OnsetUnits)
			printNameF(printN, "%g\t%g\tComeup\t%q\n",
				tempdrinfo.ComeUpMin,
				tempdrinfo.ComeUpMax,
				tempdrinfo.ComeUpUnits)
			printNameF(printN, "%g\t%g\tPeak\t%q\n",
				tempdrinfo.PeakMin,
				tempdrinfo.PeakMax,
				tempdrinfo.PeakUnits)
			printNameF(printN, "%g\t%g\tOffset\t%q\n",
				tempdrinfo.OffsetMin,
				tempdrinfo.OffsetMax,
				tempdrinfo.OffsetUnits)
			printNameF(printN, "%g\t%g\tTotal\t%q\n",
				tempdrinfo.TotalDurMin,
				tempdrinfo.TotalDurMax,
				tempdrinfo.TotalDurUnits)
			printName(printN, "Time of fetch:", time.Unix(int64(tempdrinfo.TimeOfFetch), 0).In(location))
			printName(printN, "====================")
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

func (cfg Config) RemoveLogs(username string, amount int, reverse bool,
	remID int64, search string) bool {

	const printN string = "RemoveLogs()"

	stmtStr := "delete from " + loggingTableName + " where username = ?"
	if amount != 0 && remID == 0 || search != "none" {
		getAll := false
		if search != "none" {
			getAll = true
		}

		logsChannel := make(chan []UserLog)
		var gotLogs []UserLog
		go cfg.GetLogs(logsChannel, amount, 0, username, getAll, reverse, false, search, false)
		gotLogs = <-logsChannel

		if gotLogs == nil {
			printName(printN, "Couldn't get logs, because of an error, no logs will be removed.")
			return false
		}

		var gotTimeOfDose []int64
		var tempTimes int64

		for i := 0; i < len(gotLogs); i++ {
			tempTimes = gotLogs[i].StartTime
			gotTimeOfDose = append(gotTimeOfDose, tempTimes)
		}

		concatTimes := ""
		for i := 0; i < len(gotTimeOfDose); i++ {
			concatTimes = concatTimes + strconv.FormatInt(gotTimeOfDose[i], 10) + ","
		}
		concatTimes = strings.TrimSuffix(concatTimes, ",")

		stmtStr = "delete from " + loggingTableName + " where timeOfDoseStart in (" + concatTimes + ")"
	} else if remID != 0 && search == "none" {
		xtrs := [1]string{xtrastmt("username", "and")}
		ret := checkIfExistsDB("timeOfDoseStart", loggingTableName,
			cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
			xtrs[:], remID, username)
		if !ret {
			printName(printN, "Log with ID:", remID, "doesn't exists.")
			return false
		}

		stmtStr = "delete from " + loggingTableName + " where timeOfDoseStart = ? AND username = ?"
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	stmt, err := tx.Prepare(stmtStr)
	if err != nil {
		printName(printN, "tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	if remID != 0 {
		_, err = stmt.Exec(remID, username)
	} else if amount != 0 || search != "none" {
		_, err = stmt.Exec()
	} else {
		_, err = stmt.Exec(username)
	}
	if err != nil {
		printName(printN, "stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, "tx.Commit():", err)
		return false
	}

	printName(printN, "Data removed from info DB successfully.")

	return true
}

func (cfg Config) SetUserLogs(set string, id int64, username string, setValue string) bool {
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

	logsChannel := make(chan []UserLog)
	var gotLogs []UserLog

	if id == 0 {
		go cfg.GetLogs(logsChannel, 1, 0, username, false, true, false, "", false)
		gotLogs = <-logsChannel
		if gotLogs == nil {
			printName(printN, "Couldn't get last log to get the ID.")
			return false
		}

		id = gotLogs[0].StartTime
	} else {
		go cfg.GetLogs(logsChannel, 1, id, username, false, true, false, "", false)
		gotLogs = <-logsChannel
		if gotLogs == nil {
			printName(printN, "Couldn't get log with id:", id)
			return false
		}
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	stmtStr := fmt.Sprintf("update "+loggingTableName+" set %s = ? where timeOfDoseStart = ?",
		setName[set])

	tx, err := db.Begin()
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

	_, err = stmt.Exec(setValue, id)
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

func (cfg Config) InitUserSettings(username string) bool {
	const printN string = "InitUserSettings()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
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

	_, err = stmt.Exec(username, 0)
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

func (cfg Config) SetUserSettings(set string, username string, setValue string) bool {
	const printN string = "SetUserSettings()"

	ret := checkIfExistsDB("username", "userSettings",
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		nil, username)
	if ret == false {
		cfg.InitUserSettings(username)
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
		if _, err := strconv.ParseInt(setValue, 10, 64); err != nil {
			printName(printN, "Error when checking if integer:", setValue, ":", err)
			return false
		}

		if setValue == "0" || setValue == "none" {
			logsChannel := make(chan []UserLog)
			var gotLogs []UserLog
			go cfg.GetLogs(logsChannel, 1, 0, username, false, true, false, "none", false)
			gotLogs = <-logsChannel
			if gotLogs == nil {
				printName(printN, "No logs to remember.")
				return false
			}

			setValue = strconv.FormatInt(gotLogs[0].StartTime, 10)
		}
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	stmtStr := fmt.Sprintf("update userSettings set %s = ? where username = ?", set)

	tx, err := db.Begin()
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

func (cfg Config) GetUserSettings(set string, username string) string {
	const printN string = "GetUserSettings()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	fmtStmt := fmt.Sprintf("select %s from userSettings where username = ?", set)
	stmt, err := db.Prepare(fmtStmt)
	if err != nil {
		printName(printN, "SQL error in prepare:", err)
		return ""
	}
	defer stmt.Close()

	var got string
	err = stmt.QueryRow(username).Scan(&got)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		printName(printN, "Received weird error:", err)
		return ""
	}

	return got
}

func (cfg Config) RememberConfig(username string) *UserLog {
	const printN string = "RememberConfig()"

	got := cfg.GetUserSettings("useIDForRemember", username)
	if got == "" {
		printName(printN, "Couldn't get setting value: useIDForRemember")
		return nil
	}

	gotInt, err := strconv.ParseInt(got, 10, 64)
	if err != nil {
		printName(printN, "Couldn't convert:", got, "; to integer:", err)
		return nil
	}

	var logsChannel = make(chan []UserLog)
	var gotLogs []UserLog
	go cfg.GetLogs(logsChannel, 1, gotInt, username, false, false, false, "", false)
	gotLogs = <-logsChannel
	if gotLogs == nil {
		printName(printN, "No logs returned for:", gotInt)
		return nil
	}

	return &gotLogs[0]
}

func (cfg Config) ForgetConfig(username string) bool {
	const printN string = "ForgetConfig()"

	ret := cfg.SetUserSettings("useIDForRemember", username, ForgetInputConfigMagicNumber)
	if ret == false {
		printName(printN, "Couldn't set setting: useIDForRemember")
		return false
	}

	return true
}
