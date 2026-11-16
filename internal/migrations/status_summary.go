package migrations

// TrackSummary 是单条迁移轨道的计数摘要。
type TrackSummary struct {
	Done       int
	Total      int
	Pending    int
	Incomplete int
}

// StatusSummary 是三轨迁移进度的只读计数摘要；未安装时三条轨道均为零值。
type StatusSummary struct {
	State     InstallRecoveryState
	Official  TrackSummary
	Framework TrackSummary
	Business  TrackSummary
}

// SummarizeStatus 把 StatusReport 折算为固定三轨的计数，缺失的轨道按零值处理。
func SummarizeStatus(report StatusReport) StatusSummary {
	summary := StatusSummary{State: report.State}
	for _, track := range report.Tracks {
		counts := TrackSummary{
			Done:       track.Applied,
			Total:      track.Registered,
			Pending:    track.Pending,
			Incomplete: track.Incomplete,
		}
		switch track.Track {
		case "official":
			summary.Official = counts
		case "framework":
			summary.Framework = counts
		case "business":
			summary.Business = counts
		}
	}
	return summary
}
