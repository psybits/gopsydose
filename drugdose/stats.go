package drugdose

import (
	"context"
	"fmt"
	"math"
	"time"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

type TimeTill struct {
	// In seconds
	TimeTillOnset  int64
	TimeTillComeup int64
	TimeTillPeak   int64
	TimeTillOffset int64
	TimeTillTotal  int64
	// Percentage of completion
	TotalCompleteMin float32
	TotalCompleteMax float32
	TotalCompleteAvg float32
	// In unix time
	StartDose int64
	EnDose    int64
}

func (cfg Config) convertToSeconds(db *sql.DB, ctx context.Context,
	units string, values ...*float32) {
	const printN string = "convertToSeconds()"

	units = cfg.MatchAndReplace(db, ctx, units, "units")
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

func (cfg Config) GetTimes(db *sql.DB, ctx context.Context,
	username string, getid int64, printit bool, prefix bool) *TimeTill {
	var printN string
	if prefix == true {
		printN = "GetTimes()"
	} else {
		printN = ""
	}

	userLogsErrChan := make(chan UserLogsError)
	go cfg.GetLogs(db, userLogsErrChan, ctx, 1, getid, username, true, "none")
	gotLogs := <-userLogsErrChan
	if gotLogs.Err != nil {
		printName(printN, gotLogs.Err)
		return nil
	}

	useLog := gotLogs.UserLogs[0]
	gotInfo := cfg.GetLocalInfo(db, ctx, useLog.DrugName)

	gotInfoNum := -1
	for i := 0; i < len(gotInfo); i++ {
		if gotInfo[i].DrugRoute == useLog.DrugRoute {
			gotInfoNum = i
			break
		}
	}

	if gotInfoNum == -1 {
		printName(printN, "Logged drug route doesn't match anything in info database.")
		return nil
	}

	gotInfoProper := gotInfo[gotInfoNum]

	if gotInfoProper.DoseUnits != useLog.DoseUnits {
		printName(printN, "The logged dose units:", useLog.DoseUnits,
			"; don't match the local info database dose units:", gotInfoProper.DoseUnits)
		return nil
	}

	// No need to do further calculation, because if the source is correct,
	// in theory almost no effect should be accomplished with this dosage.
	if gotInfoProper.Threshold != 0 && useLog.Dose < gotInfoProper.Threshold {
		printName(printN, "The dosage is below the source threshold.")
		printName(printN, "Will not calculate times.")
		return nil
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

	if printit {
		location, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			printName(printN, "LoadLocation:", err)
			return nil
		}

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
			printNameF(printN, "Finish Dose:\t%q (%d)\n",
				time.Unix(useLog.EndTime, 0).In(location),
				useLog.EndTime)

			printNameF(printN, "Adjust Finish:\t%q (%d)\n",
				time.Unix(useLoggedTime, 0).In(location), useLoggedTime)
		}
		printNameF(printN, "Current Time:\t%q (%d)\n", time.Unix(curTime, 0).In(location), curTime)

		printNameF(printN, "Time passed:\t%d minutes\n", int(getDiffSinceLastLog/60))
		fmt.Println()

		printNameF(printN, "Drug:\t%q\n", useLog.DrugName)
		printNameF(printN, "Dose:\t%f\n", useLog.Dose)
		printNameF(printN, "Units:\t%q\n\n", useLog.DoseUnits)

		printName(printN, "=== Time left in minutes until ===")

		if onsetAvg != 0 {
			printNameF(printN, "Onset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOnset)/60)))
		}

		if comeupAvg != 0 {
			printNameF(printN, "Comeup:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillComeup/60))))
		}

		if peakAvg != 0 {
			printNameF(printN, "Peak:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillPeak/60))))
		}

		if offsetAvg != 0 {
			printNameF(printN, "Offset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOffset/60))))
		}

		if totalAvg != 0 {
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
			int(math.Round(float64(totalAvg)/60)))

		printNameF(printN, "Total:\tMin: %d%% (of %d minutes) ; Max: %d%% (of %d minutes)\n",
			int(timeTill.TotalCompleteMin*100),
			int(math.Round(float64(gotInfoProper.TotalDurMin)/60)),
			int(timeTill.TotalCompleteMax*100),
			int(math.Round(float64(gotInfoProper.TotalDurMax)/60)))
	}

	return &timeTill
}
