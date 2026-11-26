//go:build unit

package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type groupCacheSettingRepo struct {
	SettingRepository
	values map[string]string
	sets   map[string]string
}

func (r *groupCacheSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *groupCacheSettingRepo) Set(_ context.Context, key, value string) error {
	if r.sets == nil {
		r.sets = map[string]string{}
	}
	r.sets[key] = value
	return nil
}

type groupCacheGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r *groupCacheGroupRepo) ListActive(context.Context) ([]Group, error) { return r.groups, nil }

func (r *groupCacheGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	for i := range r.groups {
		if r.groups[i].ID == id {
			return &r.groups[i], nil
		}
	}
	return nil, ErrGroupNotFound
}

type groupCacheUsageRepo struct {
	UsageLogRepository
	rows []usagestats.GroupCacheUsageSummary
}

func (r *groupCacheUsageRepo) GetGroupCacheUsageSummary(context.Context, time.Time, time.Time) ([]usagestats.GroupCacheUsageSummary, error) {
	return r.rows, nil
}

func groupCacheFloor(enabled bool, floor *float64) PublicTransitGroupCacheSetting {
	return PublicTransitGroupCacheSetting{Enabled: enabled, Floor: floor}
}

func groupCacheWindow(input, creation, read int64, rate float64) PublicTransitCacheUsageWindow {
	return PublicTransitCacheUsageWindow{
		InputTokens:         input,
		CacheCreationTokens: creation,
		CacheReadTokens:     read,
		CacheHitRate:        rate,
	}
}

func TestApplyPublicTransitCacheFloor_LiftsRateAndScalesCacheReadTokens(t *testing.T) {
	for _, tc := range []struct {
		floor  float64
		reads  int64
		within float64
	}{
		{floor: 85, reads: 680, within: 1e-9},
		{floor: 85.5, reads: 708, within: 0.01},
	} {
		got := applyPublicTransitCacheFloor(groupCacheWindow(100, 20, 30, 20), groupCacheFloor(true, &tc.floor))
		require.Equal(t, int64(100), got.InputTokens)
		require.Equal(t, int64(20), got.CacheCreationTokens)
		require.Equal(t, tc.reads, got.CacheReadTokens)
		require.InDelta(t, tc.floor, got.CacheHitRate, 1e-12)
		// The scaled tokens recompute to the disclosed rate.
		recomputed := float64(got.CacheReadTokens) / float64(got.InputTokens+got.CacheCreationTokens+got.CacheReadTokens) * 100
		require.InDelta(t, tc.floor, recomputed, tc.within)
	}
}

func TestApplyPublicTransitCacheFloor_LeavesWindowUntouched(t *testing.T) {
	eighty, hundred := 80.0, 100.0
	for name, tc := range map[string]struct {
		window  PublicTransitCacheUsageWindow
		setting PublicTransitGroupCacheSetting
	}{
		"paused":                    {groupCacheWindow(100, 20, 30, 20), groupCacheFloor(false, &eighty)},
		"unset floor":               {groupCacheWindow(100, 20, 30, 20), groupCacheFloor(true, nil)},
		"real rate above floor":     {groupCacheWindow(100, 20, 490, 80.33), groupCacheFloor(true, &eighty)},
		"real rate equals floor":    {groupCacheWindow(100, 20, 480, 80), groupCacheFloor(true, &eighty)},
		"no prompt tokens":          {groupCacheWindow(0, 0, 0, 0), groupCacheFloor(true, &eighty)},
		"out of range stored floor": {groupCacheWindow(100, 20, 30, 20), groupCacheFloor(true, &hundred)},
	} {
		require.Equal(t, tc.window, applyPublicTransitCacheFloor(tc.window, tc.setting), name)
	}
}

func TestApplyPublicTransitCacheFloors_AdjustsEachWindowIndependently(t *testing.T) {
	floor := 50.0
	usage := PublicTransitCacheUsage{
		Last24h: groupCacheWindow(100, 0, 10, 9.09),
		Last7d:  groupCacheWindow(100, 100, 100, 33.33),
		Total:   groupCacheWindow(10, 10, 80, 80),
	}

	got := applyPublicTransitCacheFloors(usage, groupCacheFloor(true, &floor))

	require.Equal(t, int64(100), got.Last24h.CacheReadTokens)
	require.InDelta(t, 50, got.Last24h.CacheHitRate, 1e-12)
	require.Equal(t, int64(200), got.Last7d.CacheReadTokens)
	require.InDelta(t, 50, got.Last7d.CacheHitRate, 1e-12)
	require.Equal(t, usage.Total, got.Total)
}

func TestPublicTransitGroupCacheSettings_IgnoresMissingUnreadableAndInvalidRows(t *testing.T) {
	repo := &groupCacheSettingRepo{values: map[string]string{
		"public_transit_group_cache.1": `{"enabled":true,"floor":85.5}`,
		"public_transit_group_cache.2": `{not json`,
		"public_transit_group_cache.3": `{"enabled":true,"floor":100}`,
		"public_transit_group_cache.4": `{"enabled":true}`,
	}}
	s := &PublicTransitService{settingService: &SettingService{settingRepo: repo}}

	settings, err := s.publicTransitGroupCacheSettings(context.Background(), []int64{1, 2, 3, 4, 5, 1})
	require.NoError(t, err)
	require.Len(t, settings, 5)
	require.True(t, settings[1].Enabled)
	require.NotNil(t, settings[1].Floor)
	require.InDelta(t, 85.5, *settings[1].Floor, 1e-12)
	for _, groupID := range []int64{2, 3, 5} {
		require.Equal(t, PublicTransitGroupCacheSetting{}, settings[groupID], "group %d", groupID)
	}
	require.True(t, settings[4].Enabled)
	require.Nil(t, settings[4].Floor)
}

func TestPublicTransitGroups_AppliesOnlyEnabledFloorsAndKeepsOrder(t *testing.T) {
	floor := 85.0
	repo := &groupCacheSettingRepo{values: map[string]string{
		"public_transit_group_cache.10": `{"enabled":true,"floor":85}`,
		"public_transit_group_cache.11": `{"enabled":false,"floor":85}`,
	}}
	s := &PublicTransitService{settingService: &SettingService{settingRepo: repo}}
	usage := PublicTransitCacheUsage{Last24h: groupCacheWindow(0, 0, 0, 0), Last7d: groupCacheWindow(0, 0, 0, 0), Total: groupCacheWindow(100, 20, 30, 20)}
	entries := []publicTransitGroupEntry{
		{ID: 10, Group: PublicTransitGroup{Name: "a", CacheUsage: usage}},
		{ID: 11, Group: PublicTransitGroup{Name: "b", CacheUsage: usage}},
		{ID: 12, Group: PublicTransitGroup{Name: "c", CacheUsage: usage}},
	}

	groups, err := s.publicTransitGroups(context.Background(), entries)
	require.NoError(t, err)
	require.Len(t, groups, 3)
	require.Equal(t, "a", groups[0].Name)
	require.Equal(t, "b", groups[1].Name)
	require.Equal(t, "c", groups[2].Name)

	require.Equal(t, int64(680), groups[0].CacheUsage.Total.CacheReadTokens)
	require.InDelta(t, floor, groups[0].CacheUsage.Total.CacheHitRate, 1e-12)
	// Paused and unconfigured groups keep their real values.
	for _, group := range groups[1:] {
		require.Equal(t, usage.Total, group.CacheUsage.Total)
	}
}

func TestUpdateGroupCacheSetting_ValidatesFloorAndStoresOneGroup(t *testing.T) {
	groupRepo := &groupCacheGroupRepo{groups: []Group{{ID: 7, Name: "public", Platform: "anthropic", Status: StatusActive}}}
	repo := &groupCacheSettingRepo{values: map[string]string{"public_transit_group_cache.8": `{"enabled":true,"floor":70}`}}
	s := NewPublicTransitService(nil, nil, &SettingService{settingRepo: repo}, nil, groupRepo, nil)

	for _, floor := range []float64{0, -1, 100, 150, 85.55, math.NaN(), math.Inf(1)} {
		_, err := s.UpdateGroupCacheSetting(context.Background(), 7, groupCacheFloor(true, &floor))
		require.True(t, infraerrors.IsBadRequest(err), "floor %v", floor)
	}
	_, err := s.UpdateGroupCacheSetting(context.Background(), 0, groupCacheFloor(true, nil))
	require.True(t, infraerrors.IsBadRequest(err))
	_, err = s.UpdateGroupCacheSetting(context.Background(), 99, groupCacheFloor(true, nil))
	require.ErrorIs(t, err, ErrGroupNotFound)
	require.Empty(t, repo.sets)

	floor := 85.5
	got, err := s.UpdateGroupCacheSetting(context.Background(), 7, groupCacheFloor(true, &floor))
	require.NoError(t, err)
	require.Equal(t, groupCacheFloor(true, &floor), got)
	require.Equal(t, map[string]string{"public_transit_group_cache.7": `{"enabled":true,"floor":85.5}`}, repo.sets)
	// Another group's row is left alone.
	require.Equal(t, `{"enabled":true,"floor":70}`, repo.values["public_transit_group_cache.8"])

	got, err = s.UpdateGroupCacheSetting(context.Background(), 7, groupCacheFloor(true, nil))
	require.NoError(t, err)
	require.Nil(t, got.Floor)
	require.Equal(t, `{"enabled":true,"floor":null}`, repo.sets["public_transit_group_cache.7"])
}

// newGroupCacheAdminService exports "public-pro" (anthropic, enabled) and
// "public-lite" (openai, paused).
func newGroupCacheAdminService() *PublicTransitService {
	groupRepo := &groupCacheGroupRepo{groups: []Group{
		{ID: 10, Name: "public-pro", Platform: "anthropic", RateMultiplier: 1.25, Status: StatusActive},
		{ID: 11, Name: "private-vip", Platform: "anthropic", Status: StatusActive, IsExclusive: true},
		{ID: 12, Name: "public-lite", Platform: "openai", RateMultiplier: 0.5, Status: StatusActive},
	}}
	channels := []Channel{{
		ID:           1,
		Name:         "primary",
		Status:       StatusActive,
		GroupIDs:     []int64{10},
		ModelPricing: []ChannelModelPricing{{Platform: "anthropic", Models: []string{"claude-sonnet-4"}, BillingMode: BillingModeToken}},
	}}
	usageRepo := &groupCacheUsageRepo{rows: []usagestats.GroupCacheUsageSummary{{
		GroupID: 10,
		Total:   usagestats.GroupCacheUsageWindow{InputTokens: 900, CacheCreationTokens: 100, CacheReadTokens: 1000, CacheHitRate: 50},
	}}}
	repo := &groupCacheSettingRepo{values: map[string]string{
		SettingKeyChannelMonitorEnabled: "false",
		"public_transit_group_cache.10": `{"enabled":true,"floor":85}`,
	}}
	return NewPublicTransitService(newAvailableChannelService(channels, groupRepo), nil, &SettingService{settingRepo: repo}, nil, groupRepo, usageRepo)
}

func TestAdminGroupCache_ReportsRealRateAndStoredSetting(t *testing.T) {
	page, err := newGroupCacheAdminService().AdminGroupCache(context.Background(), PublicTransitGroupCacheQuery{})
	require.NoError(t, err)
	rows := page.Items
	require.Len(t, rows, 2)

	require.Equal(t, int64(10), rows[0].GroupID)
	require.Equal(t, "public-pro", rows[0].Name)
	require.Equal(t, "anthropic", rows[0].Platform)
	require.InDelta(t, 1.25, rows[0].RateMultiplier, 1e-12)
	require.Equal(t, 1, rows[0].ModelCount)
	// The console shows the real rate and tokens even though the snapshot would disclose 85.
	require.InDelta(t, 50, rows[0].CacheHitRate, 1e-12)
	require.Equal(t, int64(900), rows[0].InputTokens)
	require.Equal(t, int64(100), rows[0].CacheCreationTokens)
	require.Equal(t, int64(1000), rows[0].CacheReadTokens)
	require.True(t, rows[0].Setting.Enabled)
	require.NotNil(t, rows[0].Setting.Floor)
	require.InDelta(t, 85, *rows[0].Setting.Floor, 1e-12)
	require.Nil(t, rows[0].Monitor)

	require.Equal(t, int64(12), rows[1].GroupID)
	require.Equal(t, "public-lite", rows[1].Name)
	require.Zero(t, rows[1].CacheHitRate)
	require.Zero(t, rows[1].InputTokens)
	require.Zero(t, rows[1].CacheCreationTokens)
	require.Zero(t, rows[1].CacheReadTokens)
	require.Equal(t, PublicTransitGroupCacheSetting{}, rows[1].Setting)
}

func TestAdminGroupCache_FiltersByQuery(t *testing.T) {
	s := newGroupCacheAdminService()

	for name, tc := range map[string]struct {
		query    PublicTransitGroupCacheQuery
		expected []int64
	}{
		"no conditions":             {PublicTransitGroupCacheQuery{}, []int64{10, 12}},
		"name keyword":              {PublicTransitGroupCacheQuery{Search: "pro"}, []int64{10}},
		"name keyword ignores case": {PublicTransitGroupCacheQuery{Search: " PUBLIC-PRO "}, []int64{10}},
		"platform":                  {PublicTransitGroupCacheQuery{Platform: "OpenAI"}, []int64{12}},
		"enabled":                   {PublicTransitGroupCacheQuery{Status: "enabled"}, []int64{10}},
		"paused":                    {PublicTransitGroupCacheQuery{Status: "paused"}, []int64{12}},
		"all conditions must match": {PublicTransitGroupCacheQuery{Search: "public", Platform: "openai", Status: "enabled"}, []int64{}},
		"combined":                  {PublicTransitGroupCacheQuery{Search: "public", Platform: "anthropic", Status: "enabled"}, []int64{10}},
		"nothing matches":           {PublicTransitGroupCacheQuery{Search: "nomatch"}, []int64{}},
	} {
		page, err := s.AdminGroupCache(context.Background(), PublicTransitGroupCacheQuery{
			Search: strings.TrimSpace(tc.query.Search), Platform: tc.query.Platform, Status: tc.query.Status,
		})
		require.NoError(t, err, name)
		ids := make([]int64, 0, len(page.Items))
		for _, row := range page.Items {
			ids = append(ids, row.GroupID)
		}
		require.Equal(t, tc.expected, ids, name)
		// An empty result is still a list, never a missing one.
		require.NotNil(t, page.Items, name)
	}

	_, err := s.AdminGroupCache(context.Background(), PublicTransitGroupCacheQuery{Status: "bogus"})
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestPublicTransitGroupCacheMonitor_PrefersProviderMatchThenName(t *testing.T) {
	group := PublicTransitGroup{Name: "Claude-Max", Platform: "anthropic"}

	require.Nil(t, publicTransitGroupCacheMonitor(group, []PublicTransitMonitor{{Provider: "openai", GroupName: "Other", PrimaryStatus: "operational", PrimaryModel: "gpt"}}))
	require.Nil(t, publicTransitGroupCacheMonitor(group, []PublicTransitMonitor{{Provider: "anthropic", PrimaryStatus: "operational", PrimaryModel: "claude"}}))

	// Without a provider match the group name alone decides.
	require.Equal(t,
		&PublicTransitGroupCacheMonitor{Status: "failed", Model: "gpt"},
		publicTransitGroupCacheMonitor(group, []PublicTransitMonitor{{Provider: "openai", GroupName: "claude-max", PrimaryStatus: "failed", PrimaryModel: "gpt"}}),
	)

	monitors := []PublicTransitMonitor{
		{Provider: "openai", GroupName: "Claude-Max", PrimaryStatus: "failed", PrimaryModel: "gpt"},
		{Provider: "anthropic", GroupName: "Claude-Max", PrimaryStatus: "operational", PrimaryModel: "claude"},
	}
	require.Equal(t,
		&PublicTransitGroupCacheMonitor{Status: "operational", Model: "claude"},
		publicTransitGroupCacheMonitor(group, monitors),
	)

	// Later monitors win, as on the public page.
	monitors = append(monitors, PublicTransitMonitor{Provider: "anthropic", GroupName: "Claude-Max", PrimaryStatus: "degraded", PrimaryModel: "claude-2"})
	require.Equal(t,
		&PublicTransitGroupCacheMonitor{Status: "degraded", Model: "claude-2"},
		publicTransitGroupCacheMonitor(group, monitors),
	)
}
