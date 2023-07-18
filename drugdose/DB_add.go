package drugdose

import (
	"context"
	"errors"
	"time"

	"database/sql"	
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

// AddToInfoDB uses subs[] to fill up the currently configured source table
// in the database. subs[] has to be filled prior to calling the function.
// This is usually achieved by fetching data from a source using it's API.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// subs - all substances of type DrugInfo to go through to add to source table
func (cfg Config) AddToInfoDB(db *sql.DB, ctx context.Context, errChannel chan error, subs []DrugInfo) {
	const printN string = "AddToInfoDB()"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, err))
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
	if handleErrRollback(err, tx, errChannel, printN, "tx.Prepare(): ") {
		return
	}

	defer stmt.Close()
	for i := 0; i < len(subs); i++ {
		subs[i].DoseUnits = cfg.MatchAndReplace(db, ctx, subs[i].DoseUnits, "units")
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
		if handleErrRollback(err, tx, errChannel, printN, "stmt.Exec(): ") {
			return
		}
	}
	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, printN, "tx.Commit(): ") {
		return
	}

	errChannel <- nil
}

func (cfg Config) AddToDoseDB(db *sql.DB, ctx context.Context, errChannel chan error,
	synct *SyncTimestamps, user string, drug string, route string,
	dose float32, units string, perc float32, printit bool) {

	const printN string = "AddToDoseDB()"

	drug = cfg.MatchAndReplace(db, ctx, drug, "substance")
	route = cfg.MatchAndReplace(db, ctx, route, "route")
	units = cfg.MatchAndReplace(db, ctx, units, "units")

	if perc != 0 {
		dose, units = cfg.ConvertUnits(db, ctx, drug, dose, perc)
		if dose == 0 || units == "" {
			errChannel <- errors.New(sprintfName(printN, "Error converting units for drug: %q"+
				" ; dose: %g ; perc: %g ; units: %q", drug, dose, perc, units))
			return
		}
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB(db, ctx,
		"drugName", cfg.UseSource,
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		xtrs[:], drug, route, units)
	if !ret {
		errChannel <- errors.New(sprintfName(printN, "Combo of Drug: %q"+
			" ; Route: %q"+
			" ; Units: %q"+
			" ; doesn't exist in local information database.", drug, route, units))
		return
	}

	var count uint32
	err, count := cfg.GetLogsCount(db, ctx, user)
	if err != nil {
		errChannel <- errors.New(sprintName(printN, err))
		return
	}

	// get lock
	synct.Lock.Lock()

	if MaxLogsPerUserSize(count) >= cfg.MaxLogsPerUser {
		diff := count - uint32(cfg.MaxLogsPerUser)
		if cfg.AutoRemove {
			errChannel2 := make(chan error)
			go cfg.RemoveLogs(db, ctx, errChannel2, user, int(diff+1), true, 0, "none")
			gotErr := <-errChannel2
			if gotErr != nil {
				errChannel <- gotErr
				// release lock
				synct.Lock.Unlock()
				return
			}
		} else {
			errChannel <- errors.New(sprintName(printN, "User:", user,
				"has reached the maximum entries per user:", cfg.MaxLogsPerUser, "; Not logging."))
			return
		}
	}

	// release lock
	synct.Lock.Unlock()

	// Add to log db
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		errChannel <- err
		return
	}

	stmt, err := tx.Prepare("insert into " + loggingTableName +
		" (timeOfDoseStart, username, timeOfDoseEnd, drugName, dose, doseUnits, drugRoute) " +
		"values(?, ?, ?, ?, ?, ?, ?)")
	if handleErrRollback(err, tx, errChannel, printN, "tx.Prepare(): ") {
		return
	}
	defer stmt.Close()

	// get lock
	synct.Lock.Lock()

	currTime := time.Now().Unix()
	if currTime == synct.LastTimestamp && user == synct.LastUser {
		time.Sleep(time.Second)
		currTime = time.Now().Unix()
	}

	_, err = stmt.Exec(currTime, user, 0, drug, dose, units, route)
	if handleErrRollback(err, tx, errChannel, printN, "stmt.Exec(): ") {
		// release lock
		synct.Lock.Unlock()
		return
	}
	err = tx.Commit()
	if handleErrRollback(err, tx, errChannel, printN, "tx.Commit(): ") {
		// release lock
		synct.Lock.Unlock()
		return
	}

	synct.LastTimestamp = currTime
	synct.LastUser = user

	// release lock
	synct.Lock.Unlock()

	if printit {
		printNameF(printN, "Logged: drug: %q ; dose: %g ; units: %q ; route: %q ; username: %q\n",
			drug, dose, units, route, user)
	}

	errChannel <- nil
}
