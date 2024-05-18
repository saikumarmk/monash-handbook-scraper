package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"handbook-scraper/format"
	"handbook-scraper/process"
	"handbook-scraper/scrape"
	"os"
)

func main() {
	// Use functions and logic from the scrape and requisites packages

	// unify into scrape [aos, courses, units, requisites] format [units courses]
	actionFlag := flag.String("choice", "scrape", "scrape, format, process")
	contentFlag := flag.String("content", "courses", "aos, courses, units, requisites")
	flag.Parse()

	if _, err := os.Stat("data"); os.IsNotExist(err) {
		os.Mkdir("data", os.ModeDir)
	}
	contentSplits := scrape.InitialiseContentSplits()

	switch *actionFlag {
	case "scrape":
		switch *contentFlag {

		// Run this only after running the unit formatter
		case "requisites":
			var unitItems [][]string

			for _, item := range contentSplits["units"] {
				unitItems = append(unitItems, []string{item})
			}

			scrape.RequisiteScrape(unitItems, "prerequisites")

			file, err := os.Open("data/prohibition_candidates.json")
			if err != nil {
				fmt.Println("Error opening file:", err)
				return
			}
			defer file.Close()

			var prohibitionCandidates [][]string

			decoder := json.NewDecoder(file)
			if err := decoder.Decode(&prohibitionCandidates); err != nil {
				fmt.Println("Error decoding JSON:", err)
				return
			}
			scrape.RequisiteScrape(prohibitionCandidates, "prohibitions")

		default:
			scrape.HandbookScrape(*contentFlag, true) // Make separate function to save instead of doing it at once
		}

	// Single responsibility into file -> saves to file in there
	case "format":
		file, err := os.Open("data/raw_" + *contentFlag + ".json")
		if err != nil {
			fmt.Println("Could not find " + "*contentFlag" + ".json")
			return
		}
		defer file.Close()
		var raw_data []map[string]interface{}
		var formatted_data map[string]interface{}
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&raw_data); err != nil {
			fmt.Println("Error decoding JSON:", err)
			return
		}

		switch *contentFlag {
		case "units":
			formatted_data = format.FormatUnits(raw_data)
		default:
			fmt.Println("How did you even get here??")
			return
		}

		data, err := json.Marshal(formatted_data)

		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
		}

		if err := os.WriteFile("data/formatted_units.json", data, 0644); err != nil {
			fmt.Println("Failed to write to file:", err)
		}
		fmt.Println("Succesfully formatted " + *contentFlag)
	case "process":
		processed := process.ProcessHandbook()

		data, err := json.Marshal(processed)

		if err != nil {
			fmt.Println("Error marshalling JSON:", err)
		}

		if err := os.WriteFile("data/processed_units.json", data, 0644); err != nil {
			fmt.Println("Failed to write to file:", err)
		}
		fmt.Println("Succesfully processed " + *contentFlag)

	default:
		fmt.Println("Not Recognised")
	}

}
