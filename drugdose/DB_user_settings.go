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
	_ "github.com/mattn/go-sqlite3"
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
