package drugdose

import (
	"context"
	"fmt"
	"strconv"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
)

// Constants used for matching settings
const settingTypeID string = "remember-id"
const rememberIDTableName string = "useIDForRemember"

func settingsTables(settingType string) (error, string) {
	const printN string = "settingsTables()"

	table := ""
	if settingType == settingTypeID {
		table = rememberIDTableName
	} else {
		return fmt.Errorf("%s%w: %s", sprintName(printN), NoNametypeError, settingType), ""
	}

	return nil, table
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
func (cfg Config) InitUserSettings(db *sql.DB, ctx context.Context, username string) error {
	const printN string = "InitUserSettings()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
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

func returnSetUserSetStmt(set string) string {
	return fmt.Sprintf("update userSettings set %s = ? where username = ?", set)
}

// SetUserSettings changes the user settings in the database.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
// (set to nil if function doesn't need to be concurrent)
//
// set - the name of the setting to change, available names are: remember-id
//
// username - the user the setting is changed for
//
// setValue - the value the setting is changed to
func (cfg Config) SetUserSettings(db *sql.DB, ctx context.Context,
	errChannel chan<- ErrorInfo, set string, username string, setValue string) ErrorInfo {

	const printN string = "SetUserSettings()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Username: username,
		Action:   ActionSetUserSettings,
	}

	ret := checkIfExistsDB(db, ctx,
		"username", "userSettings",
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		nil, username)
	if ret == false {
		err := cfg.InitUserSettings(db, ctx, username)
		if err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			if errChannel != nil {
				errChannel <- tempErrInfo
			}
			return tempErrInfo
		}
	}

	err, set := settingsTables(set)
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	stmtStr := returnSetUserSetStmt(set)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%s: %w", sprintName(printN), "db.BeginTx()", err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	stmt, err := tx.Prepare(stmtStr)
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Prepare(): ") {
		return tempErrInfo
	}
	defer stmt.Close()

	_, err = stmt.Exec(setValue, username)

	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "stmt.Exec():") {
		return tempErrInfo
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Commit(): ") {
		return tempErrInfo
	}

	printNameVerbose(cfg.VerbosePrinting, printN, set+": setting changed to:", setValue)

	if errChannel != nil {
		errChannel <- tempErrInfo
	}
	return tempErrInfo
}

// GetUserSettings return the value of a setting for a given user from the database.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// userSetErrChannel - a gorouting channel which is of type UserSettingError,
// holding an error variable and a string of the value for a given setting,
// make sure to always check the error first
// (set to nil if function doesn't need to be concurrent)
//
// set - the name of the setting to get the value of
//
// username - the user for which to get the setting
func (cfg Config) GetUserSettings(db *sql.DB, ctx context.Context,
	userSetErrChannel chan<- UserSettingError, set string, username string) UserSettingError {

	const printN string = "GetUserSettings()"

	tempUserSetErr := UserSettingError{
		UserSetting: "",
		Username:    username,
		Err:         nil,
	}

	fmtStmt := fmt.Sprintf("select %s from userSettings where username = ?", set)
	stmt, err := db.PrepareContext(ctx, fmtStmt)
	if err != nil {
		tempUserSetErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		tempUserSetErr.UserSetting = ""
		if userSetErrChannel != nil {
			userSetErrChannel <- tempUserSetErr
		}
		return tempUserSetErr
	}
	defer stmt.Close()

	var got string
	err = stmt.QueryRowContext(ctx, username).Scan(&got)
	if err != nil {
		tempUserSetErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		tempUserSetErr.UserSetting = ""
		if userSetErrChannel != nil {
			userSetErrChannel <- tempUserSetErr
		}
		return tempUserSetErr
	}

	tempUserSetErr.Err = nil
	tempUserSetErr.UserSetting = got

	if userSetErrChannel != nil {
		userSetErrChannel <- tempUserSetErr
	}
	return tempUserSetErr
}

// RememberDosing stores an ID of a log to reuse later via RecallDosing().
// This allows input of the dose only then drug, route, units will be reused
// according to the ID set to recall.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
// (set to nil if function doesn't need to be concurrent)
//
// username - the user to use for remembering a dosing
//
// forID - the ID to use for remembering a dosing
func (cfg Config) RememberDosing(db *sql.DB, ctx context.Context,
	errChannel chan<- ErrorInfo, username string, forID int64) ErrorInfo {
	const printN string = "RememberDosing()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Username: username,
		Action:   ActionRememberDosing,
	}

	forIDStr := strconv.FormatInt(forID, 10)
	if forIDStr == "0" {
		gotLogs := cfg.GetLogs(db, ctx, nil, 1, 0, username, true, "none", "")
		if gotLogs.Err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), gotLogs.Err)
			if errChannel != nil {
				errChannel <- tempErrInfo
			}
			return tempErrInfo
		}
		forIDStr = strconv.FormatInt(gotLogs.UserLogs[0].StartTime, 10)
	}

	gotErrInfo := cfg.SetUserSettings(db, ctx, nil, settingTypeID, username, forIDStr)
	if gotErrInfo.Err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), gotErrInfo.Err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	if errChannel != nil {
		errChannel <- tempErrInfo
	}
	return tempErrInfo
}

// RecallDosing gives the data for the last configured dosing using the ID.
// Checkout RememberDosing() for more info!
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// userLogsErrorChannel - the goroutine channel used to return the logs and
// the error
// (set to nil if function doesn't need to be concurrent)
//
// username - the user to recall the logs for
func (cfg Config) RecallDosing(db *sql.DB, ctx context.Context,
	userLogsErrorChannel chan<- UserLogsError, username string) UserLogsError {
	const printN string = "RememberConfig()"

	tempUserLogsError := UserLogsError{
		UserLogs: nil,
		Username: username,
		Err: nil,
	}
	gotUserSetErr := cfg.GetUserSettings(db, ctx, nil, "useIDForRemember", username)
	err := gotUserSetErr.Err
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
		tempUserLogsError.Err = err
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}
	gotID := gotUserSetErr.UserSetting
	if gotID == ForgetInputConfigMagicNumber {
		tempUserLogsError.Err = nil
		tempUserLogsError.UserLogs = nil
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}
	gotInt, err := strconv.ParseInt(gotID, 10, 64)
	if err != nil {
		err := fmt.Errorf("%s%s: %w", sprintName(printN), "strconv.ParseInt()", err)
		tempUserLogsError.Err = err
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}

	tempUserLogsError = cfg.GetLogs(db, ctx, nil, 1, gotInt, username, false, "", "")
	err = tempUserLogsError.Err
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
	}
	tempUserLogsError.Err = err
	if userLogsErrorChannel != nil {
		userLogsErrorChannel <- tempUserLogsError
	}
	return tempUserLogsError
}

// ForgetDosing is meant to reset the setting for remembering dosings.
// ForgetInputConfigMagicNumber is a constant containing the value meant to
// be ignored. Checkout RememberDosing()!
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
// (set to nil if function doesn't need to be concurrent)
//
// username - the user for which to forget the dosing
func (cfg Config) ForgetDosing(db *sql.DB, ctx context.Context,
	errChannel chan<- ErrorInfo, username string) ErrorInfo {
	const printN string = "ForgetConfig()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Username: username,
		Action:   ActionForgetDosing,
	}

	gotErrInfo := cfg.SetUserSettings(db, ctx, nil, settingTypeID, username, ForgetInputConfigMagicNumber)
	if gotErrInfo.Err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), gotErrInfo.Err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	if errChannel != nil {
		errChannel <- tempErrInfo
	}
	return tempErrInfo
}
