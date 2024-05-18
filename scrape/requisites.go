package scrape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// Processes the requisites
func RequisiteScrape(unitCodeList [][]string, fileName string) {
	numWorkers := 10

	var wg sync.WaitGroup
	results := make(chan map[string]interface{}, len(unitCodeList))

	for idx := 0; idx < numWorkers; idx++ {
		start := (idx * len(unitCodeList)) / numWorkers
		end := ((idx + 1) * len(unitCodeList)) / numWorkers

		wg.Add(1)
		go func(unitCodesSlice [][]string) {
			defer wg.Done()
			for _, unitCode := range unitCodesSlice {
				response, err := postRequest(unitCode)
				if err != nil {
					fmt.Printf("Error for unit %s: %v\n", unitCode, err)
				} else {
					results <- response
				}
			}
		}(unitCodeList[start:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var responses []map[string]interface{}

	for response := range results {
		responses = append(responses, response)
	}

	// refactor to save separately, have filter on top, have this return the responses instead
	saveResponsesToJSON(responses, fileName)
}

func postRequest(unitCodes []string) (map[string]interface{}, error) {
	url := "https://mscv.apps.monash.edu"
	payload := createRequestPayload(unitCodes)

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func createRequestPayload(unitCodes []string) map[string]interface{} {
	units := make([]map[string]interface{}, len(unitCodes))

	for i, unitCode := range unitCodes {
		unit := map[string]interface{}{
			"unitCode":    unitCode,
			"placeholder": false,
		}
		units[i] = unit
	}

	payload := map[string]interface{}{
		"startYear":            2024,
		"advancedStanding":     []interface{}{},
		"internationalStudent": false,
		"courseInfo":           map[string]interface{}{},
		"teachingPeriods": []map[string]interface{}{
			{
				"year":         2024,
				"code":         "S1-01",
				"units":        units,
				"intermission": false,
				"studyAbroad":  false,
			},
		},
	}

	return payload
}

func saveResponsesToJSON(responses []map[string]interface{}, fileName string) {
	file, err := os.Create("data/raw_" + fileName + ".json")
	if err != nil {
		fmt.Printf("Error creating JSON file: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(responses); err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
	}
}
