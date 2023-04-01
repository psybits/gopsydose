package drugdose

import (
	"context"
	"fmt"

	"github.com/hasura/go-graphql-client"
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

func (cfg *Config) InitGraphqlClient() *graphql.Client {
	if !cfg.AutoFetch {
		fmt.Println("Automatic fetching is disabled, returning.")
		return nil
	}

	gotsrcData := GetSourceData()

	api := gotsrcData[cfg.UseSource].API_URL

	client := graphql.NewClient("https://"+api, nil)
	return client
}

func (cfg *Config) FetchPsyWiki(drugname string, client *graphql.Client) bool {
	if !cfg.AutoFetch {
		fmt.Println("Automatic fetching is disabled, returning.")
		return false
	}

	drugname = cfg.MatchAndReplace(drugname, "substance")

	ret := checkIfExistsDB("drugName",
		"psychonautwiki",
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drugname)
	if ret {
		fmt.Println("Drug already in DB, returning.")
		return false
	}

	fmt.Println("Fetching from source:", cfg.UseSource)

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
		fmt.Println("Error from Psychonautwiki API:", err)
		return false
	}

	InfoDrug := []DrugInfo{}

	if len(query.PsychonautwikiSubstance) != 0 {
		subs := query.PsychonautwikiSubstance
		for i := 0; i < len(subs); i++ {
			if len(subs[i].Roas) != 0 {
				for o := 0; o < len(subs[i].Roas); o++ {
					fmt.Println("From source:", cfg.UseSource, "; Substance:", subs[i].Name,
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
				fmt.Println("FetchPsyWiki: No roas for:", subs[i])
			}
		}

		if len(InfoDrug) != 0 {
			ret := cfg.AddToInfoDB(InfoDrug)
			if !ret {
				fmt.Println("Data couldn't be added to info DB, because of an error.")
				return false
			}
			fmt.Println("Data added to info DB successfully.")
		} else {
			fmt.Println("Struct array is empty, nothing added to DB.")
			return false
		}
	} else {
		fmt.Println("The Psychonautwiki API returned nothing, so query is wrong or connection is broken.")
		return false
	}

	return true
}
