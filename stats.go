package drugdose

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
)

type TimeTill struct {
	// In seconds
	//
	// Information for the different points is from Psychonautwiki.
	//
	// The onset phase can be defined as the period until the very first
	// changes in perception (i.e. "first alerts") are able to be detected.
	TimeTillOnset int64
	// The "come up" phase can be defined as the period between the first
	// noticeable changes in perception and the point of highest subjective
	// intensity.
	TimeTillComeup int64
	// The peak phase can be defined as period of time in which the
	// intensity of the substance's effects are at its height.
	TimeTillPeak int64
	// The offset phase can be defined as the amount of time in between the
	// conclusion of the peak and shifting into a sober state.
	TimeTillOffset int64
	// The total duration of a substance can be defined as the amount of
	// time it takes for the effects of a substance to completely wear off
	// into sobriety, starting from the moment the substance is first
	// administered.
	TimeTillTotal int64
	// Percentage of completion
	TotalCompleteMin float32
	TotalCompleteMax float32
	TotalCompleteAvg float32
	// In unix time
	StartDose int64
	EnDose    int64
}

type TimeTillError struct {
	TimeT    *TimeTill
	Username string
	Err      error
	// Bellow is extra information only needed internally
	useLog        UserLog
	approxEnd     int64
	useLoggedTime int64
	onsetAvg      float32
	comeupAvg     float32
	peakAvg       float32
	offsetAvg     float32
	totalAvg      float32
	gotInfoProper DrugInfo
}

func (cfg Config) convertToSeconds(db *sql.DB, ctx context.Context,
	units string, values ...*float32) {
	const printN string = "convertToSeconds()"

	units = cfg.MatchAndReplace(db, ctx, units, NameTypeUnits)
	if units == "hours" {
		for _, value := range values {
			*value *= 60 * 60
		}
	} else if units == "minutes" {
		for _, value := range values {
			*value *= 60
		}
	} else {
		printName(printN, "unit:", units, "; is not valid, didn't convert")
	}
}

func getAverage(first float32, second float32) float32 {
	if first+second != 0 {
		return (first + second) / 2
	}

	return 0
}

func calcTimeTill(timetill *int64, diff int64, average ...float32) {
	*timetill = 0
	var total float32 = 0
	for _, v := range average {
		total += v
	}
	if float32(diff) < total {
		*timetill = int64(math.Round(float64(total - float32(diff))))
	}
}

// GetTimes returns the times till reaching a specific point of the experience.
// The points are defined in the TimeTill struct. PrintTimeTill() can be used
// to output the information gathered in this function to the terminal.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// timeTillErrChan - the goroutine channel which returns the TimeTill struct
// and an error
// (set to nil if function doesn't need to be concurrent)
//
// username - the user for which to get the information
//
// getid - if 0 gives information about the last log, a specific ID can be
// passed to get the times for that log
func (cfg Config) GetTimes(db *sql.DB, ctx context.Context,
	timeTillErrChan chan<- TimeTillError, username string, getid int64) TimeTillError {
	const printN string = "GetTimes()"

	tempTimeTillErr := TimeTillError{
		Err:      nil,
		Username: "",
		TimeT:    nil,
	}

	gotLogs := cfg.GetLogs(db, ctx, nil, 1, getid, username, true, "none", "")
	if gotLogs.Err != nil {
		tempTimeTillErr.Err = fmt.Errorf("%s%w", sprintName(printN), gotLogs.Err)
		if timeTillErrChan != nil {
			timeTillErrChan <- tempTimeTillErr
		}
		return tempTimeTillErr
	}

	useLog := gotLogs.UserLogs[0]

	gotDrugInfoErr := cfg.GetLocalInfo(db, ctx, nil, useLog.DrugName, username)
	gotInfo := gotDrugInfoErr.DrugI
	err := gotDrugInfoErr.Err
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
		tempTimeTillErr.Err = err
		if timeTillErrChan != nil {
			timeTillErrChan <- tempTimeTillErr
		}
		return tempTimeTillErr
	}

	gotInfoNum := -1
	for i := 0; i < len(gotInfo); i++ {
		if gotInfo[i].DrugRoute == useLog.DrugRoute {
			gotInfoNum = i
			break
		}
	}

	if gotInfoNum == -1 {
		tempTimeTillErr.Err = fmt.Errorf("%s%w", sprintName(printN), LoggedRouteInfoError)
		if timeTillErrChan != nil {
			timeTillErrChan <- tempTimeTillErr
		}
		return tempTimeTillErr
	}

	gotInfoProper := gotInfo[gotInfoNum]

	if gotInfoProper.DoseUnits != useLog.DoseUnits {
		tempTimeTillErr.Err = fmt.Errorf("%s%w: %s ; info table units: %s",
			sprintName(printN), LoggedUnitsInfoError,
			useLog.DoseUnits, gotInfoProper.DoseUnits)
		if timeTillErrChan != nil {
			timeTillErrChan <- tempTimeTillErr
		}
		return tempTimeTillErr
	}

	// No need to do further calculation, because if the source is correct,
	// in theory almost no effect should be accomplished with this dosage.
	if gotInfoProper.Threshold != 0 && useLog.Dose < gotInfoProper.Threshold {
		tempTimeTillErr.Err = fmt.Errorf("%s%w: %s", sprintName(printN),
			DoseBelowThresholdError, "will not calculate times")
		if timeTillErrChan != nil {
			timeTillErrChan <- tempTimeTillErr
		}
		return tempTimeTillErr
	}

	cfg.convertToSeconds(db, ctx,
		gotInfoProper.OnsetUnits,
		&gotInfoProper.OnsetMin,
		&gotInfoProper.OnsetMax)
	cfg.convertToSeconds(db, ctx,
		gotInfoProper.ComeUpUnits,
		&gotInfoProper.ComeUpMin,
		&gotInfoProper.ComeUpMax)
	cfg.convertToSeconds(db, ctx,
		gotInfoProper.PeakUnits,
		&gotInfoProper.PeakMin,
		&gotInfoProper.PeakMax)
	cfg.convertToSeconds(db, ctx,
		gotInfoProper.OffsetUnits,
		&gotInfoProper.OffsetMin,
		&gotInfoProper.OffsetMax)
	cfg.convertToSeconds(db, ctx,
		gotInfoProper.TotalDurUnits,
		&gotInfoProper.TotalDurMin,
		&gotInfoProper.TotalDurMax)

	curTime := time.Now().Unix()
	var useLoggedTime int64

	lightAvg := getAverage(gotInfoProper.LowDoseMin, gotInfoProper.LowDoseMax)

	useDose := lightAvg
	if gotInfoProper.Threshold != 0 {
		useDose = gotInfoProper.Threshold
	}

	// This is used when for example you're drinking a glass of beer.
	// What timing should be used when you've been drinking for 30 mins for example?
	// The solution I came up with (which is pretty unreliable probably) is just
	// to calculate how many units per second it takes to get to the threshold/light dosage
	// and start timing onset and etc. using the average of that point and the time you've finished.
	// The reason it's an average is because you actually keep on consuming until the finish most likely,
	// so it's best to take that middle point and go from there.
	// This will be used until proper metabolism is taken into consideration.
	// Eventually age, weight and gender needs to be considered.
	useLoggedTime = useLog.StartTime
	if useLog.EndTime != 0 && useLog.Dose > useDose {
		totalSec := useLog.EndTime - useLog.StartTime

		// units per second
		ups := useLog.Dose / float32(totalSec)

		// The point where over time units have accumulated
		// to the average low or threshold dose.
		// The problem with this is, because metabolism probably
		// doesn't work this way, there needs to be a more solid way
		// of determining average times, but as an experiment,
		// this will do for now.
		startOfLight := useDose / ups

		useLoggedTime = (int64(getAverage(startOfLight, float32(totalSec)))) + useLog.StartTime
	} else if useLog.EndTime != 0 && useLog.Dose <= useDose {
		// Fall back to just average between start and finish, because we're below the light dose.
		// If there's no info about threshold, this is the best thing I came up with :D
		// If we're below the threshold, we won't even get to here.
		useLoggedTime = int64(getAverage(float32(useLog.StartTime), float32(useLog.EndTime)))
	}

	timeTill := TimeTill{}

	getDiffSinceLastLog := curTime - useLoggedTime

	timeTill.TotalCompleteMin = 1 //100%
	if float32(getDiffSinceLastLog) < gotInfoProper.TotalDurMin {
		timeTill.TotalCompleteMin = float32(getDiffSinceLastLog) / gotInfoProper.TotalDurMin
	}

	timeTill.TotalCompleteMax = 1 //100%
	if float32(getDiffSinceLastLog) < gotInfoProper.TotalDurMax {
		timeTill.TotalCompleteMax = float32(getDiffSinceLastLog) / gotInfoProper.TotalDurMax
	}

	onsetAvg := getAverage(gotInfoProper.OnsetMin, gotInfoProper.OnsetMax)
	comeupAvg := getAverage(gotInfoProper.ComeUpMin, gotInfoProper.ComeUpMax)
	peakAvg := getAverage(gotInfoProper.PeakMin, gotInfoProper.PeakMax)
	offsetAvg := getAverage(gotInfoProper.OffsetMin, gotInfoProper.OffsetMax)
	totalAvg := getAverage(gotInfoProper.TotalDurMin, gotInfoProper.TotalDurMax)

	timeTill.TotalCompleteAvg = 1 //100%
	if float32(getDiffSinceLastLog) < totalAvg {
		timeTill.TotalCompleteAvg = float32(getDiffSinceLastLog) / totalAvg
	}

	if onsetAvg != 0 {
		calcTimeTill(&timeTill.TimeTillOnset, getDiffSinceLastLog, onsetAvg)
	}

	if comeupAvg != 0 {
		calcTimeTill(&timeTill.TimeTillComeup, getDiffSinceLastLog, onsetAvg, comeupAvg)
	}

	if peakAvg != 0 {
		calcTimeTill(&timeTill.TimeTillPeak, getDiffSinceLastLog, onsetAvg, comeupAvg, peakAvg)
	}

	if offsetAvg != 0 {
		calcTimeTill(&timeTill.TimeTillOffset, getDiffSinceLastLog, onsetAvg, comeupAvg, peakAvg, offsetAvg)
	}

	if totalAvg != 0 {
		calcTimeTill(&timeTill.TimeTillTotal, getDiffSinceLastLog, totalAvg)
	}

	timeTill.StartDose = useLog.StartTime
	timeTill.EnDose = useLog.EndTime

	var approxEnd int64 = useLoggedTime + int64(totalAvg)

	tempTimeTillErr.Username = username
	tempTimeTillErr.TimeT = &timeTill
	tempTimeTillErr.useLog = useLog
	tempTimeTillErr.approxEnd = approxEnd
	tempTimeTillErr.useLoggedTime = useLoggedTime
	tempTimeTillErr.onsetAvg = onsetAvg
	tempTimeTillErr.comeupAvg = comeupAvg
	tempTimeTillErr.peakAvg = peakAvg
	tempTimeTillErr.offsetAvg = offsetAvg
	tempTimeTillErr.totalAvg = totalAvg
	tempTimeTillErr.gotInfoProper = gotInfoProper
	if timeTillErrChan != nil {
		timeTillErrChan <- tempTimeTillErr
	}
	return tempTimeTillErr
}

// PrintTimeTill prints the information gotten using GetTimes() to the terminal.
//
// timeTillErr - the struct returned from GetTimes()
//
// prefix - if true, adds the function name to every print
func (cfg Config) PrintTimeTill(timeTillErr TimeTillError, prefix bool) error {
	var printN string
	if prefix == true {
		printN = "GetTimes()"
	} else {
		printN = ""
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN, "LoadLocation: "), err)
		return err
	}

	timeTill := timeTillErr.TimeT
	useLog := timeTillErr.useLog
	approxEnd := timeTillErr.approxEnd
	useLoggedTime := timeTillErr.useLoggedTime
	gotInfoProper := timeTillErr.gotInfoProper

	printName(printN, "Warning: All data in here is approximations based on averages.")
	printName(printN, "Please don't let that influence the experience too much!")
	fmt.Println()
	printNameF(printN, "Start Dose:\t%q (%d)\n",
		time.Unix(useLog.StartTime, 0).In(location),
		useLog.StartTime)
	if approxEnd != useLog.StartTime {
		printNameF(printN, "Approx. End:\t%q (%d)\n",
			time.Unix(approxEnd, 0).In(location),
			approxEnd)
	}

	if useLog.EndTime != 0 {
		printNameF(printN, "\nFinish Dose:\t%q (%d)\n",
			time.Unix(useLog.EndTime, 0).In(location),
			useLog.EndTime)

		printNameF(printN, "Adjust Finish:\t%q (%d)\n",
			time.Unix(useLoggedTime, 0).In(location), useLoggedTime)
	}
	curTime := time.Now().Unix()
	printNameF(printN, "\nCurrent Time:\t%q (%d)\n", time.Unix(curTime, 0).In(location), curTime)

	getDiffSinceLastLog := curTime - useLoggedTime
	printNameF(printN, "Time passed:\t%d minutes\n", int(getDiffSinceLastLog/60))
	fmt.Println()
	printNameF(printN, "Drug:\t%q\n", useLog.DrugName)
	printNameF(printN, "Dose:\t%f\n", useLog.Dose)
	printNameF(printN, "Units:\t%q\n", useLog.DoseUnits)
	printNameF(printN, "Route:\t%q\n\n", useLog.DrugRoute)

	printName(printN, "=== Time left in minutes until ===")

	if timeTillErr.onsetAvg != 0 {
		printNameF(printN, "Onset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOnset)/60)))
	}

	if timeTillErr.comeupAvg != 0 {
		printNameF(printN, "Comeup:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillComeup/60))))
	}

	if timeTillErr.peakAvg != 0 {
		printNameF(printN, "Peak:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillPeak/60))))
	}

	if timeTillErr.offsetAvg != 0 {
		printNameF(printN, "Offset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOffset/60))))
	}

	if timeTillErr.totalAvg != 0 {
		printNameF(printN, "Total:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillTotal/60))))
	}

	printNameF(printN, "Total:\tMin: %d ; Max: %d\n",
		int(math.Round(
			float64(
				(gotInfoProper.TotalDurMin-
					(timeTill.TotalCompleteMin*gotInfoProper.TotalDurMin))/60))),
		int(math.Round(
			float64(
				(gotInfoProper.TotalDurMax-
					(timeTill.TotalCompleteMax*gotInfoProper.TotalDurMax))/60))))

	printName(printN, "=== Percentage of time left completed ===")

	printNameF(printN, "Total:\t%d%% (of %d average minutes)\n",
		int(timeTill.TotalCompleteAvg*100),
		int(math.Round(float64(timeTillErr.totalAvg)/60)))

	printNameF(printN, "Total:\tMin: %d%% (of %d minutes) ; Max: %d%% (of %d minutes)\n",
		int(timeTill.TotalCompleteMin*100),
		int(math.Round(float64(gotInfoProper.TotalDurMin)/60)),
		int(timeTill.TotalCompleteMax*100),
		int(math.Round(float64(gotInfoProper.TotalDurMax)/60)))

	return nil
}

var LoggedRouteInfoError error = errors.New("dose route doesn't match anything in info table")
var LoggedUnitsInfoError error = errors.New("dose units don't match anything in info table")
var DoseBelowThresholdError error = errors.New("the dosage is below the source threshold")
