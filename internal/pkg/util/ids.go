package util

import "fmt"

// NormalizeIDs 校验并去重一批 id，保持首次出现顺序。
// rejectEmpty 控制空批次是否报错（删除管理员允许空批次，由其调用方短路）；
// invalidIDErr 为每个非正 id 构造错误，让各调用点保留自己的文案与哨兵值。
// 空批次被拒绝时返回 nil，否则返回非 nil 的空切片，与各站点历史契约一致。
func NormalizeIDs(ids []int32, entity string, rejectEmpty bool, invalidIDErr func(id int32) error) ([]int32, error) {
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
