package format

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var unitPattern = regexp.MustCompile(`[A-Z]{3}[0-9]{4}`)

func pullHandbookRequisites(handbookDict map[string]interface{}) map[string]bool {
	prohibitions := make(map[string]bool)

	if manualRules, exists := handbookDict["enrolment_rules"]; exists {

		for _, rule := range manualRules.([]interface{}) {
			description := rule.(map[string]interface{})["description"]
			if strings.Contains(strings.ToUpper(description.(string)), "PROHIBITION") {

				matches := unitPattern.FindAllString(description.(string), -1)
				for _, match := range matches {
					prohibitions[match] = true
				}
			}
		}
	}

	if boxedRules, exists := handbookDict["requisites"]; exists {
		for _, ruleBox := range boxedRules.([]interface{}) {
			switch ruleType := ruleBox.(map[string]interface{})["requisite_type"].(map[string]interface{})["value"].(string); ruleType {
			case "prohibitions":
				container := ruleBox.(map[string]interface{})["container"].([]interface{})
				// Check if container is not empty before accessing [0]
				if len(container) > 0 {
					relationships := container[0].(map[string]interface{})["relationships"].([]interface{})
					for _, unitSpec := range relationships {
						unitValue := strings.Fields(unitSpec.(map[string]interface{})["academic_item"].(map[string]interface{})["value"].(string))[1]
						prohibitions[unitValue] = true
					}
				}
			}
		}
	}

	return prohibitions
}

func formatOffering(data interface{}) map[string]interface{} {
	offering, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"name":     offering["display_name"],
		"location": offering["location"].(map[string]interface{})["value"],
		"mode":     offering["attendance_mode"].(map[string]interface{})["value"],
		"period":   offering["teaching_period"].(map[string]interface{})["value"],
	}
}

func getOfferings(raw_offerings []interface{}) []map[string]interface{} {
	var offerings []map[string]interface{}

	for _, offering := range raw_offerings {

		offerings = append(offerings, formatOffering(offering))

	}

	return offerings
}

func possiblyExam(assessment map[string]interface{}) string {
	var name = assessment["assessment_name"].(string)
	var examType = assessment["assessment_type"].(map[string]interface{})["value"]

	if examType == nil {
		nameLower := strings.ToLower(name)

		if strings.Contains(nameLower, "exam") || strings.Contains(nameLower, strings.ToLower("Scheduled final assessment")) {
			return "exam"
		}

		if strings.Contains(nameLower, "lab") {
			return "lab"
		}

		if strings.Contains(nameLower, "tutorial") {
			return "tutorial"
		}

		if strings.Contains(nameLower, "applied") {
			return "applied"
		}

		return "unknown"
	}
	return examType.(string)
}

func formatAssessment(raw_assessment interface{}) map[string]interface{} {
	var assessment, ok = raw_assessment.(map[string]interface{})

	if !ok {
		return nil
	}

	return map[string]interface{}{
		"name": assessment["assessment_name"].(string),
		"type": possiblyExam(assessment),
	}
}

func getAssessments(raw_assessments []interface{}) []map[string]interface{} {
	var assessments []map[string]interface{}

	for _, assessment := range raw_assessments {
		assessments = append(assessments, formatAssessment(assessment))
	}

	return assessments
}

// ExtractSCABand safely retrieves the SCA band value
func ExtractSCABand(rawUnit map[string]interface{}) int {
	// Check if "highest_sca_band" exists
	highestSCA, exists := rawUnit["highest_sca_band"]
	if !exists {
		return 0
	}

	// Case 1: highest_sca_band is a map and contains "value"
	if bandMap, ok := highestSCA.(map[string]interface{}); ok {
		if val, exists := bandMap["value"]; exists {
			if strVal, ok := val.(string); ok {
				return extractLastDigit(strVal)
			}
		}
	}

	// Case 2: highest_sca_band is directly a string
	if str, ok := highestSCA.(string); ok {
		return extractLastDigit(str)
	}

	// Return 0 if no valid band found
	return 0
}

// extractLastDigit extracts the integer value from the last character of a string
func extractLastDigit(s string) int {
	if len(s) == 0 {
		return 0
	}
	lastChar := s[len(s)-1:]
	if num, err := strconv.Atoi(lastChar); err == nil {
		return num
	}
	return 0
}

func FormatUnit(raw_unit map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"title":         raw_unit["title"],
		"code":          raw_unit["code"],
		"credit_points": raw_unit["credit_points"],
		"level": func() int {
			if code, ok := raw_unit["code"].(string); ok && len(code) >= 4 {
				if num, err := strconv.Atoi(string(code[3])); err == nil {
					return num
				}
			}
			return 0
		}(),
		"sca_band":     ExtractSCABand(raw_unit),
		"academic_org": raw_unit["academic_org"].(map[string]interface{})["value"],
		"school":       raw_unit["school"].(map[string]interface{})["value"],
		"offerings":    getOfferings(raw_unit["unit_offering"].([]interface{})),
		"assessments":  getAssessments((raw_unit["assessments"].([]interface{}))),
	}
	// Add in enrolment rules and requisites here
}

// raw_unit["level"].(map[string]interface{})["value"],
// Generates two artefacts, one is IO'd
func FormatUnits(raw_units []map[string]interface{}) map[string]interface{} {

	var formatted_unit_data = make(map[string]interface{})
	var prohibition_candidates [][]string

	for _, unit := range raw_units {
		code, ok := unit["code"].(string)
		if !ok {
			continue
		}
		formatted_unit_data[code] = FormatUnit(unit)

	}

	for _, unit := range raw_units {
		code, ok := unit["code"].(string)
		if !ok {
			continue
		}
		prelim_candidate := pullHandbookRequisites(unit)
		var prohibition_candidate = make([]string, 0)
		prohibition_candidate = append(prohibition_candidate, code)

		// Get only existing units
		for candidate := range prelim_candidate {
			if _, ok := formatted_unit_data[candidate]; ok {
				prohibition_candidate = append(prohibition_candidate, candidate)
			}
		}
		if len(prohibition_candidate) > 1 {
			prohibition_candidates = append(prohibition_candidates, prohibition_candidate)
		}
	}

	data, err := json.Marshal(prohibition_candidates)

	if err != nil {
		fmt.Println("Encountered an error with prohibition JSON marshalling:", err)
	}
	if err := os.WriteFile("data/prohibition_candidates.json", data, 0644); err != nil {
		fmt.Println("Encountered an error writing:", err)
	}

	return formatted_unit_data

}
