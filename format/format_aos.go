package format

type StructureElement struct {
	Title           string             `json:"title"`
	Description     string             `json:"description,omitempty"`
	CreditPoints    int                `json:"credit_points,omitempty"`
	Courses         map[string]string  `json:"courses,omitempty"`
	Structure       []StructureElement `json:"structure,omitempty"`
	ParentConnector string             `json:"parent_connector,omitempty"`
}

func getMonashStructure(structure []StructureElement, currContainer []map[string]interface{}) {
	for _, element := range currContainer {
		newElement := StructureElement{
			Title:           element["title"].(string),
			Description:     element["description"].(string),
			CreditPoints:    element["credit_points"].(int),
			Courses:         make(map[string]string),
			Structure:       make([]StructureElement, 0),
			ParentConnector: element["parent_connector"].(map[string]interface{})["label"].(string),
		}

		structure = append(structure, newElement)

		if relationship, ok := element["relationship"].([]interface{}); ok && len(relationship) > 0 {
			for _, course := range relationship {
				if courseMap, ok := course.(map[string]interface{}); ok {
					if academicItemCode, ok := courseMap["academic_item_code"].(string); ok {
						newElement.Courses[academicItemCode] = courseMap["academic_item_name"].(string)
					} else if description, ok := courseMap["description"].(string); ok && description != "" {
						newElement.Courses[description] = "1"
					}
				}
			}
		}
		if container, ok := element["container"].([]map[string]interface{}); ok && len(container) > 0 {
			getMonashStructure(newElement.Structure, container)
		}
	}
}

func FormatAOS(raw_aos map[string]interface{}) map[string]interface{} {
	// Considering adding: offered_by (237)
	return map[string]interface{}{
		"title":                raw_aos["title"],
		"code":                 raw_aos["code"],
		"study_level":          raw_aos["study_level"],
		"credit_points":        raw_aos["credit_points"],
		"handbook_description": raw_aos["handbook_description"],
		"aos_type":             raw_aos["academic_item_type"],
		"school":               raw_aos["school"].(map[string]interface{})["name"],
		"locations":            raw_aos["aos_offering_locations"],
	}
}
