package format

import "strconv"

// Helper function to parse int values safely
func parseInt(value interface{}) int {
	if v, ok := value.(string); ok {
		if intValue, err := strconv.Atoi(v); err == nil {
			return intValue
		}
	}
	if v, ok := value.(float64); ok {
		return int(v)
	}
	return 0
}
func FormatAOS(raw_aos map[string]interface{}) map[string]interface{} {
	// Extract curriculumStructure.container if available

	return map[string]interface{}{
		"title":                raw_aos["title"],
		"code":                 raw_aos["code"],
		"study_level":          raw_aos["study_level"],
		"credit_points":        raw_aos["credit_points"],
		"handbook_description": raw_aos["handbook_description"],
		"aos_type":             raw_aos["academic_item_type"],
		"school":               raw_aos["school"].(map[string]interface{})["name"],
		"locations":            raw_aos["aos_offering_locations"],
		"curriculum_structure": FormatStructure(raw_aos),
	}
}

func FormatAOSs(raw_aoss []map[string]interface{}) map[string]interface{} {

	var formatted_aos_data = make(map[string]interface{})

	for _, unit := range raw_aoss {
		code, ok := unit["code"].(string)
		if !ok {
			continue
		}
		formatted_aos_data[code] = FormatAOS(unit)

	}

	return formatted_aos_data

}
