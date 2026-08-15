package admin

import (
	"context"
	"time"
)

// MessageTraffic ?????????
type MessageTraffic struct {
	Hourly           []TimeCount          `json:"hourly"`
	Daily            []TimeCount          `json:"daily"`
	PrivateGroup     RoomTypeDistribution `json:"private_group"`
	TypeDistribution map[string]int64     `json:"type_distribution"`
	PeakHour         string               `json:"peak_hour"`
	TopRooms         []KeyCount           `json:"top_rooms"`
	TopUsers         []KeyCount           `json:"top_users"`
	GeneratedAt      time.Time            `json:"generated_at"`
}

type RoomTypeDistribution struct {
	Private int64 `json:"private"`
	Group   int64 `json:"group"`
}

// MessageTraffic ???? 24 ????? 7 ??????
func (s *DashboardService) buildMessageTraffic(ctx context.Context) (*MessageTraffic, error) {
	now := time.Now()
	last24h := now.Add(-24 * time.Hour)
	last7d := now.AddDate(0, 0, -6)
	dayStart := startOfDay(now)

	hourly, err := s.messageStore.HourlyCounts(ctx, last24h, now)
	if err != nil {
		return nil, err
	}
	daily, err := s.messageStore.DailyCounts(ctx, last7d, now.Add(time.Hour))
	if err != nil {
		return nil, err
	}

	private, group, err := s.messageStore.CountPrivateGroupMessages(ctx, dayStart, now)
	if err != nil {
		return nil, err
	}

	typeCounts, err := s.messageStore.CountMessagesByType(ctx, dayStart, now)
	if err != nil {
		return nil, err
	}
	typeDistribution := map[string]int64{
		"text":   typeCounts[1],
		"file":   typeCounts[2],
		"system": typeCounts[3],
	}
	agentCount, err := s.messageStore.CountAgentMentions(ctx, dayStart, now)
	if err != nil {
		return nil, err
	}
	typeDistribution["agent"] = agentCount

	topRooms, err := s.messageStore.TopRooms(ctx, last24h, now, 10)
	if err != nil {
		return nil, err
	}
	topUsers, err := s.messageStore.TopSenders(ctx, last24h, now, 10)
	if err != nil {
		return nil, err
	}

	return &MessageTraffic{
		Hourly:           hourly,
		Daily:            daily,
		PrivateGroup:     RoomTypeDistribution{Private: private, Group: group},
		TypeDistribution: typeDistribution,
		PeakHour:         peakHour(hourly),
		TopRooms:         topRooms,
		TopUsers:         topUsers,
		GeneratedAt:      now,
	}, nil
}

func peakHour(hourly []TimeCount) string {
	if len(hourly) == 0 {
		return ""
	}
	var peak TimeCount
	for _, item := range hourly {
		if item.Count > peak.Count {
			peak = item
		}
	}
	return peak.Time
}
