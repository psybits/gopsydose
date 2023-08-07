package drugdose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hasura/go-graphql-client"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"
)

// TODO: Encryption should be done by default unless specified not to by the user from the settings
// But first the official implementation for encryption has to be done in the sqlite module

const SqliteDriver string = "sqlite"
const MysqlDriver string = "mysql"

const loggingTableName string = "userLogs"
const userSetTableName string = "userSettings"

// When this number is set as the reference ID for remembering
// a particular input, it means that it's now "forgotten"
// and there should be no attempts to "remember" any inputs.
// This is related to the RememberConfig() and ForgetConfig() functions.
const ForgetInputConfigMagicNumber string = "0"

const ActionFetchFromSource string = "fetching from source completed"
const ActionChangeUserLog string = "changing user log completed"
const ActionAddToInfoTable string = "adding to info table completed"
const ActionFetchFromPsychonautWiki string = "fetching from psychonautwiki completed"
const ActionAddToDoseTable string = "adding to dose table completed"
const ActionRemoveLogs string = "removing logs from dose table completed"
const ActionRemoveSingleDrugInfo string = "removing single drug info completed"
const ActionSetUserSettings string = "user settings change completed"
const ActionRememberDosing string = "dosing remember completed"
const ActionForgetDosing string = "dosing forgetting completed"

var NoValidSourceSel error = errors.New("no valid source selected")
var TimeoutValueEmptyError error = errors.New("timeout value is empty")

func exitProgram(printN string) {
	printName(printN, "exitProgram(): Exiting")
	os.Exit(1)
}

func errorCantOpenDB(path string, err error, printN string) {
	printName(printN, "errorCantOpenDB(): Error opening DB:", path, ":", err)
	exitProgram(printN)
}

// If err is not nil, starts a transaction rollback and returns the error
// through errChannel.
//
// This function is meant to be used within concurrently ran functions.
//
// Returns true if there's an error, false otherwise.
func handleErrRollback(err error, tx *sql.Tx, errChannel chan<- ErrorInfo,
	errInfo ErrorInfo, printN string, xtraMsg string) bool {
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			errInfo.Err = fmt.Errorf("error when attempting to roll back: %s%w",
				sprintName(printN, xtraMsg), err2)
			errChannel <- errInfo
			return true
		}
		errInfo.Err = fmt.Errorf("rolling back: %s%w", sprintName(printN, xtraMsg), err)
		errChannel <- errInfo
		return true
	}
	return false
}

// If err is not nil, starts a transaction rollback and returns the error.
//
// This function is meant to be used within sequentially ran functions.
//
// Returns the error if there's one, nil otherwise.
func handleErrRollbackSeq(err error, tx *sql.Tx, printN string, xtraMsg string) error {
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return fmt.Errorf("error when attempting to roll back: %s%w",
				sprintName(printN, xtraMsg), err2)
		}
		return fmt.Errorf("rolling back: %s%w", sprintName(printN, xtraMsg), err)
	}
	return nil
}

// Make sure the input column name matches exactly with the proper names.
func checkColIsInvalid(validCols []string, gotCol string, printN string) error {
	validCol := false
	if gotCol != "" && gotCol != "none" && len(validCols) != 0 {
		validCols := validLogCols()
		for i := 0; i < len(validCols); i++ {
			if gotCol == validCols[i] {
				validCol = true
				break
			}
		}
		if validCol == false {
			return fmt.Errorf("%s%w", sprintName(printN), InvalidColInput)
		}
	} else if gotCol == "" || gotCol == "none" {
		return errors.New(sprintName(printN, "Empty column given."))
	} else if len(validCols) == 0 {
		return errors.New(sprintName(printN, "Invalid parameters when checking if column is invalid."))
	}

	return nil
}

type UserLog struct {
	StartTime    int64
	Username     string
	EndTime      int64
	DrugName     string
	Dose         float32
	DoseUnits    string
	DrugRoute    string
	Cost         float32
	CostCurrency string
}

type UserLogsError struct {
	UserLogs []UserLog
	Username string
	Err      error
}

type DrugNamesError struct {
	DrugNames []string
	Username  string
	Err       error
}

type DrugInfoError struct {
	DrugI    []DrugInfo
	Username string
	Err      error
}

type UserSettingError struct {
	UserSetting string
	Username    string
	Err         error
}

type LogCountError struct {
	LogCount uint32
	Username string
	Err      error
}

type AllUsersError struct {
	AllUsers []string
	Username string
	Err      error
}

type ErrorInfo struct {
	Err      error
	Action   string
	Username string
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

// Uses the value of Timeout from the settings file to create a WithTimeout
// context. If no errors are found, it then returns the context to be used
// where it's needed.
func (cfg Config) UseConfigTimeout() (context.Context, context.CancelFunc, error) {
	const printN string = "UseConfigTimeout()"

	if cfg.Timeout == "" || cfg.Timeout == "none" {
		return nil, nil, fmt.Errorf("%s%w", sprintName(printN), TimeoutValueEmptyError)
	}

	gotDuration, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("%s%w", sprintName(printN, "time.ParseDuration(): "), err)
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
	const printN string = "OpenDBConnection()"

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err, printN)
	}

	cfg.PingDB(db, ctx)
	return db
}

// Ping verifies a connection to the database is still alive,
// establishing a connection if necessary.
//
// db - open database connection
//
// ctx - context to be passed to PingContext()
func (cfg Config) PingDB(db *sql.DB, ctx context.Context) {
	const printN string = "PingDB()"

	err := db.PingContext(ctx)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err, printN)
	}
}

// Function which generates and returns a query for looking up table
// names in the database.
// If tableName is empty, query returns all tables in the database.
// If tableName is not empty, query returns a specific table if it exists.
func (cfg Config) getTableNamesQuery(tableName string) string {
	var queryStr string
	andTable := ""
	if cfg.DBDriver == SqliteDriver {
		if tableName != "" {
			andTable = " AND name = '" + tableName + "'"
		}
		queryStr = "SELECT name FROM sqlite_schema WHERE type='table'" + andTable
	} else if cfg.DBDriver == MysqlDriver {
		if tableName != "" {
			andTable = " AND table_name = '" + tableName + "'"
		}
		dbName := strings.Split(cfg.DBSettings[cfg.DBDriver].Path, "/")
		queryStr = "SELECT table_name FROM information_schema.tables WHERE table_schema = '" +
			dbName[1] + "'" + andTable
	}
	return queryStr
}

// CheckTables returns true if a table exists in the database with the name
// tableName. It returns false in case of error or if the table isn't found.
// If tableName is empty it will search for all tables in the database.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// tableName - name of table to check if it exists
func (cfg Config) CheckTables(db *sql.DB, ctx context.Context, tableName string) bool {
	const printN string = "CheckTables()"

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

// FetchFromSource goes through all source names and picks the proper
// function for fetching drug information. The information is automatically
// added to the proper info table depending on the Config struct.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// drugname - the name of the substance to fetch information for
//
// client - the initialised structure for the graphql client,
// best done using InitGraphqlClient(), but can be done manually if needed
//
// username - the user requesting the fetch
func (cfg Config) FetchFromSource(db *sql.DB, ctx context.Context,
	errChannel chan<- ErrorInfo, drugname string, client graphql.Client,
	username string) {

	const printN string = "FetchFromSource()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Action:   ActionFetchFromSource,
		Username: username,
	}

	gotsrcData := GetSourceData()
	printNameVerbose(cfg.VerbosePrinting,
		printN, "Using API from settings.toml:", cfg.UseSource)
	printNameVerbose(cfg.VerbosePrinting,
		printN, "Got API URL from sources.toml:", gotsrcData[cfg.UseSource].API_ADDRESS)

	if cfg.UseSource == "psychonautwiki" {
		errChannel2 := make(chan ErrorInfo)
		go cfg.FetchPsyWiki(db, ctx, errChannel2, drugname, client, username)
		gotErrInfo := <-errChannel2
		if gotErrInfo.Err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w",
				sprintName(printN, "While fetching from: ", cfg.UseSource, " ; error: "),
				gotErrInfo.Err)
			errChannel <- tempErrInfo
			return
		}
	} else {
		tempErrInfo.Err = fmt.Errorf("%s%w: %s", sprintName(printN), NoValidSourceSel, cfg.UseSource)
		errChannel <- tempErrInfo
		return
	}

	errChannel <- tempErrInfo
}

// ChangeUserLog can be used to modify log data of a single log.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// set - what log data to change, if name is invalid, InvalidColInput
// error will be send through userLogsErrorChannel
//
// id - if 0 will change the newest log, else it will change the log with
// the given id
//
// username - the user who's log we're changing
//
// setValue - the new value to set
func (cfg Config) ChangeUserLog(db *sql.DB, ctx context.Context, errChannel chan<- ErrorInfo,
	set string, id int64, username string, setValue string) {
	const printN string = "ChangeUserLog()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Action:   ActionChangeUserLog,
		Username: username,
	}

	err := checkColIsInvalid(validLogCols(), set, printN)
	if err != nil {
		tempErrInfo.Err = err
		errChannel <- tempErrInfo
		return
	}

	if setValue == "now" && set == LogStartTimeCol || setValue == "now" && set == LogEndTimeCol {
		setValue = strconv.FormatInt(time.Now().Unix(), 10)
	}

	if set == LogStartTimeCol || set == LogEndTimeCol {
		if _, err := strconv.ParseInt(setValue, 10, 64); err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN, "strconv.ParseInt(): "), err)
			errChannel <- tempErrInfo
			return
		}
	}

	if set == "dose" {
		if _, err := strconv.ParseFloat(setValue, 64); err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN, "strconv.ParseFloat(): "), err)
			errChannel <- tempErrInfo
			return
		}
	}

	userLogsErrChan := make(chan UserLogsError)
	var gotLogs []UserLog
	var gotErr error

	go cfg.GetLogs(db, ctx, userLogsErrChan, 1, id, username, true, "", "")
	gotUserLogsErr := <-userLogsErrChan
	gotErr = gotUserLogsErr.Err
	if gotErr != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), gotErr)
		errChannel <- tempErrInfo
		return
	}

	gotLogs = gotUserLogsErr.UserLogs
	id = gotLogs[0].StartTime

	stmtStr := fmt.Sprintf("update "+loggingTableName+" set %s = ? where timeOfDoseStart = ?", set)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN, "db.Begin(): "), err)
		errChannel <- tempErrInfo
		return
	}

	stmt, err := tx.Prepare(stmtStr)
	if handleErrRollback(err, tx, errChannel, tempErrInfo, printN, "tx.Prepare(): ") {
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(setValue, id)
	if handleErrRollback(err, tx, errChannel, tempErrInfo, printN, "stmt.Exec(): ") {
		return
	}

	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, tempErrInfo, printN, "tx.Commit(): ") {
		return
	}

	printName(printN, "entry:", id, "; changed:", set, "; to value:", setValue, "; for user:", username)

	errChannel <- tempErrInfo
}
