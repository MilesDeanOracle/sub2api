package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// PublicTransitGroupCacheSetting is the per-group floor of the cache hit rate
// disclosed by the public snapshot. Floor is a percentage; nil means unset.
type PublicTransitGroupCacheSetting struct {
	Enabled bool     `json:"enabled"`
	Floor   *float64 `json:"floor"`
}

// PublicTransitGroupCacheRow is one operations console row. CacheHitRate and the
// token details are real cumulative values and are never adjusted, so the
// console always contrasts them with the configured floor.
type PublicTransitGroupCacheRow struct {
	GroupID             int64                           `json:"group_id"`
	Name                string                          `json:"name"`
	Platform            string                          `json:"platform"`
	RateMultiplier      float64                         `json:"rate_multiplier"`
	ModelCount          int                             `json:"model_count"`
	Monitor             *PublicTransitGroupCacheMonitor `json:"monitor,omitempty"`
	CacheHitRate        float64                         `json:"cache_hit_rate"`
	InputTokens         int64                           `json:"input_tokens"`
	CacheCreationTokens int64                           `json:"cache_creation_tokens"`
	CacheReadTokens     int64                           `json:"cache_read_tokens"`
	Setting             PublicTransitGroupCacheSetting  `json:"setting"`
}

// PublicTransitGroupCacheMonitor mirrors the availability summary of the public
// page: the status and model of the monitor bound to the group.
type PublicTransitGroupCacheMonitor struct {
	Status string `json:"status"`
	Model  string `json:"model"`
}

func publicTransitGroupCacheSettingKey(groupID int64) string {
	return SettingKeyPublicTransitGroupCachePrefix + strconv.FormatInt(groupID, 10)
}

func publicTransitGroupIDs(entries []publicTransitGroupEntry) []int64 {
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// PublicTransitGroupCacheQuery narrows the operations console list.
type PublicTransitGroupCacheQuery struct {
	Search   string // 分组名称关键字，不区分大小写
	Platform string
	Status   string // "enabled" 或 "paused"，留空表示全部
}

func (q PublicTransitGroupCacheQuery) validate() error {
	if q.Status != "" && q.Status != "enabled" && q.Status != "paused" {
		return infraerrors.BadRequest("INVALID_GROUP_CACHE_STATUS", "Status must be enabled or paused")
	}
	return nil
}

// matches reports whether a row satisfies every non-empty condition.
func (q PublicTransitGroupCacheQuery) matches(row PublicTransitGroupCacheRow) bool {
	if q.Search != "" && !strings.Contains(strings.ToLower(row.Name), strings.ToLower(q.Search)) {
		return false
	}
	if q.Platform != "" && !strings.EqualFold(row.Platform, q.Platform) {
		return false
	}
	switch q.Status {
	case "enabled":
		return row.Setting.Enabled
	case "paused":
		return !row.Setting.Enabled
	}
	return true
}

// PublicTransitGroupCachePage is the list response. Items is always present,
// even when nothing matches, because the generic envelope drops an empty data
// field and clients would otherwise receive no list at all.
type PublicTransitGroupCachePage struct {
	Items []PublicTransitGroupCacheRow `json:"items"`
}

// AdminGroupCache lists the exported groups matching the query, with their real
// cache hit rate and the stored floor. It works whether or not the public
// snapshot is enabled.
func (s *PublicTransitService) AdminGroupCache(ctx context.Context, query PublicTransitGroupCacheQuery) (*PublicTransitGroupCachePage, error) {
	if err := query.validate(); err != nil {
		return nil, err
	}
	entries, err := s.publicTransitEntries(ctx)
	if err != nil {
		return nil, err
	}
	monitors, err := s.publicMonitorItems(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.publicTransitGroupCacheSettings(ctx, publicTransitGroupIDs(entries))
	if err != nil {
		return nil, fmt.Errorf("load public group cache floors: %w", err)
	}
	return &PublicTransitGroupCachePage{Items: filterPublicTransitGroupCacheRows(buildPublicTransitGroupCacheRows(entries, monitors, settings), query)}, nil
}

func filterPublicTransitGroupCacheRows(rows []PublicTransitGroupCacheRow, query PublicTransitGroupCacheQuery) []PublicTransitGroupCacheRow {
	matched := make([]PublicTransitGroupCacheRow, 0, len(rows))
	for _, row := range rows {
		if query.matches(row) {
			matched = append(matched, row)
		}
	}
	return matched
}

// publicTransitGroups turns the exported entries into the snapshot groups with
// the stored floors applied to their disclosed cache usage.
func (s *PublicTransitService) publicTransitGroups(ctx context.Context, entries []publicTransitGroupEntry) ([]PublicTransitGroup, error) {
	settings, err := s.publicTransitGroupCacheSettings(ctx, publicTransitGroupIDs(entries))
	if err != nil {
		return nil, fmt.Errorf("load public group cache floors: %w", err)
	}
	groups := make([]PublicTransitGroup, 0, len(entries))
	for _, entry := range entries {
		entry.Group.CacheUsage = applyPublicTransitCacheFloors(entry.Group.CacheUsage, settings[entry.ID])
		groups = append(groups, entry.Group)
	}
	return groups, nil
}

func buildPublicTransitGroupCacheRows(entries []publicTransitGroupEntry, monitors []PublicTransitMonitor, settings map[int64]PublicTransitGroupCacheSetting) []PublicTransitGroupCacheRow {
	rows := make([]PublicTransitGroupCacheRow, 0, len(entries))
	for _, entry := range entries {
		group := entry.Group
		rows = append(rows, PublicTransitGroupCacheRow{
			GroupID:             entry.ID,
			Name:                group.Name,
			Platform:            group.Platform,
			RateMultiplier:      group.RateMultiplier,
			ModelCount:          len(group.Models),
			Monitor:             publicTransitGroupCacheMonitor(group, monitors),
			CacheHitRate:        group.CacheUsage.Total.CacheHitRate,
			InputTokens:         group.CacheUsage.Total.InputTokens,
			CacheCreationTokens: group.CacheUsage.Total.CacheCreationTokens,
			CacheReadTokens:     group.CacheUsage.Total.CacheReadTokens,
			Setting:             settings[entry.ID],
		})
	}
	return rows
}

// UpdateGroupCacheSetting stores the floor of one group. Groups that are not
// exported today may still be configured; the snapshot only reads the setting
// of the groups it exports.
func (s *PublicTransitService) UpdateGroupCacheSetting(ctx context.Context, groupID int64, setting PublicTransitGroupCacheSetting) (PublicTransitGroupCacheSetting, error) {
	if groupID <= 0 {
		return PublicTransitGroupCacheSetting{}, infraerrors.BadRequest("INVALID_GROUP_ID", "Invalid group ID")
	}
	if err := validatePublicTransitGroupCacheSetting(setting); err != nil {
		return PublicTransitGroupCacheSetting{}, err
	}
	if _, err := s.groupRepo.GetByIDLite(ctx, groupID); err != nil {
		return PublicTransitGroupCacheSetting{}, err
	}
	value, err := json.Marshal(setting)
	if err != nil {
		return PublicTransitGroupCacheSetting{}, err
	}
	if err := s.settingService.settingRepo.Set(ctx, publicTransitGroupCacheSettingKey(groupID), string(value)); err != nil {
		return PublicTransitGroupCacheSetting{}, fmt.Errorf("save public group cache floor: %w", err)
	}
	return setting, nil
}

func validatePublicTransitGroupCacheSetting(setting PublicTransitGroupCacheSetting) error {
	if setting.Floor == nil {
		return nil
	}
	floor := *setting.Floor
	if math.IsNaN(floor) || math.IsInf(floor, 0) || floor <= 0 || floor >= 100 {
		return infraerrors.BadRequest("INVALID_CACHE_HIT_RATE_FLOOR", "Cache hit rate floor must be greater than 0 and less than 100")
	}
	if math.Abs(floor*10-math.Round(floor*10)) > 1e-6 {
		return infraerrors.BadRequest("INVALID_CACHE_HIT_RATE_FLOOR", "Cache hit rate floor supports at most one decimal place")
	}
	return nil
}

// publicTransitGroupCacheSettings reads the stored floors of the given groups.
// A missing, unreadable or out-of-range row yields the unset setting.
func (s *PublicTransitService) publicTransitGroupCacheSettings(ctx context.Context, groupIDs []int64) (map[int64]PublicTransitGroupCacheSetting, error) {
	settings := make(map[int64]PublicTransitGroupCacheSetting, len(groupIDs))
	owners := make(map[string]int64, len(groupIDs))
	keys := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		if _, seen := settings[groupID]; seen {
			continue
		}
		key := publicTransitGroupCacheSettingKey(groupID)
		settings[groupID] = PublicTransitGroupCacheSetting{}
		owners[key] = groupID
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return settings, nil
	}
	values, err := s.settingService.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, err
	}
	for key, raw := range values {
		groupID, ok := owners[key]
		if !ok {
			continue
		}
		var setting PublicTransitGroupCacheSetting
		if err := json.Unmarshal([]byte(raw), &setting); err != nil {
			logger.L().Warn("public_transit_group_cache_setting_unreadable", zap.String("key", key), zap.Error(err))
			continue
		}
		if validatePublicTransitGroupCacheSetting(setting) != nil {
			// Validation only fails for a non-nil floor.
			logger.L().Warn("public_transit_group_cache_setting_invalid", zap.String("key", key), zap.Float64("floor", *setting.Floor))
			continue
		}
		settings[groupID] = setting
	}
	return settings, nil
}

// applyPublicTransitCacheFloor lifts one window to the floor. Input and cache
// creation tokens stay untouched; cache read tokens grow to the only count that
// yields exactly the floor, so the rate and the token counts still agree.
func applyPublicTransitCacheFloor(window PublicTransitCacheUsageWindow, setting PublicTransitGroupCacheSetting) PublicTransitCacheUsageWindow {
	if !setting.Enabled || setting.Floor == nil || validatePublicTransitGroupCacheSetting(setting) != nil || window.CacheHitRate >= *setting.Floor {
		return window
	}
	base := window.InputTokens + window.CacheCreationTokens
	if base <= 0 {
		// Without any prompt tokens there is no usage to scale.
		return window
	}
	floor := *setting.Floor
	window.CacheReadTokens = int64(math.Round(floor / (100 - floor) * float64(base)))
	window.CacheHitRate = floor
	return window
}

func applyPublicTransitCacheFloors(usage PublicTransitCacheUsage, setting PublicTransitGroupCacheSetting) PublicTransitCacheUsage {
	usage.Last24h = applyPublicTransitCacheFloor(usage.Last24h, setting)
	usage.Last7d = applyPublicTransitCacheFloor(usage.Last7d, setting)
	usage.Total = applyPublicTransitCacheFloor(usage.Total, setting)
	return usage
}

// publicTransitGroupCacheMonitor resolves the same monitor the public page
// shows for a group: the monitor with matching provider and name first, then
// any monitor with the same name. Later monitors win, as on the public page.
func publicTransitGroupCacheMonitor(group PublicTransitGroup, monitors []PublicTransitMonitor) *PublicTransitGroupCacheMonitor {
	var exact, folded *PublicTransitMonitor
	for i := range monitors {
		item := &monitors[i]
		if item.GroupName == "" {
			continue
		}
		if item.GroupName == group.Name && item.Provider == group.Platform {
			exact = item
		}
		if strings.EqualFold(item.GroupName, group.Name) {
			folded = item
		}
	}
	item := exact
	if item == nil {
		item = folded
	}
	if item == nil {
		return nil
	}
	return &PublicTransitGroupCacheMonitor{Status: item.PrimaryStatus, Model: item.PrimaryModel}
}
