package migrations

import (
	"reflect"
	"testing"
)

func TestSummarizeStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		report StatusReport
		want   StatusSummary
	}{
		{
			// 三轨全部完成：done 等于 total，pending/incomplete 为 0。
			name: "全部完成",
			report: StatusReport{
				State: InstallStrictUpgrade,
				Tracks: []TrackStatus{
					{Track: "official", Registered: 12, Applied: 12},
					{Track: "framework", Registered: 1, Applied: 1},
					{Track: "business", Registered: 3, Applied: 3},
				},
			},
			want: StatusSummary{
				State:     InstallStrictUpgrade,
				Official:  TrackSummary{Done: 12, Total: 12},
				Framework: TrackSummary{Done: 1, Total: 1},
				Business:  TrackSummary{Done: 3, Total: 3},
			},
		},
		{
			// 有待执行：pending 来自 TrackStatus，按轨道分别落到对应字段。
			name: "有待执行",
			report: StatusReport{
				State: InstallStrictUpgrade,
				Tracks: []TrackStatus{
					{Track: "official", Registered: 12, Applied: 12},
					{Track: "framework", Registered: 1, Applied: 1},
					{Track: "business", Registered: 3, Applied: 1, Pending: 2},
				},
			},
			want: StatusSummary{
				State:     InstallStrictUpgrade,
				Official:  TrackSummary{Done: 12, Total: 12},
				Framework: TrackSummary{Done: 1, Total: 1},
				Business:  TrackSummary{Done: 1, Total: 3, Pending: 2},
			},
		},
		{
			// 有 incomplete（end_time 为空的中断记录）：原样透传到对应轨道。
			name: "有未完成记录",
			report: StatusReport{
				State: InstallStrictUpgrade,
				Tracks: []TrackStatus{
					{Track: "official", Registered: 12, Applied: 12},
					{Track: "framework", Registered: 1, Applied: 1, Incomplete: 1},
					{Track: "business", Registered: 3, Applied: 3},
				},
			},
			want: StatusSummary{
				State:     InstallStrictUpgrade,
				Official:  TrackSummary{Done: 12, Total: 12},
				Framework: TrackSummary{Done: 1, Total: 1, Incomplete: 1},
				Business:  TrackSummary{Done: 3, Total: 3},
			},
		},
		{
			// 未安装/空台账：Status 不返回轨道，摘要保持三轨零值。
			name:   "空台账",
			report: StatusReport{State: InstallFresh},
			want:   StatusSummary{State: InstallFresh},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SummarizeStatus(tt.report); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SummarizeStatus() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
