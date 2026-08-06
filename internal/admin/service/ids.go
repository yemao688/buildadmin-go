package service

import "fmt"

// normalizeIDs validates and de-duplicates a batch of ids, preserving order.
// rejectEmpty controls whether an empty input is an error (administrator
// deletion intentionally accepts it and lets the caller short-circuit);
// invalidIDErr builds the per-id error so each caller keeps its exact error
// shape and message. Empty input rejected returns nil, otherwise a non-nil
// empty slice — matching each historical normalizeXxxIDs contract.
func normalizeIDs(ids []int32, entity string, rejectEmpty bool, invalidIDErr func(id int32) error) ([]int32, error) {
	if rejectEmpty && len(ids) == 0 {
		return nil, fmt.Errorf("invalid %s ids", entity)
	}
	seen := make(map[int32]struct{}, len(ids))
	normalized := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, invalidIDErr(id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return normalized, nil
}
