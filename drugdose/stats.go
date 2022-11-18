package drugdose

import (
	"fmt"
	"math"
	"time"

	// SQLite driver needed for sql module, present in DB.go
	_ "github.com/mattn/go-sqlite3"
)

type TimeTill struct {
	// In seconds
	TimeTillOnset  int64
	TimeTillComeup int64
	TimeTillPeak   int64
	TimeTillOffset int64
	// Percentage of completion
	TotalCompleteMin float32
	TotalCompleteMax float32
	// In unix time
	StartDose int64
	EnDose    int64
}

func convertToSeconds(units string, min *float32, max *float32) {
	if units == "hours" || units == "h" {
		*min *= 60 * 60
		*max *= 60 * 60
	} else if units == "minutes" || units == "m" {
		*min *= 60
		*max *= 60
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

func (cfg Config) GetTimes(username string, getid int64, printit bool) *TimeTill {
	gotLogs := cfg.GetLogs(1, getid, username, false, true, false, "none")
	if gotLogs == nil {
		fmt.Println("GetTimes: No logs for getting the times.")
		return nil
	}

	useLog := gotLogs[0]
	gotInfo := cfg.GetLocalInfo(useLog.DrugName, false)

	gotInfoNum := -1
	for i := 0; i < len(gotInfo); i++ {
		if gotInfo[i].DrugRoute == useLog.DrugRoute {
			gotInfoNum = i
			break
		}
	}

	if gotInfoNum == -1 {
		fmt.Println("GetTimes: Logged drug route doesn't match anything in info database.")
		return nil
	}

	gotInfoProper := gotInfo[gotInfoNum]

	if gotInfoProper.DoseUnits != useLog.DoseUnits {
		fmt.Println("GetTimes: The logged dose units:", useLog.DoseUnits,
			"; don't match the local info database dose units:", gotInfoProper.DoseUnits)
		return nil
	}

	// No need to do further calculation, because if the source is correct,
	// in theory almost no effect should be accomplished with this dosage.
	if gotInfoProper.Threshold != 0 && useLog.Dose < gotInfoProper.Threshold {
		fmt.Println("GetTimes: The dosage is below the source threshold.")
		fmt.Println("GetTimes: Will not calculate times.")
		return nil
	}

	convertToSeconds(gotInfoProper.OnsetUnits,
		&gotInfoProper.OnsetMin,
		&gotInfoProper.OnsetMax)
	convertToSeconds(gotInfoProper.ComeUpUnits,
		&gotInfoProper.ComeUpMin,
		&gotInfoProper.ComeUpMax)
	convertToSeconds(gotInfoProper.PeakUnits,
		&gotInfoProper.PeakMin,
		&gotInfoProper.PeakMax)
	convertToSeconds(gotInfoProper.OffsetUnits,
		&gotInfoProper.OffsetMin,
		&gotInfoProper.OffsetMax)
	convertToSeconds(gotInfoProper.TotalDurUnits,
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

	timeTill.StartDose = useLog.StartTime
	timeTill.EnDose = useLog.EndTime

	if printit {
		location, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			fmt.Println("GetTimes: LoadLocation:", err)
			return nil
		}

		fmt.Println("Warning: All data in here is approximations based on averages.")
		fmt.Println("Please don't let that influence the experience too much!")
		fmt.Println()
		fmt.Printf("Start Dose:\t%q (%d)\n",
			time.Unix(useLog.StartTime, 0).In(location),
			useLog.StartTime)

		if useLog.EndTime != 0 {
			fmt.Printf("End Dose:\t%q (%d)\n",
				time.Unix(useLog.EndTime, 0).In(location),
				useLog.EndTime)

			if useLoggedTime != useLog.EndTime {
				fmt.Printf("Offset End:\t%q (%d)\n",
					time.Unix(useLoggedTime, 0).In(location), useLoggedTime)
			}
		}
		fmt.Printf("Current Time:\t%q (%d)\n", time.Unix(curTime, 0).In(location), curTime)
		fmt.Printf("Time passed:\t%d minutes\n", int(getDiffSinceLastLog/60))
		fmt.Println()

		fmt.Printf("Drug:\t%q\n", useLog.DrugName)
		fmt.Printf("Dose:\t%f\n", useLog.Dose)
		fmt.Printf("Units:\t%q\n\n", useLog.DoseUnits)

		fmt.Printf("Total:\tMin: %d%% (of %d minutes) ; Max: %d%% (of %d minutes)\n",
			int(timeTill.TotalCompleteMin*100),
			int(math.Round(float64(gotInfoProper.TotalDurMin)/60)),
			int(timeTill.TotalCompleteMax*100),
			int(math.Round(float64(gotInfoProper.TotalDurMax)/60)))

		fmt.Println("Time left in minutes")
		fmt.Printf("Total:\tMin: %d ; Max: %d\n",
			int(math.Round(float64((gotInfoProper.TotalDurMin-
				(timeTill.TotalCompleteMin*
					gotInfoProper.TotalDurMin))/60))),

			int(math.Round(float64((gotInfoProper.TotalDurMax-
				(timeTill.TotalCompleteMax*
					gotInfoProper.TotalDurMax))/60))))

		if onsetAvg != 0 {
			fmt.Printf("Onset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOnset)/60)))
		}

		if comeupAvg != 0 {
			fmt.Printf("Comeup:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillComeup/60))))
		}

		if peakAvg != 0 {
			fmt.Printf("Peak:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillPeak/60))))
		}

		if offsetAvg != 0 {
			fmt.Printf("Offset:\t%d (average)\n", int(math.Round(float64(timeTill.TimeTillOffset/60))))
		}
	}

	return &timeTill
}
