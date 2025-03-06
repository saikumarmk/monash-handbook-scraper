package format

type StructureElement struct {
	Title           string             `json:"title"`
	Description     string             `json:"description,omitempty"`
	CreditPoints    int                `json:"credit_points,omitempty"`
	Courses         map[string]string  `json:"courses,omitempty"`
	Structure       []StructureElement `json:"structure,omitempty"`
	ParentConnector string             `json:"parent_connector,omitempty"`
}

func getMonashStructure(structure *[]StructureElement, currContainer []map[string]interface{}) {
	for _, element := range currContainer {
		newElement := StructureElement{
			Title:        element["title"].(string),
			Description:  element["description"].(string),
			CreditPoints: parseInt(element["credit_points"]),
			Courses:      make(map[string]string),
			Structure:    []StructureElement{},
		}

		if parent, ok := element["parent_connector"].(map[string]interface{}); ok {
			if label, ok := parent["label"].(string); ok {
				newElement.ParentConnector = label
			}
		}

		// Extract courses
		if relationship, ok := element["relationship"].([]interface{}); ok {
			for _, course := range relationship {
				if courseMap, ok := course.(map[string]interface{}); ok {
					if academicItemCode, ok := courseMap["academic_item_code"].(string); ok {
						newElement.Courses[academicItemCode] = courseMap["academic_item_name"].(string)
					}
				}
			}
		}

		// Recursively process nested structure
		if container, ok := element["container"].([]interface{}); ok {
			childContainers := make([]map[string]interface{}, 0)
			for _, item := range container {
				if itemMap, ok := item.(map[string]interface{}); ok {
					childContainers = append(childContainers, itemMap)
				}
			}
			getMonashStructure(&newElement.Structure, childContainers)
		}

		*structure = append(*structure, newElement)
	}
}

func FormatStructure(raw_item map[string]interface{}) []StructureElement {

	var structure []StructureElement
	curriculumStructure, ok := raw_item["curriculumStructure"]
	if ok {
		if container, ok := curriculumStructure.(map[string]interface{})["container"].([]interface{}); ok {
			currContainer := make([]map[string]interface{}, 0)
			for _, item := range container {
				if itemMap, ok := item.(map[string]interface{}); ok {
					currContainer = append(currContainer, itemMap)
				}
			}
			getMonashStructure(&structure, currContainer)
		}
	}
	return structure

}
