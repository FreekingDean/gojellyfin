package items

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func descending(order *[]api.SortOrder) bool {
	if order == nil {
		return false
	}

	for _, value := range *order {
		if value == api.Descending {
			return true
		}
	}

	return false
}

func sortFields(sortBy *[]api.ItemSortBy) []string {
	if sortBy == nil {
		return nil
	}

	fields := make([]string, 0, len(*sortBy))
	for _, field := range *sortBy {
		fields = append(fields, string(field))
	}

	return fields
}
