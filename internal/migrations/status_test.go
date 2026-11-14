package migrations

import (
	"reflect"
	"testing"
)

func TestSummarizeTrack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		registered []string
		completed  []string
		incomplete int
		want       TrackStatus
	}{
		{
			// 注册名单与台账完全一致：全部已应用、无待应用。
			name:       "全部匹配",
			registered: []string{"a", "b"},
			completed:  []string{"a", "b"},
			want:       TrackStatus{Track: "business", Registered: 2, Applied: 2, Pending: 0},
		},
		{
			// 部分完成：只有台账里的 name 计入 Applied，其余按注册顺序进入 PendingNames。
			name:       "部分应用",
			registered: []string{"a", "b", "c"},
			completed:  []string{"a", "c"},
			want: TrackStatus{
				Track: "business", Registered: 3, Applied: 2, Pending: 1,
				PendingNames: []string{"b"},
			},
		},
		{
			// 台账存在但没有任何已完成记录（含 nil completed，模拟台账缺失）：全部名字待应用。
			name:       "全部未应用",
			registered: []string{"a", "b"},
			completed:  nil,
			want: TrackStatus{
				Track: "business", Registered: 2, Applied: 0, Pending: 2,
				PendingNames: []string{"a", "b"},
			},
		},
		{
			// 台账里存在注册名单之外的完成记录：忽略，不影响 Applied/Pending/PendingNames。
			name:       "台账多余名称被忽略",
			registered: []string{"a"},
			completed:  []string{"a", "legacy-x", "legacy-y"},
			want:       TrackStatus{Track: "business", Registered: 1, Applied: 1, Pending: 0},
		},
		{
			// PendingNames 保持注册顺序，不跟随台账顺序。
			name:       "待应用名单保持注册顺序",
			registered: []string{"c", "a", "b"},
			completed:  []string{"a"},
			want: TrackStatus{
				Track: "business", Registered: 3, Applied: 1, Pending: 2,
				PendingNames: []string{"c", "b"},
			},
		},
		{
			// incomplete 由调用方统计后透传，不参与计算。
			name:       "incomplete 原样透传",
			registered: []string{"a", "b"},
			completed:  []string{"a"},
			incomplete: 3,
			want: TrackStatus{
				Track: "business", Registered: 2, Applied: 1, Pending: 1, Incomplete: 3,
				PendingNames: []string{"b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := summarizeTrack("business", tt.registered, tt.completed, tt.incomplete)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("summarizeTrack() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
