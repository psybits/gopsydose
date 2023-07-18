package drugdose

import (
	"context"
	"errors"
	"os"
	"path"

	"database/sql"	
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

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
