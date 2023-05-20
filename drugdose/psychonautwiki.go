package drugdose

import (
	"context"

	"github.com/hasura/go-graphql-client"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"
)

type PsychonautwikiSubstance []struct {
	Name string

	Roas []struct {
		Name string

		Dose struct {
			Units     string
			Threshold float64
			Light     struct {
				Min float64
				Max float64
			}
			Common struct {
				Min float64
				Max float64
			}
			Strong struct {
				Min float64
				Max float64
			}
		}

		Duration struct {
			Onset struct {
				Min   float64
				Max   float64
				Units string
			}

			Comeup struct {
				Min   float64
				Max   float64
				Units string
			}

			Peak struct {
				Min   float64
				Max   float64
				Units string
			}

			Offset struct {
				Min   float64
				Max   float64
				Units string
			}

			Total struct {
				Min   float64
				Max   float64
				Units string
			}
		}
	}
}

// Used to initialise the GraphQL struct, using the source address from
// the drugdose Config struct.
//
// returns: bool (true if client is initialised, false otherwise),
// client (the GraphQL struct used with github.com/hasura/go-graphql-client
func (cfg Config) InitGraphqlClient() (bool, graphql.Client) {
	const printN string = "InitGraphqlClient()"

	client := graphql.Client{}

	if !cfg.AutoFetch {
		printName(printN, "Automatic fetching is disabled, returning.")
		return false, client
	}

	gotsrcData := GetSourceData()

	api := gotsrcData[cfg.UseSource].API_ADDRESS

	client_new := graphql.NewClient("https://"+api, nil)
	return true, *client_new
}

func (cfg Config) FetchPsyWiki(db *sql.DB, ctx context.Context,
	drugname string, client graphql.Client) bool {
	const printN string = "FetchPsyWiki()"

	if !cfg.AutoFetch {
		printName(printN, "Automatic fetching is disabled, returning.")
		return false
	}

	drugname = cfg.MatchAndReplace(db, ctx, drugname, "substance")

	ret := checkIfExistsDB(db, ctx,
		"drugName",
		"psychonautwiki",
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drugname)
	if ret {
		printNameVerbose(cfg.VerbosePrinting, printN, "Drug already in DB, returning. No need to fetch anything from Psychonautwiki.")
		return false
	}

	printName(printN, "Fetching from source:", cfg.UseSource)

	// This is the graphql query for Psychonautwiki.
	// The way it works is, the full query is generated
	// using the PsychonautwikiSubstance struct.
	var query struct {
		PsychonautwikiSubstance `graphql:"substances(query: $dn)"`
	}

	// Since the query has to be a string, the module has provided
	// an argument allowing to map a variable to the string.
	variables := map[string]interface{}{
		"dn": drugname,
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		printName(printN, "Error from Psychonautwiki API:", err)
		return false
	}

	InfoDrug := []DrugInfo{}

	if len(query.PsychonautwikiSubstance) != 0 {
		subs := query.PsychonautwikiSubstance
		for i := 0; i < len(subs); i++ {
			if len(subs[i].Roas) != 0 {
				for o := 0; o < len(subs[i].Roas); o++ {
					printNameVerbose(cfg.VerbosePrinting, printN, "From source:", cfg.UseSource, "; Substance:", subs[i].Name,
						"; Route:", subs[i].Roas[o])

					tempInfoDrug := DrugInfo{}

					tempInfoDrug.DrugName = subs[i].Name
					tempInfoDrug.DrugRoute = subs[i].Roas[o].Name
					tempInfoDrug.Threshold = float32(subs[i].Roas[o].Dose.Threshold)
					tempInfoDrug.LowDoseMin = float32(subs[i].Roas[o].Dose.Light.Min)
					tempInfoDrug.LowDoseMax = float32(subs[i].Roas[o].Dose.Light.Max)
					tempInfoDrug.MediumDoseMin = float32(subs[i].Roas[o].Dose.Common.Min)
					tempInfoDrug.MediumDoseMax = float32(subs[i].Roas[o].Dose.Common.Max)
					tempInfoDrug.HighDoseMin = float32(subs[i].Roas[o].Dose.Strong.Min)
					tempInfoDrug.HighDoseMax = float32(subs[i].Roas[o].Dose.Strong.Max)
					tempInfoDrug.DoseUnits = subs[i].Roas[o].Dose.Units
					tempInfoDrug.OnsetMin = float32(subs[i].Roas[o].Duration.Onset.Min)
					tempInfoDrug.OnsetMax = float32(subs[i].Roas[o].Duration.Onset.Max)
					tempInfoDrug.OnsetUnits = subs[i].Roas[o].Duration.Onset.Units
					tempInfoDrug.ComeUpMin = float32(subs[i].Roas[o].Duration.Comeup.Min)
					tempInfoDrug.ComeUpMax = float32(subs[i].Roas[o].Duration.Comeup.Max)
					tempInfoDrug.ComeUpUnits = subs[i].Roas[o].Duration.Comeup.Units
					tempInfoDrug.PeakMin = float32(subs[i].Roas[o].Duration.Peak.Min)
					tempInfoDrug.PeakMax = float32(subs[i].Roas[o].Duration.Peak.Max)
					tempInfoDrug.PeakUnits = subs[i].Roas[o].Duration.Peak.Units
					tempInfoDrug.OffsetMin = float32(subs[i].Roas[o].Duration.Offset.Min)
					tempInfoDrug.OffsetMax = float32(subs[i].Roas[o].Duration.Offset.Max)
					tempInfoDrug.OffsetUnits = subs[i].Roas[o].Duration.Offset.Units
					tempInfoDrug.TotalDurMin = float32(subs[i].Roas[o].Duration.Total.Min)
					tempInfoDrug.TotalDurMax = float32(subs[i].Roas[o].Duration.Total.Max)
					tempInfoDrug.TotalDurUnits = subs[i].Roas[o].Duration.Total.Units

					InfoDrug = append(InfoDrug, tempInfoDrug)
				}
			} else {
				printName(printN, "No roas for:", subs[i])
			}
		}

		if len(InfoDrug) != 0 {
			ret := cfg.AddToInfoDB(db, ctx, InfoDrug)
			if !ret {
				printName(printN, "Data couldn't be added to info DB, because of an error.")
				return false
			}
			printName(printN, "Data added to info DB successfully.")
		} else {
			printName(printN, "Struct array is empty, nothing added to DB.")
			return false
		}
	} else {
		printName(printN, "The Psychonautwiki API returned nothing, so query is wrong or connection is broken.")
		return false
	}

	return true
}
