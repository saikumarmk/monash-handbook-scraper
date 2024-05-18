// This package combines the MonPlan requisite data and the Handbook data.

package process

import (
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	unitPattern = regexp.MustCompile(`[A-Z]{3}[0-9]{4}`)
	numPattern  = regexp.MustCompile(`[0-9]{1,3}`)
)

type References struct {
	TeachingPeriodCode         string `json:"teachingPeriodCode"`
	TeachingPeriodStartingYear int    `json:"teachingPeriodStartingYear"`
	UnitCode                   string `json:"unitCode"`
}

type EnrolmentError struct {
	Description string       `json:"description"`
	Level       string       `json:"level"`
	References  []References `json:"references"`
	Title       string       `json:"title"`
	Type        string       `json:"type"`
}

type Rule struct {
	CourseErrors []EnrolmentError `json:"courseErrors"`
}

type RawRequisite struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type RefinedRequisite struct {
	Permission    bool                     `json:"permission"`
	Prohibitions  []string                 `json:"prohibitions"`
	Corequisites  []map[string]interface{} `json:"corequisites"`
	Prerequisites []map[string]interface{} `json:"prerequisites"`
	CPRequired    int                      `json:"cp_required"`
}

func getNumberFromMsg(msg string) int {
	numStr := numPattern.FindString(msg)
	if numStr != "" {
		num, _ := strconv.Atoi(numStr)
		return num
	}
	return 0
}

func getNamedUnits(msg string) []string {
	return unitPattern.FindAllString(msg, -1)
}

func rulesToRequisites(rules Rule) map[string][]RawRequisite {
	unitRequisites := make(map[string][]RawRequisite)

	for _, rule := range rules.CourseErrors {
		unitCode := rule.References[0].UnitCode
		if _, exists := unitRequisites[unitCode]; !exists {
			unitRequisites[unitCode] = make([]RawRequisite, 0)
		}

		if rule.Title != "Duplicate unit" {
			unitRequisites[unitCode] = append(unitRequisites[unitCode], RawRequisite{
				Title:       rule.Title,
				Description: rule.Description,
			})
		}
	}

	return unitRequisites
}

func trimSpaces(units []string) []string {
	trimmedUnits := make([]string, len(units))
	for i, unit := range units {
		trimmedUnits[i] = strings.TrimSpace(unit)
	}
	return trimmedUnits
}

func refineRequisites(requsiteResults map[string][]RawRequisite) map[string]*RefinedRequisite {
	parsedRequisites := make(map[string]*RefinedRequisite)
	// Internal Note: This does not cover every unit, and nil's them by default, to fix
	for unit, unitRules := range requsiteResults {
		parsedRequisites[unit] = &RefinedRequisite{
			Permission:    false,
			Prohibitions:  make([]string, 0),
			Corequisites:  make([]map[string]interface{}, 0),
			Prerequisites: make([]map[string]interface{}, 0),
			CPRequired:    0,
		}

		for _, unitRule := range unitRules {
			switch unitRule.Title {
			case "Prohibited unit":
				parsedRequisites[unit].Prohibitions = append(
					parsedRequisites[unit].Prohibitions,
					getNamedUnits(unitRule.Description)...,
				)

			case "Have not enrolled in a unit", "Have not completed enough units":
				parsedRequisites[unit].Prerequisites = append(
					parsedRequisites[unit].Prerequisites,
					map[string]interface{}{
						"NumReq": 1,
						"units":  getNamedUnits(unitRule.Description),
					},
				)

			case "Have not passed enough units", "Missing corequisites":
				splits := strings.Split(unitRule.Description, ":")
				numberRequiredMsg, unitsMsg := splits[0], splits[1]

				if strings.Contains(unitRule.Title, "Have not passed enough units") {
					parsedRequisites[unit].Prerequisites = append(
						parsedRequisites[unit].Prerequisites,
						map[string]interface{}{
							"NumReq": getNumberFromMsg(numberRequiredMsg),
							"units":  trimSpaces(strings.Split(strings.ReplaceAll(unitsMsg, " or", ","), ", ")), // Need to strip here
						},
					)
				} else {
					parsedRequisites[unit].Corequisites = append(
						parsedRequisites[unit].Corequisites,
						map[string]interface{}{
							"NumReq": getNumberFromMsg(numberRequiredMsg),
							"units":  trimSpaces(strings.Split(strings.ReplaceAll(unitsMsg, " or", ","), ", ")),
						},
					)
				}

			case "Not enough passed credit points", "Not enough enrolled credit points":
				parsedRequisites[unit].CPRequired = getNumberFromMsg(unitRule.Description)

			case "Permission is required for this unit":
				parsedRequisites[unit].Permission = true
			}
		}

	}

	return parsedRequisites
}

func ProcessRequisites() map[string]*RefinedRequisite {
	var requisite_rules []Rule
	var prohibition_rules []Rule

	file1, err := os.ReadFile("data/raw_prerequisites.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(file1, &requisite_rules); err != nil {
		log.Fatal(err)
	}

	file2, err := os.ReadFile("data/raw_prohibitions.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(file2, &prohibition_rules); err != nil {
		log.Fatal(err)
	} // this appears so much...

	var rules = Rule{}

	for _, rule := range requisite_rules {
		rules.CourseErrors = append(rules.CourseErrors, rule.CourseErrors...)
	}

	for _, rule := range prohibition_rules { // have to filter here
		for _, message := range rule.CourseErrors {
			if message.Title == "Prohibited Unit" {
				rules.CourseErrors = append(rules.CourseErrors, message)
			}
		}

	}

	rawRequisites := rulesToRequisites(rules)
	return refineRequisites(rawRequisites)

}

func ProcessHandbook() map[string]interface{} {

	processesdRequisites := ProcessRequisites()

	var processedHandbook map[string]interface{}

	file, err := os.ReadFile("data/formatted_units.json") // Separate loading
	if err != nil {
		log.Fatal(err)
	}

	if err := json.Unmarshal(file, &processedHandbook); err != nil {
		log.Fatal(err)
	}

	for unitCode := range processedHandbook {
		processedHandbook[unitCode].(map[string]interface{})["requisites"] = processesdRequisites[unitCode]
	}

	return processedHandbook

}
