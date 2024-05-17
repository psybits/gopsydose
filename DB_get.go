package drugdose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
)

var NoLogsError error = errors.New("no logs returned for user")

// NoUsersReturned is the error returned when no unique usernames from the log
// table have been retrieved.
var NoUsersReturned error = errors.New("no usernames have been returned")

// EmptyListDrugNamesError is the error returned when no unique drug names could
// be retrieved from the database.
var EmptyListDrugNamesError error = errors.New("empty list of drug names from table")

var NoDrugInfoTable error = errors.New("no such drug in info table")

var InvalidColInput error = errors.New("an invalid column name has been given")

// GetDBSize returns the total size of the database in bytes (int64).
func (cfg Config) GetDBSize() int64 {
	const printN string = "GetDBSize()"

	if cfg.DBDriver == SqliteDriver {
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
	} else if cfg.DBDriver == MysqlDriver {
		db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
		if err != nil {
			errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err, printN)
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
// currently present in the dose log table.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// allUsersErrChan - the goroutine channel used to return all unique usernames
// and the error
//
// username - user to get unique usernames for
func (cfg Config) GetUsers(db *sql.DB, ctx context.Context,
	allUsersErrChan chan<- AllUsersError, username string) AllUsersError {
	const printN string = "GetUsers()"

	var allUsers []string
	var tempUser string

	tempAllUsersErr := AllUsersError{
		AllUsers: nil,
		Username: username,
		Err:      nil,
	}

	rows, err := db.QueryContext(ctx, "select distinct username from "+loggingTableName)
	if err != nil {
		tempAllUsersErr.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.QueryContext()"), err)
		if allUsersErrChan != nil {
			allUsersErrChan <- tempAllUsersErr
		}
		return tempAllUsersErr
	}

	for rows.Next() {
		err = rows.Scan(&tempUser)
		if err != nil {
			tempAllUsersErr.Err = fmt.Errorf("%s: %w", sprintName(printN, "rows.Scan()"), err)
			if allUsersErrChan != nil {
				allUsersErrChan <- tempAllUsersErr
			}
			return tempAllUsersErr
		}
		allUsers = append(allUsers, tempUser)
	}

	if len(allUsers) == 0 {
		tempAllUsersErr.Err = fmt.Errorf("%s%w", sprintName(printN), NoUsersReturned)
		if allUsersErrChan != nil {
			allUsersErrChan <- tempAllUsersErr
		}
		return tempAllUsersErr
	}

	tempAllUsersErr.AllUsers = allUsers
	if allUsersErrChan != nil {
		allUsersErrChan <- tempAllUsersErr
	}
	return tempAllUsersErr
}

// GetLogsCount returns total amount of logs in
// the dose log table for username set in user parameter.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// user - user to get log count for
//
// logCountErrChan - the goroutine channel used to return the log count and
// the error
func (cfg Config) GetLogsCount(db *sql.DB, ctx context.Context, user string,
	logCountErrChan chan<- LogCountError) LogCountError {
	const printN string = "GetLogsCount()"

	tempLogCountErr := LogCountError{
		LogCount: 0,
		Username: user,
		Err:      nil,
	}

	var count uint32

	stmt, err := db.PrepareContext(ctx,
		"select count(*) from "+loggingTableName+" where username = ?")
	if err != nil {
		tempLogCountErr.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.PrepareContext()"), err)
		if logCountErrChan != nil {
			logCountErrChan <- tempLogCountErr
		}
		return tempLogCountErr
	}

	row := stmt.QueryRowContext(ctx, user)
	err = row.Scan(&count)
	if err != nil {
		tempLogCountErr.Err = fmt.Errorf("%s: %w", sprintName(printN, "row.Scan()"), err)
		if logCountErrChan != nil {
			logCountErrChan <- tempLogCountErr
		}
		return tempLogCountErr
	}

	tempLogCountErr.LogCount = count
	if logCountErrChan != nil {
		logCountErrChan <- tempLogCountErr
	}
	return tempLogCountErr
}

// GetLogs returns all logs for a given username in the drug log table.
// It uses a single channel with the type UserLogsError, containing a slice of
// UserLogs structs and a variable with an error type. When using this function,
// the error must be checked before reading the logs. Every log is a separate
// element of the UserLogs slice.
//
// db - an open database connection
//
// ctx - context that will be passed to the sql query function
//
// userLogsErrorChannel - the goroutine channel used to return the logs and
// the error
// (set to nil if function doesn't need to be concurrent)
//
// num - amount of logs to return (limit), if 0 returns all logs (without limit)
//
// id - if not 0, will return the exact log matching that id for the given user
//
// user - the user which created the logs, will returns only the logs for that
// username
//
// desc - if true (descending) go from high values to low values,
// this should return the newest logs first, false (ascending) is
// the opposite direction
//
// search - return logs which contain this string
//
// getExact - if not empty, choose which column to search for and changes
// the search behavior to exact matching, if name is invalid, InvalidColInput
// error will be send through userLogsErrorChannel or returned
func (cfg Config) GetLogs(db *sql.DB, ctx context.Context,
	userLogsErrorChannel chan<- UserLogsError, num int, id int64,
	user string, desc bool, search string, getExact string) UserLogsError {

	printN := "GetLogs()"

	numstr := strconv.Itoa(num)

	userlogs := []UserLog{}
	tempUserLogsError := UserLogsError{
		UserLogs: userlogs,
		Username: user,
		Err:      nil,
	}

	var endstmt string
	if num == 0 {
		endstmt = ""
	} else {
		endstmt = " limit " + numstr
	}

	orientation := "asc"
	if desc {
		orientation = "desc"
	}

	if getExact != "" && getExact != "none" {
		err := checkColIsInvalid(validLogCols(), getExact, printN)
		if err != nil {
			tempUserLogsError.Err = err
			if userLogsErrorChannel != nil {
				userLogsErrorChannel <- tempUserLogsError
			}
			return tempUserLogsError
		}
	}

	searchStmt := ""
	var searchArr []any
	if search != "none" && search != "" {
		search = cfg.MatchAndReplaceAll(db, ctx, search)
		if getExact == "none" || getExact == "" {
			searchColumns := []string{"drugName",
				LogDoseCol,
				LogDoseUnitsCol,
				LogDrugRouteCol,
				LogCostCol,
				LogCostCurrencyCol}
			searchArr = append(searchArr, user)
			searchStmt += "and (" + searchColumns[0] + " like ? "
			searchArr = append(searchArr, "%"+search+"%")
			for i := 1; i < len(searchColumns); i++ {
				searchStmt += "or " + searchColumns[i] + " like ? "
				searchArr = append(searchArr, "%"+search+"%")
			}
			searchStmt += ") "
		} else {
			searchArr = append(searchArr, user)
			searchArr = append(searchArr, search)
			searchStmt = "and " + getExact + " = ? "
		}
	}

	mainQuery := "select * from " + loggingTableName + " where username = ? " + searchStmt +
		"order by timeOfDoseStart " + orientation + endstmt
	stmt, err := db.PrepareContext(ctx, mainQuery)
	if err != nil {
		tempUserLogsError.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.PrepareContext()"), err)
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}
	var rows *sql.Rows
	if id == 0 {
		if search == "none" || search == "" {
			rows, err = stmt.QueryContext(ctx, user)
		} else {
			rows, err = stmt.QueryContext(ctx, searchArr...)
		}
	} else {
		stmt, err = db.PrepareContext(ctx,
			"select * from "+loggingTableName+" where username = ? and timeOfDoseStart = ?")
		if err != nil {
			tempUserLogsError.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.PrepareContext()"), err)
			if userLogsErrorChannel != nil {
				userLogsErrorChannel <- tempUserLogsError
			}
			return tempUserLogsError
		}
		rows, err = stmt.QueryContext(ctx, user, id)
		if err != nil {
			tempUserLogsError.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.QueryContext()"), err)
			if userLogsErrorChannel != nil {
				userLogsErrorChannel <- tempUserLogsError
			}
			return tempUserLogsError
		}
	}
	defer rows.Close()

	for rows.Next() {
		tempul := UserLog{}
		err = rows.Scan(&tempul.StartTime, &tempul.Username, &tempul.EndTime, &tempul.DrugName,
			&tempul.Dose, &tempul.DoseUnits, &tempul.DrugRoute, &tempul.Cost, &tempul.CostCurrency)
		if err != nil {
			tempUserLogsError.Err = fmt.Errorf("%s: %w", sprintName(printN, "rows.Scan()"), err)
			tempUserLogsError.UserLogs = userlogs
			if userLogsErrorChannel != nil {
				userLogsErrorChannel <- tempUserLogsError
			}
			return tempUserLogsError
		}

		userlogs = append(userlogs, tempul)
	}
	err = rows.Err()
	if err != nil {
		tempUserLogsError.Err = fmt.Errorf("%s: %w", sprintName(printN, "rows.Err()"), err)
		tempUserLogsError.UserLogs = userlogs
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}
	if len(userlogs) == 0 {
		tempUserLogsError.Err = fmt.Errorf("%s%w: %s", sprintName(printN), NoLogsError, user)
		tempUserLogsError.UserLogs = userlogs
		if userLogsErrorChannel != nil {
			userLogsErrorChannel <- tempUserLogsError
		}
		return tempUserLogsError
	}

	tempUserLogsError.Err = nil
	tempUserLogsError.UserLogs = userlogs
	if userLogsErrorChannel != nil {
		userLogsErrorChannel <- tempUserLogsError
	}
	return tempUserLogsError
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
		if elem.Cost != 0 {
			printNameF(printN, "Cost:\t%g\n", elem.Cost)
			printNameF(printN, "Curr:\t%q\n", elem.CostCurrency)
		}
		printName(printN, "=========================")
	}
}

// GetLocalInfo returns a slice containing all information about a drug.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// drugInfoErrChan - the goroutine channel used to return the slice containing
// information about all routes for a drug and the error
//
// drug - drug to get information about
//
// username - the user requesting the local info
func (cfg Config) GetLocalInfo(db *sql.DB, ctx context.Context,
	drugInfoErrChan chan<- DrugInfoError, drug string, username string) {
	printN := "GerLocalInfo()"

	drug = cfg.MatchAndReplace(db, ctx, drug, NameTypeSubstance)

	tempDrugInfoErr := DrugInfoError{
		DrugI:    nil,
		Username: username,
		Err:      nil,
	}

	ret := checkIfExistsDB(db, ctx,
		"drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		tempDrugInfoErr.Err = fmt.Errorf("%s%w: %s", sprintName(printN), NoDrugInfoTable, drug)
		drugInfoErrChan <- tempDrugInfoErr
		return
	}

	rows, err := db.QueryContext(ctx, "select * from "+cfg.UseSource+" where drugName = ?", drug)
	if err != nil {
		tempDrugInfoErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		drugInfoErrChan <- tempDrugInfoErr
		return
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
			tempDrugInfoErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			drugInfoErrChan <- tempDrugInfoErr
			return
		}

		infoDrug = append(infoDrug, tempdrinfo)
	}
	err = rows.Err()
	if err != nil {
		tempDrugInfoErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		drugInfoErrChan <- tempDrugInfoErr
		return
	}

	tempDrugInfoErr.DrugI = infoDrug
	drugInfoErrChan <- tempDrugInfoErr
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

// GetLoggedNames returns a slice containing all unique names of drugs
// present in the local info table or log table.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// drugNamesErrorChannel - the goroutine channel used to return the list of
// drug names and the error
//
// info - if true will get names from local info table, if false from log table
//
// getExact - choose which column to get unique names for, if name is invalid,
// InvalidColInput error will be send through userLogsErrorChannel
func (cfg Config) GetLoggedNames(db *sql.DB, ctx context.Context,
	drugNamesErrorChannel chan<- DrugNamesError, info bool,
	username string, getExact string) {
	const printN string = "GetLoggedNames()"

	tempDrugNamesError := DrugNamesError{
		DrugNames: nil,
		Username:  username,
		Err:       nil,
	}

	useTable := loggingTableName
	logCols := validLogCols()
	addToStmt := ""
	if info == true {
		useTable = cfg.UseSource
		logCols = validInfoCols()
	} else {
		addToStmt = " where username = ?"
	}

	err := checkColIsInvalid(logCols, getExact, printN)
	if err != nil {
		tempDrugNamesError.Err = err
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}

	mainStmt := "select distinct " + getExact + " from " + useTable + addToStmt
	stmt, err := db.PrepareContext(ctx, mainStmt)
	if err != nil {
		tempDrugNamesError.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.PreapeContext()"), err)
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}

	var rows *sql.Rows
	if info == true {
		rows, err = db.QueryContext(ctx, mainStmt)
	} else {
		rows, err = stmt.QueryContext(ctx, username)
	}
	if err != nil {
		tempDrugNamesError.Err = fmt.Errorf("%s: %w", sprintName(printN, "db.QueryContext()"), err)
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}
	defer rows.Close()

	var drugList []string
	for rows.Next() {
		var holdName string
		err := rows.Scan(&holdName)
		if err != nil {
			tempDrugNamesError.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			drugNamesErrorChannel <- tempDrugNamesError
			return
		}

		drugList = append(drugList, holdName)
	}
	err = rows.Err()
	if err != nil {
		tempDrugNamesError.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}

	if len(drugList) == 0 {
		tempDrugNamesError.Err = fmt.Errorf("%s%w", sprintName(printN), EmptyListDrugNamesError)
	}

	tempDrugNamesError.DrugNames = drugList
	drugNamesErrorChannel <- tempDrugNamesError
}
