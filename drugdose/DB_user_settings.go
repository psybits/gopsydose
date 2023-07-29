package drugdose

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"
)

// InitUserSettings creates the row with default settings for a user.
// These settings are kept in the database and are not global like the config
// files. All users have their own settings.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// username - the user to create default settings for
func (cfg Config) InitUserSettings(db *sql.DB, ctx context.Context, username string) error {
	const printN string = "InitUserSettings()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		err = errors.New(sprintName(printN, err))
		return err
	}

	stmt, err := tx.Prepare("insert into userSettings" +
		" (username, useIDForRemember) " +
		"values(?, ?)")
	err = handleErrRollbackSeq(err, tx, printN, "tx.Prepare(): ")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(username, ForgetInputConfigMagicNumber)
	err = handleErrRollbackSeq(err, tx, printN, "tx.Exec(): ")
	if err != nil {
		return err
	}
	err = tx.Commit()
	err = handleErrRollbackSeq(err, tx, printN, "tx.Commit(): ")
	if err != nil {
		return err
	}

	printName(printN, "User settings initialized successfully!")

	return nil
}

// SetUserSettings changes the user settings in the database.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// set - the name of the setting to change
//
// username - the user the setting is changed for
//
// setValue - the value the setting is changed to
func (cfg Config) SetUserSettings(db *sql.DB, ctx context.Context,
	errChannel chan<- error, set string, username string, setValue string) {

	const printN string = "SetUserSettings()"

	ret := checkIfExistsDB(db, ctx,
		"username", "userSettings",
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		nil, username)
	if ret == false {
		err := cfg.InitUserSettings(db, ctx, username)
		if err != nil {
			errChannel <- errors.New(sprintName(printN, err))
			return
		}
	}

	if username == "none" {
		errChannel <- errors.New(sprintName(printN, "Please specify an username!"))
		return
	}

	if set == "none" {
		errChannel <- errors.New(sprintName(printN, "Please specify a set type!"))
		return
	}

	if setValue == "none" {
		errChannel <- errors.New(sprintName(printN, "Please specify a value to set!"))
		return
	}

	stmtStr := fmt.Sprintf("update userSettings set %s = ? where username = ?", set)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, "db.Begin():", err))
		return
	}

	stmt, err := tx.Prepare(stmtStr)
	if handleErrRollback(err, tx, errChannel, printN, "tx.Prepare(): ") {
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(setValue, username)

	if handleErrRollback(err, tx, errChannel, printN, "stmt.Exec():") {
		return
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, printN, "tx.Commit(): ") {
		return
	}

	printNameVerbose(cfg.VerbosePrinting, printN, set+": setting changed to:", setValue)

	errChannel <- nil
}

// GetUserSettings return the value of a setting for a given user from the database.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// userSetErrChannel - a gorouting channel which is of type UserSettingError,
// holding an error variable and a string of the value for a given setting,
// make sure to always check the error first
//
// set - the name of the setting to get the value of
//
// username - the user for which to get the setting
func (cfg Config) GetUserSettings(db *sql.DB, ctx context.Context,
	userSetErrChannel chan<- UserSettingError, set string, username string) {

	const printN string = "GetUserSettings()"

	tempUserSetErr := UserSettingError{}

	fmtStmt := fmt.Sprintf("select %s from userSettings where username = ?", set)
	stmt, err := db.PrepareContext(ctx, fmtStmt)
	if err != nil {
		tempUserSetErr.Err = errors.New(sprintName(printN, "SQL error in prepare:", err))
		tempUserSetErr.UserSetting = ""
		userSetErrChannel <- tempUserSetErr
		return
	}
	defer stmt.Close()

	var got string
	err = stmt.QueryRowContext(ctx, username).Scan(&got)
	if err != nil {
		tempUserSetErr.Err = errors.New(sprintName(printN, err))
		tempUserSetErr.UserSetting = ""
		userSetErrChannel <- tempUserSetErr
		return
	}

	tempUserSetErr.Err = nil
	tempUserSetErr.UserSetting = got

	userSetErrChannel <- tempUserSetErr
}

// RememberDosing stores an ID of a log to reuse later via RecallDosing().
// This allows input of the dose only then drug, route, units will be reused
// according to the ID set to recall.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// username - the user to use for remembering a dosing
//
// forID - the ID to use for remembering a dosing
func (cfg Config) RememberDosing(db *sql.DB, ctx context.Context,
	errChannel chan<- error, username string, forID int64) {
	const printN string = "RememberDosing()"

	forIDStr := strconv.FormatInt(forID, 10)
	if forIDStr == "0" {
		userLogsErrChan := make(chan UserLogsError)
		go cfg.GetLogs(db, ctx, userLogsErrChan, 1, 0, username, true, "none")
		gotLogs := <-userLogsErrChan
		if gotLogs.Err != nil {
			errChannel <- errors.New(sprintName(printN, gotLogs.Err))
			return
		}
		forIDStr = strconv.FormatInt(gotLogs.UserLogs[0].StartTime, 10)
	}

	errChannel2 := make(chan error)
	go cfg.SetUserSettings(db, ctx, errChannel2, "useIDForRemember", username, forIDStr)
	err := <-errChannel2
	if err != nil {
		err := errors.New(sprintName(printN, err))
		errChannel <- err
		return
	}

	errChannel <- nil
}

// RecallDosing gives the data for the last configured dosing using the ID.
// Checkout RememberDosing() for more info!
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// userLogsErrorChannel - the goroutine channel used to return the logs and
// the error
//
// username - the user to recall the logs for
func (cfg Config) RecallDosing(db *sql.DB, ctx context.Context,
	userLogsErrorChannel chan<- UserLogsError, username string) {
	const printN string = "RememberConfig()"

	tempUserLogsError := UserLogsError{}
	userSetErr := make(chan UserSettingError)
	go cfg.GetUserSettings(db, ctx, userSetErr, "useIDForRemember", username)
	gotUserSetErr := <-userSetErr
	err := gotUserSetErr.Err
	if err != nil {
		err = errors.New(sprintName(printN, "Couldn't get setting value: useIDForRemember: ", err))
		tempUserLogsError.Err = err
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	gotID := gotUserSetErr.UserSetting
	if gotID == ForgetInputConfigMagicNumber {
		tempUserLogsError.Err = nil
		tempUserLogsError.UserLogs = nil
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	gotInt, err := strconv.ParseInt(gotID, 10, 64)
	if err != nil {
		err := errors.New(sprintName(printN, "Couldn't convert:", gotID, "; to integer: ", err))
		tempUserLogsError.Err = err
		userLogsErrorChannel <- tempUserLogsError
		return
	}

	userLogsErrChan2 := make(chan UserLogsError)
	go cfg.GetLogs(db, ctx, userLogsErrChan2, 1, gotInt, username, false, "")
	tempUserLogsError = <-userLogsErrChan2
	err = tempUserLogsError.Err
	if err != nil {
		err = errors.New(sprintName(printN, err))
	}
	tempUserLogsError.Err = err
	userLogsErrorChannel <- tempUserLogsError
}

// ForgetDosing is meant to reset the setting for remembering dosings.
// ForgetInputConfigMagicNumber is a constant containing the value meant to
// be ignored. Checkout RememberDosing()!
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// username - the user for which to forget the dosing
func (cfg Config) ForgetDosing(db *sql.DB, ctx context.Context,
	errChannel chan<- error, username string) {
	const printN string = "ForgetConfig()"

	errChannel2 := make(chan error)
	go cfg.SetUserSettings(db, ctx, errChannel2, "useIDForRemember", username, ForgetInputConfigMagicNumber)
	err := <-errChannel2
	if err != nil {
		err = errors.New(sprintName(printN, "Couldn't reset setting: useIDForRemember: ", err))
		errChannel <- err
		return
	}

	errChannel <- nil
}
