package drugdose

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"

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
	} else {
		return 0
	}
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

func GetTimes(driver string, path string, username string, source string, getid int, printit bool) *TimeTill {
	if username == "default" {
		username = default_username
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	commonSelect := "select timeOfDoseStart,timeOfDoseEnd,drugName,dose,drugRoute "

	queryVars := []interface{}{username}
	stmtTxt := commonSelect +
		"from userLogs where username = ? order by timeOfDoseStart desc limit 1"

	if getid != 0 {
		getidstr := strconv.Itoa(getid)
		stmtTxt = commonSelect +
			"from userLogs where username = ? and timeOfDoseStart = ?"
		queryVars = append(queryVars, getidstr)
	}

	rows, err := db.Query(stmtTxt, queryVars...)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer rows.Close()

	var starTime int64
	var endTime int64
	var drugName string
	var dose float32
	var drugRoute string
	for rows.Next() {
		err = rows.Scan(&starTime, &endTime, &drugName, &dose, &drugRoute)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	rows2, err := db.Query("select onsetMin,onsetMax,onsetUnits,comeUpMin,comeUpMax,comeUpUnits,"+
		"peakMin,peakMax,peakUnits,offsetMin,offsetMax,offsetUnits,"+
		"totalDurMin,totalDurMax,totalDurUnits,"+
		"lowDoseMin,lowDoseMax,threshold"+
		" from "+source+" where drugName = ? and drugRoute = ? limit 1", drugName, drugRoute)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer rows2.Close()

	var onsetMin float32
	var onsetMax float32
	var onsetUnits string
	var comeUpMin float32
	var comeUpMax float32
	var comeUpUnits string
	var peakMin float32
	var peakMax float32
	var peakUnits string
	var offsetMin float32
	var offsetMax float32
	var offsetUnits string
	var totalDurMin float32
	var totalDurMax float32
	var totalDurUnits string
	var lowDoseMin float32
	var lowDoseMax float32
	var threshold float32
	for rows2.Next() {
		err = rows2.Scan(&onsetMin, &onsetMax, &onsetUnits,
			&comeUpMin, &comeUpMax, &comeUpUnits,
			&peakMin, &peakMax, &peakUnits,
			&offsetMin, &offsetMax, &offsetUnits,
			&totalDurMin, &totalDurMax, &totalDurUnits,
			&lowDoseMin, &lowDoseMax, &threshold)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	}
	err = rows2.Err()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// No need to do further calculation, because if the source is correct,
	// in theory almost no effect should be accomplished with this dosage.
	if threshold != 0 && dose < threshold {
		fmt.Println("The dosage is below the source threshold.")
		fmt.Println("Will not calculate times.")
		return nil
	}

	convertToSeconds(onsetUnits, &onsetMin, &onsetMax)
	convertToSeconds(comeUpUnits, &comeUpMin, &comeUpMax)
	convertToSeconds(peakUnits, &peakMin, &peakMax)
	convertToSeconds(offsetUnits, &offsetMin, &offsetMax)
	convertToSeconds(totalDurUnits, &totalDurMin, &totalDurMax)

	curTime := time.Now().Unix()
	var useLoggedTime int64

	lightAvg := getAverage(lowDoseMin, lowDoseMax)

	useDose := lightAvg
	if threshold != 0 {
		useDose = threshold
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
	useLoggedTime = starTime
	if endTime != 0 && dose > useDose {
		totalSec := endTime - starTime

		// units per second
		ups := dose / float32(totalSec)

		// The point where over time units have accumulated
		// to the average low or threshold dose.
		// The problem with this is, because metabolism probably
		// doesn't work this way, there needs to be a more solid way
		// of determining average times, but as an experiment,
		// this will do for now.
		startOfLight := useDose / ups

		useLoggedTime = (int64(getAverage(startOfLight, float32(totalSec)))) + starTime
	} else if endTime != 0 && dose <= useDose {
		// Fall back to just average between start and finish, because we're below the light dose.
		// If there's no info about threshold, this is the best thing I came up with :D
		// If we're below the threshold, we won't even get to here.
		useLoggedTime = int64(getAverage(float32(starTime), float32(endTime)))
	}

	timeTill := TimeTill{}

	getDiffSinceLastLog := curTime - useLoggedTime

	timeTill.TotalCompleteMin = 1 //100%
	if float32(getDiffSinceLastLog) < totalDurMin {
		timeTill.TotalCompleteMin = float32(getDiffSinceLastLog) / totalDurMin
	}

	timeTill.TotalCompleteMax = 1 //100%
	if float32(getDiffSinceLastLog) < totalDurMax {
		timeTill.TotalCompleteMax = float32(getDiffSinceLastLog) / totalDurMax
	}
	onsetAvg := getAverage(onsetMin, onsetMax)
	comeupAvg := getAverage(comeUpMin, comeUpMax)
	peakAvg := getAverage(peakMin, peakMax)
	offsetAvg := getAverage(offsetMin, offsetMax)

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

	timeTill.StartDose = starTime
	timeTill.EnDose = endTime

	if printit {
		location, err := time.LoadLocation("Local")
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println("Warning: All data in here is approximations based on averages.")
		fmt.Println("Please don't let that influence the experience too much!")
		fmt.Println()
		fmt.Printf("Start Dose:\t%s (%d)\n", time.Unix(int64(starTime), 0).In(location), starTime)
		if endTime != 0 {
			fmt.Printf("End Dose:\t%s (%d)\n", time.Unix(int64(endTime), 0).In(location), endTime)
			if useLoggedTime != endTime {
				fmt.Printf("Offset End:\t%s (%d)\n",
					time.Unix(int64(useLoggedTime), 0).In(location), useLoggedTime)
			}
		}
		fmt.Printf("Current Time:\t%s (%d)\n", time.Unix(curTime, 0).In(location), curTime)
		fmt.Printf("Time passed:\t%d minutes\n", int(getDiffSinceLastLog/60))
		fmt.Println()

		fmt.Printf("Drug:\t%s\n", drugName)

		fmt.Printf("Total:\tMin: %d%% (of %d minutes) ; Max: %d%% (of %d minutes)\n",
			int(timeTill.TotalCompleteMin*100),
			int(math.Round(float64(totalDurMin)/60)),
			int(timeTill.TotalCompleteMax*100),
			int(math.Round(float64(totalDurMax)/60)))

		fmt.Println("Time left in minutes")
		fmt.Printf("Total:\tMin: %d ; Max: %d\n",
			int(math.Round(float64((totalDurMin-(timeTill.TotalCompleteMin*totalDurMin))/60))),
			int(math.Round(float64((totalDurMax-(timeTill.TotalCompleteMax*totalDurMax))/60))))

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
