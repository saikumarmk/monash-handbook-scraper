// Fetches and scrapes a list of items from the Monash Handbook
// site.

package scrape

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// fetchMonashIndex fetches the Monash Index of Units, AOS, and Courses for the current year.
func fetchMonashIndex() (map[string]interface{}, error) {
	/*
		Fetches Monash Index of Units, AOS, and Courses for current year.
	*/
	response, err := http.Get("https://api-ap-southeast-2.prod.courseloop.com/publisher/search-all?from=0&query=&searchType=advanced&siteId=monash-prod-pres&siteYear=current&size=7000")

	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var data map[string]interface{}

	decoder := json.NewDecoder((response.Body))
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// processMonashIndex processes the current Monash index into a map of lists.
func processMonashIndex() map[string][]string {

	monashData, err := fetchMonashIndex()
	content_splits := make(map[string][]string)

	if err != nil {
		fmt.Printf("Error fetching Monash data: %v\n", err)
		return content_splits
	}
	results, ok := monashData["data"].(map[string]interface{})["results"].([]interface{})
	if !ok {
		fmt.Println("Error extracting content list")
		return content_splits
	}

	for _, result := range results {
		content, ok := result.(map[string]interface{})

		if !ok {
			fmt.Println(("Error"))
			continue
		}

		uri, _ := content["uri"].(string)
		parts := strings.Split(uri, "/")
		content_type := parts[2]
		code, ok := content["code"].(string)

		if !ok {
			continue
		}
		content_splits[content_type] = append(content_splits[content_type], code)
	}

	return content_splits

}

// loadContentSplits reads the content_splits from a JSON file.
func loadContentSplits() (map[string][]string, error) {
	data, err := os.ReadFile("data/content_splits.json")

	if err != nil {
		return nil, err
	}

	var content_splits map[string][]string
	if err := json.Unmarshal(data, &content_splits); err != nil {
		return nil, err
	}
	return content_splits, nil

}

// saveContentSplits saves the content_splits to a JSON file.
func saveContentSplits(content_splits map[string][]string) error {
	data, err := json.Marshal(content_splits)

	if err != nil {
		return err
	}

	if err := os.WriteFile("data/content_splits.json", data, 0644); err != nil {
		return err
	}
	return nil
}

// getContent retrieves an item from a specific category and sends the JSON response to channels.
// If the item fails to be scraped (Rate Limited typically), it will be added to a failure channel.
func getContent(item string, category string, results chan map[string]interface{}, failures chan string, rate_limited chan string) {
	response, err := http.Get("https://handbook.monash.edu/_next/data/1F6sQtV9SmVrQtVZjV3Zh/current/" + category + "/" + item + ".json?year=current")

	if err != nil {
		results <- nil
		failures <- item
		return
	}
	defer response.Body.Close()

	var data map[string]interface{}
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&data); err != nil {
		results <- nil
		failures <- item
		return
	}

	if _, ok := data["message"]; ok {
		results <- nil
		rate_limited <- item
		return
	}

	results <- data
}

// loadUnitsFailed reads a list of failed units (due to rate limiting) from a JSON file.
func loadUnitsFailed() ([]string, error) {
	data, err := os.ReadFile("data/units_failed.json")
	if err != nil {
		return nil, err
	}

	var unitsFailed []string
	if err := json.Unmarshal(data, &unitsFailed); err != nil {
		return nil, err
	}

	return unitsFailed, nil
}

// parallelScrapeContent performs parallel scraping of items in a category,
// stores results in JSON, and handles rate-limited items.
func parallelScrapeContent(items []string, category string, numWorkers int) {

	var wg sync.WaitGroup
	var existingData []map[string]interface{}
	var failedList []string
	results := make(chan map[string]interface{}, len(items))
	failures := make(chan string, len(items))
	rateLimited := make(chan string, len(items))

	itemsPerWorker := len(items) / numWorkers
	for idx := 0; idx < numWorkers; idx++ {
		start := idx * itemsPerWorker
		end := min((idx+1)*itemsPerWorker, len(items))

		if idx == numWorkers-1 {
			end = len(items)
		}

		wg.Add(1)

		go func(itemsSlice []string) {
			defer wg.Done()
			for _, item := range itemsSlice {
				getContent(item, category, results, failures, rateLimited)
			}
		}(items[start:end])
	}

	go func() {
		wg.Wait()
		close(results)
		close(failures)
		close(rateLimited)

	}()
	// Attempt to write to JSON, create if it doesn't exist
	var outputFileName = "data/raw_" + category + ".json"
	var successFile *os.File

	_, err := os.Stat(outputFileName)
	if os.IsNotExist(err) {
		successFile, err = os.Create(outputFileName)

		if err != nil {
			fmt.Println("Error creating file")
			return
		}
	} else {
		successFile, err = os.OpenFile(outputFileName, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			fmt.Println("Error opening file")
			return
		}
		decoder := json.NewDecoder(successFile)
		if err := decoder.Decode(&existingData); err != nil {
			fmt.Printf("Error decoding JSON content: %v\n", err)
			return
		}
		if _, err := successFile.Seek(0, 0); err != nil {
			fmt.Printf("Error seeking the file: %v\n", err)
			return
		}
		if err := successFile.Truncate(0); err != nil {
			fmt.Printf("Error truncating the file: %v\n", err)
			return
		}

	}

	defer successFile.Close()

	// Merge results
	for data := range results {
		if data != nil {
			if data["pageProps"] != nil {
				if data["pageProps"].(map[string]interface{})["pageContent"] != nil {
					existingData = append(existingData, data["pageProps"].(map[string]interface{})["pageContent"].(map[string]interface{}))
				}
			}
		}
	}
	fmt.Printf("Done %d\n", len(existingData))

	fmt.Println("Writing Data to JSON")
	encoder := json.NewEncoder(successFile)
	encoder.Encode(existingData)
	fmt.Println("Data Written!")

	// Store units that got rate limited for another attempt
	for fail := range rateLimited {
		failedList = append(failedList, fail)
	}

	failed_file, err := os.Create("data/" + category + "_failed.json")
	if err != nil {
		fmt.Println("Error Writing to File")
		return
	}
	defer failed_file.Close()

	for failed := range rateLimited {
		failedList = append(failedList, failed)
	}
	fail_encoder := json.NewEncoder(failed_file)
	fail_encoder.Encode(failedList)

}

// Creates or Loads an existing Handbook index
func InitialiseContentSplits() map[string][]string {
	contentSplits, err := loadContentSplits()

	if err != nil {
		fmt.Println("Getting new index...")
		contentSplits = processMonashIndex()

		if err := saveContentSplits(contentSplits); err != nil {
			fmt.Println("Failed to save content split")
			return nil
		}
		fmt.Println("Got new index!")
	}
	return contentSplits
}

// Scrapes the Handbook for a category of items, producing relevant json files for them.
// Can continue an existing scrape (for units) or start from scratch.
func HandbookScrape(category string, fresh bool) {

	// Get/Create index split
	contentSplits := InitialiseContentSplits()
	duration := 7 * time.Minute
	numWorkers := 10

	if fresh {
		// Remove the existing JSON file if a fresh scrape is requested.
		if err := os.Remove(category + ".json"); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Error removing existing JSON file: %v\n", err)
			return
		}
	}

	switch category {
	case "units":
		// Scrape units and retry on rate limiting.
		parallelScrapeContent(contentSplits["units"], "units", numWorkers)
		fmt.Println("Starting the 7-minute pause...")
		time.Sleep(duration)
		for idx := 2; idx <= 5; idx++ {
			fmt.Printf("Scraping attempt %d...\n", idx)
			unitsFailed, _ := loadUnitsFailed()
			parallelScrapeContent(unitsFailed, "units", 10)

			fmt.Println("Starting the 7-minute pause...")
			time.Sleep(duration)
			fmt.Println("Pause complete. Resuming the program.")
		}

	case "aos":
		parallelScrapeContent(contentSplits["aos"], "aos", numWorkers)

	case "courses":
		parallelScrapeContent(contentSplits["courses"], "courses", numWorkers)

	default:
		fmt.Println("Invalid category")
	}

}
