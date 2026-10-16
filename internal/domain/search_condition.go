package domain

import "fmt"

// SearchCondition is one independent clause. Conditions are combined with AND.
// Values may contain a legacy union; new UI clauses contain one value.
type SearchCondition struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

func (c SearchCondition) Validate(entity string) error {
	if len(c.Values) == 0 || len(c.Values) > 1000 {
		return fmt.Errorf("invalid condition values")
	}
	switch c.Field {
	case "keyword", "name_contains", "name_equals", "from", "to":
		if len(c.Values) != 1 {
			return fmt.Errorf("condition requires one value")
		}
	case "tag_all", "tag_any", "tag_not":
	case "kind", "box":
		if entity != "files" {
			return fmt.Errorf("file condition in box search")
		}
	default:
		return fmt.Errorf("unknown condition field")
	}
	for _, value := range c.Values {
		if value == "" || len(value) > 4096 {
			return fmt.Errorf("invalid condition value")
		}
		switch c.Field {
		case "box", "tag_all", "tag_any", "tag_not":
			id, err := ParseID(value)
			if err != nil || id.IsZero() {
				return fmt.Errorf("invalid condition reference")
			}
		case "from", "to":
			if _, err := ParseLocalDate(value); err != nil {
				return err
			}
		case "kind":
			switch value {
			case "image", "video", "audio", "document", "other":
			default:
				return fmt.Errorf("invalid media kind")
			}
		}
	}
	return nil
}
