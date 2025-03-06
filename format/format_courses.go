package format

func FormatCourse(raw_course map[string]interface{}) map[string]interface{} {
	// Extract curriculumStructure.container if available, 73 courses without course maps

	return map[string]interface{}{
		"title":                raw_course["title"],
		"code":                 raw_course["code"],
		"abbreviated_name":     raw_course["abbreviated_name"],
		"aqf_level":            raw_course["aqf_level"].(map[string]interface{})["label"],
		"aos_type":             raw_course["academic_item_type"],
		"school":               raw_course["school"].(map[string]interface{})["value"],
		"structure":            raw_course["structure"],
		"curriculum_structure": FormatStructure(raw_course),
	}
}

func FormatCourses(raw_courses []map[string]interface{}) map[string]interface{} {

	var formatted_course_data = make(map[string]interface{})

	for _, unit := range raw_courses {
		code, ok := unit["code"].(string)
		if !ok {
			continue
		}
		formatted_course_data[code] = FormatCourse(unit)

	}

	return formatted_course_data

}
