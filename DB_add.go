package drugdose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
)

// AddToInfoTable uses subs[] to fill up the currently configured source table
// in the database. subs[] has to be filled prior to calling the function.
// This is usually achieved by fetching data from a source using it's API.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
//
// subs - all substances of type DrugInfo to go through to add to source table
//
// username - user requesting addition
func (cfg Config) AddToInfoTable(db *sql.DB, ctx context.Context,
	errChannel chan<- ErrorInfo, subs []DrugInfo, username string) {
	const printN string = "AddToInfoTable()"

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Username: username,
		Action:   ActionAddToInfoTable,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		errChannel <- tempErrInfo
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
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Prepare(): ") {
		return
	}

	defer stmt.Close()
	for i := 0; i < len(subs); i++ {
		subs[i].DoseUnits = cfg.MatchAndReplace(db, ctx, subs[i].DoseUnits, NameTypeUnits)
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
		if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "stmt.Exec(): ") {
			return
		}
	}
	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Commit(): ") {
		return
	}

	errChannel <- tempErrInfo
}

// AddToDoseTable adds a new logged dose to the local database.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// errChannel - the gorouting channel which returns the errors
// (set to nil if function doesn't need to be concurrent)
//
// synct - pointer to SyncTimestamps struct used for synchronizing all AddToDoseTable() goroutines,
// it makes sure no conflicts happen when new doses are added
// (set to nil if function doesn't need to be concurrent)
//
// user - the username to log, if the same timestamps for the same username are chosen,
// the function will increment them all with 1 second to avoid conflicts
//
// drug - the name of the drug to log, it has to be present in the local info (source) database
//
// route - the name of the route to log, examples begin oral, smoked, etc. and it has
// to be present in the local info (source) database for the given drug
//
// dose - the amount of drug to log
//
// units - the units to be used for dose (amount)
//
// perc - when not 0, will attempt to convert the amount and units to new amount and units
// according to the configurations present in the database, checkout ConvertUnits() in
// names.go for more information on how this works
//
// cost - the cost in money for the log, it has to be calculated manually
// using the total amount paid
//
// costCur - the currency the cost is in
//
// printit - when true, prints what has been added to the database in the terminal
func (cfg Config) AddToDoseTable(db *sql.DB, ctx context.Context, errChannel chan<- ErrorInfo,
	synct *SyncTimestamps, user string, drug string, route string,
	dose float32, units string, perc float32, cost float32, costCur string,
	printit bool) ErrorInfo {

	const printN string = "AddToDoseTable()"

	drug = cfg.MatchAndReplace(db, ctx, drug, NameTypeSubstance)
	route = cfg.MatchAndReplace(db, ctx, route, NameTypeRoute)
	units = cfg.MatchAndReplace(db, ctx, units, NameTypeUnits)

	tempErrInfo := ErrorInfo{
		Err:      nil,
		Action:   ActionAddToDoseTable,
		Username: user,
	}

	var err error = nil
	if perc != 0 {
		err, dose, units = cfg.ConvertUnits(db, ctx, drug, dose, perc)
		if err != nil {
			tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			if errChannel != nil {
				errChannel <- tempErrInfo
			}
			return tempErrInfo
		}
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB(db, ctx,
		"drugName", cfg.UseSource,
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		xtrs[:], drug, route, units)
	if !ret {
		tempErrInfo.Err = fmt.Errorf("%s%w: %s", sprintName(printN), ComboInputError,
			fmt.Sprintf("Drug: %q"+
				" ; Route: %q"+
				" ; Units: %q",
				drug, route, units))
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	var count uint32
	gotLogCountErr := cfg.GetLogsCount(db, ctx, user, nil)
	err = gotLogCountErr.Err
	count = gotLogCountErr.LogCount
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	if errChannel != nil && synct != nil {
		// get lock
		synct.Lock.Lock()
	}

	if MaxLogsPerUserSize(count) >= cfg.MaxLogsPerUser {
		diff := count - uint32(cfg.MaxLogsPerUser)
		if cfg.AutoRemove {
			gotErrInfo := cfg.RemoveLogs(db, ctx, nil, user, int(diff+1), true, 0, "none", "")
			if gotErrInfo.Err != nil {
				tempErrInfo.Err = fmt.Errorf("%s%w", sprintName(printN), gotErrInfo.Err)
				if errChannel != nil && synct != nil {
					errChannel <- tempErrInfo
					// release lock
					synct.Lock.Unlock()
				}
				return tempErrInfo
			}
		} else {
			tempErrInfo.Err = fmt.Errorf("%s: %w: %q ; Not logging", sprintName(printN, "User:", user),
				MaxLogsPerUserError, cfg.MaxLogsPerUser)
			if errChannel != nil && synct != nil {
				errChannel <- tempErrInfo
				// release lock
				synct.Lock.Unlock()
			}
			return tempErrInfo
		}
	}

	if errChannel != nil && synct != nil {
		// release lock
		synct.Lock.Unlock()
	}

	// Add to log db
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		tempErrInfo.Err = fmt.Errorf("%s%s: %w", sprintName(printN), "db.BeginTx()", err)
		if errChannel != nil {
			errChannel <- tempErrInfo
		}
		return tempErrInfo
	}

	stmt, err := tx.Prepare("insert into " + loggingTableName +
		" (timeOfDoseStart, username, drugName, dose, doseUnits, drugRoute, cost, costCurrency) " +
		"values(?, ?, ?, ?, ?, ?, ?, ?)")
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Prepare(): ") {
		return tempErrInfo
	}
	defer stmt.Close()

	if errChannel != nil && synct != nil {
		// get lock
		synct.Lock.Lock()
	}

	currTime := time.Now().Unix()
	if errChannel != nil && synct != nil {
		if currTime <= synct.LastTimestamp && user == synct.LastUser {
			currTime = synct.LastTimestamp + 1
		}
	}

	if costCur == "" && cost != 0 {
		costCur = cfg.CostCurrency
	}

	_, err = stmt.Exec(currTime, user, drug, dose, units, route, cost, costCur)
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "stmt.Exec(): ") {
		if errChannel != nil && synct != nil {
			// release lock
			synct.Lock.Unlock()
		}
		return tempErrInfo
	}
	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, &tempErrInfo, printN, "tx.Commit(): ") {
		if errChannel != nil && synct != nil {
			// release lock
			synct.Lock.Unlock()
		}
		return tempErrInfo
	}

	if errChannel != nil && synct != nil {
		synct.LastTimestamp = currTime
		synct.LastUser = user
	}

	if errChannel != nil && synct != nil {
		// release lock
		synct.Lock.Unlock()
	}

	if printit {
		printNameF(printN, "Logged: drug: %q ; dose: %g ; units: %q ; route: %q ; username: %q "+
			"; cost: %g ; costCurrency: %q\n",
			drug, dose, units, route, user, cost, costCur)
	}

	if errChannel != nil {
		errChannel <- tempErrInfo
	}
	return tempErrInfo
}

var ComboInputError error = errors.New("combo of input parameters not in database")
var MaxLogsPerUserError error = errors.New("reached the maximum entries per user")
