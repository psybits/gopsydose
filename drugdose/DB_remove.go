package drugdose

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"
)

// CleanDB deletes all tables in the database.
// Make sure you don't have any other tables related to other projects in
// the database! It's a good idea to create different databases for
// every project.
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
		err = handleErrRollbackSeq(err, tx, printN, "tx.Exec(): ")
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	err = handleErrRollbackSeq(err, tx, printN, "tx.Commit(): ")
	if err != nil {
		return err
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
	err = handleErrRollbackSeq(err, tx, printN, "tx.Exec(): ")
	if err != nil {
		return err
	}

	err = tx.Commit()
	err = handleErrRollbackSeq(err, tx, printN, "tx.Commit(): ")
	if err != nil {
		return err
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
// replaceOnly - if true, remove only replace tables (source specific),
// keep the global ones intact
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
		err = handleErrRollbackSeq(err, tx, printN, "tx.Exec(): ")
		if err != nil {
			return err
		}
	}
	fmt.Println()

	err = tx.Commit()
	err = handleErrRollbackSeq(err, tx, printN, "tx.Commit(): ")
	if err != nil {
		return err
	}

	printName(printN, "All tables removed from DB.")

	return nil
}

// RemoveLogs removes logs from the dose log table.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// username - the user's logs that will be removed, no other user's logs will
// be touched
//
// amount - how many logs to remove, if 0 it removes all
//
// reverse - from which direction to start removing logs, if true go from high
// values to low values, this should remove the newest logs first,
// false is the opposite direction
//
// remID - if not 0, remove a specific log using it's start timestamp (ID)
//
// search - remove logs only matching this string
func (cfg Config) RemoveLogs(db *sql.DB, ctx context.Context,
	errChannel chan error, username string, amount int, reverse bool,
	remID int64, search string) {

	const printN string = "RemoveLogs()"

	stmtStr := "delete from " + loggingTableName + " where username = ?"
	if (amount != 0 && remID == 0) || (search != "none" && search != "") {
		if search != "none" && search != "" {
			amount = 0
		}

		userLogsErrChan := make(chan UserLogsError)
		go cfg.GetLogs(db, ctx, userLogsErrChan, amount, 0, username, reverse, search)
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
	} else if remID != 0 && (search == "none" || search == "") {
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
	if handleErrRollback(err, tx, errChannel, printN, "tx.Prepare(): ") {
		return
	}
	defer stmt.Close()
	if remID != 0 {
		_, err = stmt.Exec(remID, username)
	} else {
		_, err = stmt.Exec(username)
	}
	if handleErrRollback(err, tx, errChannel, printN, "stmt.Exec(): ") {
		return
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, printN, "tx.Commit(): ") {
		return
	}

	printNameVerbose(cfg.VerbosePrinting, printN, "Data removed from log table in DB successfully: user:",
		username, "; amount:", amount, "; reverse:", reverse, "; remID:", remID,
		"; search:", search)

	errChannel <- nil
}

// RemoveSingleDrugInfoDB removes all entries of a single drug from the local
// info DB, instead of deleting the whole DB/table. For example if there's a need to
// clear all information about dosage and timing for a specific drug if it's
// old or incorrect.
//
// This function is meant to be run concurrently.
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
	if handleErrRollback(err, tx, errChannel, printN, "tx.Prepare(): ") {
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(drug)
	if handleErrRollback(err, tx, errChannel, printN, "stmt.Exec(): ") {
		return
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, printN, "tx.Commit(): ") {
		return
	}

	printName(printN, "Data removed from info DB successfully:", drug, "; source: ", cfg.UseSource)

	errChannel <- nil
}
