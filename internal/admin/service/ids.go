package service

import "buildadmin-go/internal/pkg/util"

// normalizeIDs 保留 admin service 内部的调用形态，归一化实现在 util.NormalizeIDs。
func normalizeIDs(ids []int32, entity string, rejectEmpty bool, invalidIDErr func(id int32) error) ([]int32, error) {
	return util.NormalizeIDs(ids, entity, rejectEmpty, invalidIDErr)
}
