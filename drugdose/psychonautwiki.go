package drugdose

import (
	"context"
	"fmt"

	"github.com/hasura/go-graphql-client"
)

type Substances []struct {
	Name graphql.String

	Roas []struct {
		Name graphql.String

		Dose struct {
			Units     graphql.String
			Threshold graphql.Float
			Light     struct {
				Min graphql.Float
				Max graphql.Float
			}
			Common struct {
				Min graphql.Float
				Max graphql.Float
			}
			Strong struct {
				Min graphql.Float
				Max graphql.Float
			}
		}

		Duration struct {
			Onset struct {
				Min   graphql.Float
				Max   graphql.Float
				Units graphql.String
			}

			Comeup struct {
				Min   graphql.Float
				Max   graphql.Float
				Units graphql.String
			}

			Peak struct {
				Min   graphql.Float
				Max   graphql.Float
				Units graphql.String
			}

			Offset struct {
				Min   graphql.Float
				Max   graphql.Float
				Units graphql.String
			}

			Total struct {
				Min   graphql.Float
				Max   graphql.Float
				Units graphql.String
			}
		}
	}
}

func (cfg *Config) InitGraphqlClient(api string) *graphql.Client {
	if api == "default" {
		api = defaultAPI
	}

	if !cfg.AutoFetch {
		fmt.Println("Automatic fetching is disabled, returning.")
		return nil
	}

	client := graphql.NewClient("https://"+api, nil)
	return client
}

func (cfg *Config) FetchPsyWiki(drugname string, drugroute string, client *graphql.Client, driver string, path string) bool {
	if !cfg.AutoFetch {
		fmt.Println("Automatic fetching is disabled, returning.")
		return false
	}

	matchedDrugName := MatchDrugName(drugname)
	if matchedDrugName != "" {
		drugname = matchedDrugName
	}

	ret := checkIfExistsDB("drugName", "psychonautwiki", driver, path, nil, drugname)
	if ret {
		fmt.Println("Drug already in DB, returning.")
		return false
	}

	fmt.Println("Fetching from API:", cfg.UseAPI)

	var q struct {
		Substances `graphql:"substances(query: $dn)"`
	}

	variables := map[string]interface{}{
		"dn": graphql.String(drugname),
	}

	err := client.Query(context.Background(), &q, variables)
	if err != nil {
		fmt.Println("Error from Psychonautwiki API:", err)
		return false
	}

	InfoDrug := []DrugInfo{}

	if len(q.Substances) != 0 {
		subs := q.Substances

		for i := 0; i < len(subs); i++ {
			if len(subs[i].Roas) != 0 {
				for o := 0; o < len(subs[i].Roas); o++ {
					fmt.Println(subs[i].Roas[o])

					tempInfoDrug := DrugInfo{}

					tempInfoDrug.DrugName = string(subs[i].Name)
					tempInfoDrug.DrugRoute = string(subs[i].Roas[o].Name)
					tempInfoDrug.Threshold = float32(subs[i].Roas[o].Dose.Threshold)
					tempInfoDrug.LowDoseMin = float32(subs[i].Roas[o].Dose.Light.Min)
					tempInfoDrug.LowDoseMax = float32(subs[i].Roas[o].Dose.Light.Max)
					tempInfoDrug.MediumDoseMin = float32(subs[i].Roas[o].Dose.Common.Min)
					tempInfoDrug.MediumDoseMax = float32(subs[i].Roas[o].Dose.Common.Max)
					tempInfoDrug.HighDoseMin = float32(subs[i].Roas[o].Dose.Strong.Min)
					tempInfoDrug.HighDoseMax = float32(subs[i].Roas[o].Dose.Strong.Max)
					tempInfoDrug.DoseUnits = string(subs[i].Roas[o].Dose.Units)
					tempInfoDrug.OnsetMin = float32(subs[i].Roas[o].Duration.Onset.Min)
					tempInfoDrug.OnsetMax = float32(subs[i].Roas[o].Duration.Onset.Max)
					tempInfoDrug.OnsetUnits = string(subs[i].Roas[o].Duration.Onset.Units)
					tempInfoDrug.ComeUpMin = float32(subs[i].Roas[o].Duration.Comeup.Min)
					tempInfoDrug.ComeUpMax = float32(subs[i].Roas[o].Duration.Comeup.Max)
					tempInfoDrug.ComeUpUnits = string(subs[i].Roas[o].Duration.Comeup.Units)
					tempInfoDrug.PeakMin = float32(subs[i].Roas[o].Duration.Peak.Min)
					tempInfoDrug.PeakMax = float32(subs[i].Roas[o].Duration.Peak.Max)
					tempInfoDrug.PeakUnits = string(subs[i].Roas[o].Duration.Peak.Units)
					tempInfoDrug.OffsetMin = float32(subs[i].Roas[o].Duration.Offset.Min)
					tempInfoDrug.OffsetMax = float32(subs[i].Roas[o].Duration.Offset.Max)
					tempInfoDrug.OffsetUnits = string(subs[i].Roas[o].Duration.Offset.Units)
					tempInfoDrug.TotalDurMin = float32(subs[i].Roas[o].Duration.Total.Min)
					tempInfoDrug.TotalDurMax = float32(subs[i].Roas[o].Duration.Total.Max)
					tempInfoDrug.TotalDurUnits = string(subs[i].Roas[o].Duration.Total.Units)

					InfoDrug = append(InfoDrug, tempInfoDrug)
				}
			} else {
				fmt.Println("FetchPsyWiki: No roas for:", subs[i])
			}
		}

		if len(InfoDrug) != 0 {
			ret := AddToInfoDB("psychonautwiki", InfoDrug, driver, path)
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
