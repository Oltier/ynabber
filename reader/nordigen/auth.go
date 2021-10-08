package nordigen

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib"
)

const redirectPort = ":3000"

func requisitionFileLocation(endUserId string) string {
	return fmt.Sprintf("%s.json", endUserId)
}

// AuthorizationWrapper tries to get requisition from disk, if it fails it will
// create a new and store that one to disk.
func AuthorizationWrapper(cli nordigen.Client, bankId string, endUserId string) (nordigen.Requisition, error) {
	store := requisitionFileLocation(endUserId)
	requisitionFile, err := os.ReadFile(store)
    if err != nil {
		log.Print("No existing requisition found, creating a new...")
        requisition, err := GetAuthorization(cli, bankId, endUserId)
		if err != nil {
			return nordigen.Requisition{}, err
		}
		err = StoreAuthorization(requisition, endUserId)
		if err != nil {
			log.Printf("Failed to write requisition to disk: %s", err)
		}
		log.Printf("Requisition stored for reuse: %s", store)
		return requisition, nil
    }

	var requisition nordigen.Requisition
	err = json.Unmarshal(requisitionFile, &requisition)
	if err != nil {
		return nordigen.Requisition{}, err
	}
	log.Print("Found existing requisition, using it...")
	return requisition, nil
}

func StoreAuthorization(requisition nordigen.Requisition, endUserId string) error {
	store := requisitionFileLocation(endUserId)
	requisitionFile, err := json.Marshal(requisition)
	if err != nil {
		return err
	}

	err = os.WriteFile(store, requisitionFile, 0644)
	if err != nil {
		return err
	}
	return nil
}

func GetAuthorization(cli nordigen.Client, bankId string, endUserId string) (nordigen.Requisition, error) {
	requisition := nordigen.Requisition{
		Redirect:  "http://localhost" + redirectPort,
		Reference: strconv.Itoa(int(time.Now().Unix())),
		EnduserId: endUserId,
		Agreements: []string{

		},
	}
	r, err := cli.CreateRequisition(requisition)

	if err != nil {
		return nordigen.Requisition{}, err
	}
	rr, err := cli.CreateRequisitionLink(r.Id, nordigen.RequisitionLinkRequest{
		AspspsId: bankId})

	if err != nil {
		return nordigen.Requisition{}, err
	}

	log.Printf("Initiate requisition by going to: %s", rr.Initiate)

	for r.Status == "CR" {
		r, err = cli.GetRequisition(r.Id)

		if err != nil {

			return nordigen.Requisition{}, err
		}
		time.Sleep(1 * time.Second)
	}

	return r, nil
}
