package drugdose

import (
	"context"
	"fmt"

	"database/sql"
	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"
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
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// costsErrChan - the goroutine channel used to return the slice containing
// all costs
//
// username - the user for which to return the costs
func (cfg Config) GetTotalCosts(db *sql.DB, ctx context.Context,
	costsErrChan chan<- CostsError, username string) {

	const printN string = "GetTotalCosts()"

	tempCostsErr := CostsError{
		Costs: nil,
		Err:   nil,
	}

	drugNamesErrChan := make(chan DrugNamesError)
	go cfg.GetLoggedNames(db, ctx, drugNamesErrChan, false, username, LogDrugNameCol)
	gotDrugNamesErr := <-drugNamesErrChan
	err := gotDrugNamesErr.Err
	uniqueDrugNames := gotDrugNamesErr.DrugNames
	if err != nil {
		tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		costsErrChan <- tempCostsErr
		return
	}

	go cfg.GetLoggedNames(db, ctx, drugNamesErrChan, false, username, LogCostCurrencyCol)
	gotDrugNamesErr = <-drugNamesErrChan
	err = gotDrugNamesErr.Err
	uniqueCurrencyNames := gotDrugNamesErr.DrugNames
	if err != nil {
		tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		costsErrChan <- tempCostsErr
		return
	}

	userLogsErrorChannel := make(chan UserLogsError)
	for i := 0; i < len(uniqueDrugNames); i++ {
		go cfg.GetLogs(db, ctx, userLogsErrorChannel, 0, 0, username,
			true, uniqueDrugNames[i], LogDrugNameCol)
		gotUserLogsChannel := <-userLogsErrorChannel
		err := gotUserLogsChannel.Err
		if err != nil {
			tempCostsErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
			costsErrChan <- tempCostsErr
			return
		}

		for o := 0; o < len(uniqueCurrencyNames); o++ {
			tempCost := Cost{
				Substance:    uniqueDrugNames[i],
				TotalCost:    0,
				CostCurrency: uniqueCurrencyNames[o],
			}
			tempCostsErr.Costs = append(tempCostsErr.Costs, tempCost)
		}

		for o := 0; o < len(gotUserLogsChannel.UserLogs); o++ {
			for p := 0; p < len(tempCostsErr.Costs); p++ {
				if gotUserLogsChannel.UserLogs[o].CostCurrency == tempCostsErr.Costs[p].CostCurrency &&
					gotUserLogsChannel.UserLogs[o].DrugName == tempCostsErr.Costs[p].Substance {
					tempCostsErr.Costs[p].TotalCost += gotUserLogsChannel.UserLogs[o].Cost
				}
			}
		}
	}
	costsErrChan <- tempCostsErr
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

	for i := 0; i < len(costs); i++ {
		if costs[i].TotalCost == 0 {
			continue
		}
		printNameF(printN, "Substance:\t%q\n", costs[i].Substance)
		printNameF(printN, "Total Cost:\t%g\n", costs[i].TotalCost)
		printNameF(printN, "Cost Currency:\t%q\n", costs[i].CostCurrency)
		printName(printN, "====================")
	}
}
