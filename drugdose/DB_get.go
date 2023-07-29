package drugdose

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"
)

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
// db - open database connection
//
// ctx - context to be passed to sql queries
func (cfg Config) GetUsers(db *sql.DB, ctx context.Context) (error, []string) {
	const printN string = "GetUsers()"

	var allUsers []string
	var tempUser string

	rows, err := db.QueryContext(ctx, "select distinct username from "+loggingTableName)
	if err != nil {
		err = errors.New(sprintName(printN, "Query: error getting usernames:", err))
		return err, nil
	}

	for rows.Next() {
		err = rows.Scan(&tempUser)
		if err != nil {
			err = errors.New(sprintName(printN, "Scan: error getting usernames:", err))
			return err, nil
		}
		allUsers = append(allUsers, tempUser)
	}

	return nil, allUsers
}

// GetLogsCount returns total amount of logs in
// the dose log table for username set in user parameter.
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

// GetLogs returns all logs for a given username in the drug log table.
// It uses a single channel with the type UserLogsError, containing a slice of
// UserLogs structs and a variable with an error type. When using this function,
// the error must be checked before reading the logs. Every log is a separate
// element of the UserLogs slice.
//
// This function is meant to be run concurrently.
//
// db - an open database connection
//
// ctx - context that will be passed to the sql query function
//
// userLogsErrorChannel - the goroutine channel used to return the logs and
// the error
//
// num - amount of logs to return (limit), if 0 returns all logs (without limit)
//
// id - if not 0, will return the exact log matching that id for the given user
//
// user - the user which created the logs, will returns only the logs for that
// username
//
// reverse - if true go from high values to low values,
// this should return the newest logs first, false is the opposite direction
//
// search - return logs only matching this string
func (cfg Config) GetLogs(db *sql.DB, ctx context.Context,
	userLogsErrorChannel chan<- UserLogsError, num int, id int64,
	user string, reverse bool,
	search string) {

	printN := "GetLogs()"

	numstr := strconv.Itoa(num)

	userlogs := []UserLog{}
	tempUserLogsError := UserLogsError{
		UserLogs: userlogs,
		Err:      nil,
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
		tempUserLogsError.Err = errors.New(sprintName(printN, "db.QueryContext(): ", err))
		tempUserLogsError.UserLogs = userlogs
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	defer rows.Close()

	for rows.Next() {
		tempul := UserLog{}
		err = rows.Scan(&tempul.StartTime, &tempul.Username, &tempul.EndTime, &tempul.DrugName,
			&tempul.Dose, &tempul.DoseUnits, &tempul.DrugRoute, &tempul.Cost, &tempul.CostCurrency)
		if err != nil {
			tempUserLogsError.Err = errors.New(sprintName(printN, "rows.Next(): ", err))
			tempUserLogsError.UserLogs = userlogs
			userLogsErrorChannel <- tempUserLogsError
			return
		}

		userlogs = append(userlogs, tempul)
	}
	err = rows.Err()
	if err != nil {
		tempUserLogsError.Err = errors.New(sprintName(printN, "rows.Err(): ", err))
		tempUserLogsError.UserLogs = userlogs
		userLogsErrorChannel <- tempUserLogsError
		return
	}
	if len(userlogs) == 0 {
		tempUserLogsError.Err = errors.New(sprintName(printN, "No logs returned for user: ", user))
		tempUserLogsError.UserLogs = userlogs
		userLogsErrorChannel <- tempUserLogsError
		return
	}

	tempUserLogsError.Err = nil
	tempUserLogsError.UserLogs = userlogs
	userLogsErrorChannel <- tempUserLogsError
}

// GetLocalInfoNames returns a slice containing all unique names of drugs
// present in the local database gotten from a source.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// drugNamesErrorChannel - the goroutine channel used to return the list of
// drug names and the error
func (cfg Config) GetLocalInfoNames(db *sql.DB, ctx context.Context,
	drugNamesErrorChannel chan<- DrugNamesError) {
	const printN string = "GetLocalInfoNames()"

	tempDrugNamesError := DrugNamesError{
		DrugNames: nil,
		Err:       nil,
	}

	rows, err := db.QueryContext(ctx, "select distinct drugName from "+cfg.UseSource)
	if err != nil {
		tempDrugNamesError.Err = errors.New(sprintName(printN, err))
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}
	defer rows.Close()

	var drugList []string
	for rows.Next() {
		var holdName string
		err := rows.Scan(&holdName)
		if err != nil {
			tempDrugNamesError.Err = errors.New(sprintName(printN, err))
			drugNamesErrorChannel <- tempDrugNamesError
			return
		}

		drugList = append(drugList, holdName)
	}
	err = rows.Err()
	if err != nil {
		tempDrugNamesError.Err = errors.New(sprintName(printN, err))
		drugNamesErrorChannel <- tempDrugNamesError
		return
	}

	tempDrugNamesError.DrugNames = drugList
	drugNamesErrorChannel <- tempDrugNamesError
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
func (cfg Config) GetLocalInfo(db *sql.DB, ctx context.Context,
	drugInfoErrChan chan<- DrugInfoError, drug string) {
	printN := "GerLocalInfo()"

	drug = cfg.MatchAndReplace(db, ctx, drug, "substance")

	tempDrugInfoErr := DrugInfoError{
		DrugI: nil,
		Err:   nil,
	}

	ret := checkIfExistsDB(db, ctx,
		"drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		tempDrugInfoErr.Err = errors.New(sprintName(printN, "No such drug in info database: ", drug))
		drugInfoErrChan <- tempDrugInfoErr
		return
	}

	rows, err := db.QueryContext(ctx, "select * from "+cfg.UseSource+" where drugName = ?", drug)
	if err != nil {
		tempDrugInfoErr.Err = errors.New(sprintName(printN, err))
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
			tempDrugInfoErr.Err = errors.New(sprintName(printN, err))
			drugInfoErrChan <- tempDrugInfoErr
			return
		}

		infoDrug = append(infoDrug, tempdrinfo)
	}
	err = rows.Err()
	if err != nil {
		tempDrugInfoErr.Err = errors.New(sprintName(printN, err))
		drugInfoErrChan <- tempDrugInfoErr
		return
	}

	tempDrugInfoErr.DrugI = infoDrug
	drugInfoErrChan <- tempDrugInfoErr
}
