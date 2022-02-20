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
		api = default_api
	}

	if cfg.AutoFetch == false {
		fmt.Println("Automatic fetching is disabled, returning.")
		return nil
	}

	client := graphql.NewClient("https://"+api, nil)
	return client
}

func (cfg *Config) FetchPsyWiki(drugname string, drugroute string, client *graphql.Client, path string) bool {
	if cfg.AutoFetch == false {
		fmt.Println("Automatic fetching is disabled, returning.")
		return false
	}

	matchedDrugName := MatchDrugName(drugname)
	if matchedDrugName != "" {
		drugname = matchedDrugName
	}

	ret := checkIfExistsDB("drugName", "psychonautwiki", path, nil, drugname)
	if ret == true {
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

	if len(q.Substances) != 0 {
		ret := AddToInfoDB("psychonautwiki", q.Substances, path)
		if ret == false {
			fmt.Println("Data couldn't be added to info DB, because of an error.")
			return false
		}
		fmt.Println("Data added to info DB successfully.")
	} else {
		fmt.Println("The Psychonautwiki API returned nothing, so query is wrong or connection is broken.")
		return false
	}

	return true
}
