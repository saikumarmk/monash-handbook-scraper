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
	const baseURL = "https://api-ap-southeast-2.prod.courseloop.com/publisher/search-all"
	const pageSize = 100

	data := make(map[string]interface{})
	var results []interface{}
	start := 0
	session := &http.Client{}

	for {
		url := fmt.Sprintf("%s?from=%d&query=&searchType=advanced&siteId=monash-prod-pres&siteYear=current&size=%d", baseURL, start, pageSize)
		response, err := session.Get(url)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		var pageData map[string]interface{}
		decoder := json.NewDecoder(response.Body)
		if err := decoder.Decode(&pageData); err != nil {
			return nil, err
		}

		// Extract "items" (or the relevant key where results are stored)
		if items, exists := pageData["data"].(map[string]interface{})["results"].([]interface{}); exists {
			results = append(results, items...)
		}

		// Check total count
		total, ok := pageData["data"].(map[string]interface{})["total"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'total' field")
		}

		start += pageSize
		if start >= int(total) {
			break
		}
	}

	// Store all results in the data map
	data["results"] = results
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
	results, ok := monashData["results"].([]interface{})
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

// https://handbook.monash.edu/_next/data/x72Bg6G_Gp9JqA01tHcsD/2024/units/FIT3175.json?year=2024&catchAll=2024&catchAll=units&catchAll=FIT3175
// /_next/data/x72Bg6G_Gp9JqA01tHcsD/2025/units/FIT3175.json

// https://handbook.monash.edu/_next/data/x72Bg6G_Gp9JqA01tHcsD/current/units/FIT3175.json?year=current&catchAll=current&catchAll=units&catchAll=FIT3175

// getContent retrieves an item from a specific category and sends the JSON response to channels.
// If the item fails to be scraped (Rate Limited typically), it will be added to a failure channel.
func getContent(item string, category string, results chan map[string]interface{}, failures chan string, rate_limited chan string) {
	response, err := http.Get("https://handbook.monash.edu/_next/data/x72Bg6G_Gp9JqA01tHcsD/current/" + category + "/" + item + ".json?year=current&catchAll=current&catchAll=" + category + "&catchAll=" + item)

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

// loadFailedItems reads a list of failed units (due to rate limiting) from a JSON file.
func loadFailedItems(itemType string) ([]string, error) {
	data, err := os.ReadFile("data/" + itemType + "_failed.json")
	if err != nil {
		return nil, err
	}

	var itemsFailed []string
	if err := json.Unmarshal(data, &itemsFailed); err != nil {
		return nil, err
	}

	return itemsFailed, nil
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

		// Check if the file is empty
		fileInfo, err := successFile.Stat()
		if err != nil {
			fmt.Printf("Error getting file info: %v\n", err)
			return
		}

		if fileInfo.Size() > 0 {
			decoder := json.NewDecoder(successFile)
			if err := decoder.Decode(&existingData); err != nil {
				fmt.Printf("Error decoding JSON content (parallel Scrape): %v\n", err)
				return
			}
		} else {
			// If file is empty, initialize existingData to a default value
			existingData = make([]map[string]interface{}, 0) // Change type based on expected structure
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
	fmt.Printf("Scraping from handbook: %s\n", category)
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
				unitsFailed, _ := loadFailedItems(category)
				parallelScrapeContent(unitsFailed, "units", 10)

				fmt.Println("Starting the 7-minute pause...")
				time.Sleep(duration)
				fmt.Println("Pause complete. Resuming the program.")
			}

		case "aos":
			parallelScrapeContent(contentSplits["aos"], "aos", numWorkers)
			itemsFailed, _ := loadFailedItems(category)
			fmt.Println("Starting the 7-minute pause...")
			time.Sleep(duration)
			parallelScrapeContent(itemsFailed, "aos", numWorkers)

		case "courses":

			parallelScrapeContent(contentSplits["courses"], "courses", numWorkers)
			coursesFailed, _ := loadFailedItems(category)
			fmt.Println("Starting the 7-minute pause...")
			time.Sleep(duration)
			parallelScrapeContent(coursesFailed, "courses", numWorkers)

		default:
			fmt.Println("Invalid category")
	}

}
