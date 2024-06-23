package drugdose

import (
	"context"
	"errors"
	"fmt"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
)

type Cost struct {
	Substance    string
	TotalCost    float32
	CostCurrency string
}

type CostsError struct {
	Costs []Cost
	Err   error
}

// GetTotalCosts returns a slice containing all costs about all drugs in all
// currencies.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// costsErrChan - the goroutine channel used to return the slice containing
// all costs
// (set to nil if function doesn't need to be concurrent)
//
// username - the user for which to return the costs
func (cfg *Config) GetTotalCosts(db *sql.DB, ctx context.Context,
	costsErrChan chan<- CostsError, username string) CostsError {

	const printN string = "GetTotalCosts()"

	tempCostsErr := CostsError{
		Costs: nil,
		Err:   nil,
	}

	gotDrugNamesErr := cfg.GetLoggedNames(db, ctx, nil, false, username, LogDrugNameCol)
	err := gotDrugNamesErr.Err
	uniqueDrugNames := gotDrugNamesErr.DrugNames
	if err != nil {
		tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		if costsErrChan != nil {
			costsErrChan <- tempCostsErr
		}
		return tempCostsErr
	}

	gotDrugNamesErr = cfg.GetLoggedNames(db, ctx, nil, false, username, LogCostCurrencyCol)
	err = gotDrugNamesErr.Err
	uniqueCurrencyNames := gotDrugNamesErr.DrugNames
	if err != nil {
		tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		if costsErrChan != nil {
			costsErrChan <- tempCostsErr
		}
		return tempCostsErr
	}

	var gotUserLogsErr UserLogsError
	for i := 0; i < len(uniqueDrugNames); i++ {
		gotUserLogsErr = cfg.GetLogs(db, ctx, nil, 0, 0, username,
			true, uniqueDrugNames[i], LogDrugNameCol)
		err = gotUserLogsErr.Err
		if err != nil {
			tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			if costsErrChan != nil {
				costsErrChan <- tempCostsErr
			}
			return tempCostsErr
		}

		for o := 0; o < len(uniqueCurrencyNames); o++ {
			tempCost := Cost{
				Substance:    uniqueDrugNames[i],
				TotalCost:    0,
				CostCurrency: uniqueCurrencyNames[o],
			}
			tempCostsErr.Costs = append(tempCostsErr.Costs, tempCost)
		}

		for o := 0; o < len(gotUserLogsErr.UserLogs); o++ {
			for p := 0; p < len(tempCostsErr.Costs); p++ {
				if gotUserLogsErr.UserLogs[o].CostCurrency == tempCostsErr.Costs[p].CostCurrency &&
					gotUserLogsErr.UserLogs[o].DrugName == tempCostsErr.Costs[p].Substance {

					tempCostsErr.Costs[p].TotalCost += gotUserLogsErr.UserLogs[o].Cost
				}
			}
		}
	}

	if costsErrChan != nil {
		costsErrChan <- tempCostsErr
	}
	return tempCostsErr
}

// PrintTotalCosts writes all costs for all currencies to console.
//
// costs - the costs slice returned from GetTotalCosts()
//
// prefix - if true the name of the function should be shown
// when writing to console
func PrintTotalCosts(costs []Cost, prefix bool) {
	var printN string
	if prefix == true {
		printN = "PrintTotalCosts()"
	} else {
		printN = ""
	}
	noCosts := true

	for i := 0; i < len(costs); i++ {
		if costs[i].TotalCost == 0 {
			continue
		}
		noCosts = false
		printNameF(printN, "Substance:\t%q\n", costs[i].Substance)
		printNameF(printN, "Total Cost:\t%g\n", costs[i].TotalCost)
		printNameF(printN, "Cost Currency:\t%q\n", costs[i].CostCurrency)
		printName(printN, "====================")
	}
	if noCosts == true {
		printNameF(printN, "No logged costs.\n")
	}
}

var TotalCostsEmptyError error = errors.New("there are no costs to return")
