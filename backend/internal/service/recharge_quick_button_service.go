package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rechargequickbutton"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

// 充值页快捷按钮：支付方式网格后面的可配置跳转入口。
// 配置存于 recharge_quick_buttons 表；进程启动时加载进内存，
// 管理端修改后写穿更新，用户侧读取只走内存（多节点部署最迟 TTL 内收敛）。
const (
	// RechargeQuickButtonOpenMethodPopup 弹窗窗口打开
	RechargeQuickButtonOpenMethodPopup = "popup"
	// RechargeQuickButtonOpenMethodNewTab 新标签页打开
	RechargeQuickButtonOpenMethodNewTab = "new_tab"

	// MaxRechargeQuickButtons 最多可配置的快捷按钮数量
	MaxRechargeQuickButtons = 10

	maxRechargeQuickButtonURLLength  = 2048
	maxRechargeQuickButtonIconLength = 300 * 1024 // 与前端 ImageUpload 默认上限一致
	maxRechargeQuickButtonLabelRunes = 50         // 与表字段 VARCHAR(50) 一致

	rechargeQuickButtonCacheTTL   = 60 * time.Second
	rechargeQuickButtonDBTimeout  = 5 * time.Second
	rechargeQuickButtonRefreshKey = "recharge-quick-buttons-reload"
)

// RechargeQuickButton 充值页快捷跳转按钮（内存快照中的领域对象）。
type RechargeQuickButton struct {
	ID         int64     `json:"id"`
	Label      string    `json:"label"`
	IconURL    string    `json:"icon_url"`
	URL        string    `json:"url"`
	OpenMethod string    `json:"open_method"`
	Enabled    bool      `json:"enabled"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UpdateRechargeQuickButton 管理端提交的单个按钮可编辑字段。
type UpdateRechargeQuickButton struct {
	Label      string `json:"label"`
	IconURL    string `json:"icon_url"`
	URL        string `json:"url"`
	OpenMethod string `json:"open_method"`
	Enabled    bool   `json:"enabled"`
	SortOrder  int    `json:"sort_order"`
}

// rechargeQuickButtonSnapshot 不可变快照，整体替换保证并发读安全。
type rechargeQuickButtonSnapshot struct {
	items    []RechargeQuickButton
	loadedAt time.Time
}

// RechargeQuickButtonService 充值页快捷按钮服务（进程内缓存 + 写穿更新）。
type RechargeQuickButtonService struct {
	entClient *dbent.Client
	cache     atomic.Pointer[rechargeQuickButtonSnapshot]
	refreshSF singleflight.Group
}

// NewRechargeQuickButtonService creates a RechargeQuickButtonService.
func NewRechargeQuickButtonService(entClient *dbent.Client) *RechargeQuickButtonService {
	return &RechargeQuickButtonService{entClient: entClient}
}

// WarmCache 启动时同步加载全部按钮到内存缓存。
func (s *RechargeQuickButtonService) WarmCache(ctx context.Context) error {
	_, err, _ := s.refresh(ctx)
	return err
}

func (s *RechargeQuickButtonService) refresh(ctx context.Context) (any, error, bool) {
	return s.refreshSF.Do(rechargeQuickButtonRefreshKey, func() (any, error) {
		return nil, s.reload(ctx)
	})
}

func (s *RechargeQuickButtonService) reload(ctx context.Context) error {
	rows, err := s.entClient.RechargeQuickButton.Query().
		Order(rechargequickbutton.BySortOrder(), rechargequickbutton.ByID()).
		All(ctx)
	if err != nil {
		return err
	}
	items := make([]RechargeQuickButton, 0, len(rows))
	for _, r := range rows {
		items = append(items, RechargeQuickButton{
			ID:         r.ID,
			Label:      r.Label,
			IconURL:    r.IconURL,
			URL:        r.URL,
			OpenMethod: r.OpenMethod,
			Enabled:    r.Enabled,
			SortOrder:  r.SortOrder,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		})
	}
	s.cache.Store(&rechargeQuickButtonSnapshot{items: items, loadedAt: time.Now()})
	return nil
}

// loadSnapshot 读取内存快照；未预热时同步兜底加载一次，
// 快照超过 TTL 时异步刷新（多节点收敛），读路径永不阻塞在数据库上。
func (s *RechargeQuickButtonService) loadSnapshot() []RechargeQuickButton {
	if snap := s.cache.Load(); snap != nil {
		if time.Since(snap.loadedAt) > rechargeQuickButtonCacheTTL {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), rechargeQuickButtonDBTimeout)
				defer cancel()
				_, _, _ = s.refresh(ctx)
			}()
		}
		return snap.items
	}
	// 理论上启动即预热；此处兜底，失败则按无按钮处理
	ctx, cancel := context.WithTimeout(context.Background(), rechargeQuickButtonDBTimeout)
	defer cancel()
	_, _, _ = s.refresh(ctx)
	if snap := s.cache.Load(); snap != nil {
		return snap.items
	}
	return nil
}

// ListAll 返回全部按钮（管理端视图，读内存）。
func (s *RechargeQuickButtonService) ListAll(ctx context.Context) []RechargeQuickButton {
	items := s.loadSnapshot()
	out := make([]RechargeQuickButton, len(items))
	copy(out, items)
	return out
}

// ListEnabled 返回启用的按钮（充值页视图，读内存）。
func (s *RechargeQuickButtonService) ListEnabled(ctx context.Context) []RechargeQuickButton {
	items := s.loadSnapshot()
	out := make([]RechargeQuickButton, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			out = append(out, item)
		}
	}
	return out
}

// ReplaceAll 校验并整体替换全部按钮，随后写穿更新内存快照（充值页下一个请求即生效）。
func (s *RechargeQuickButtonService) ReplaceAll(ctx context.Context, req []UpdateRechargeQuickButton) ([]RechargeQuickButton, error) {
	normalized, err := normalizeRechargeQuickButtons(req)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }() // 提交后为 no-op

	if _, err := tx.RechargeQuickButton.Delete().Exec(ctx); err != nil {
		return nil, err
	}
	if len(normalized) > 0 {
		creators := make([]*dbent.RechargeQuickButtonCreate, 0, len(normalized))
		for _, item := range normalized {
			creators = append(creators, tx.RechargeQuickButton.Create().
				SetLabel(item.Label).
				SetIconURL(item.IconURL).
				SetURL(item.URL).
				SetOpenMethod(item.OpenMethod).
				SetEnabled(item.Enabled).
				SetSortOrder(item.SortOrder))
		}
		if err := tx.RechargeQuickButton.CreateBulk(creators...).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 写穿：落库成功后立即重建内存快照
	if err := s.reload(ctx); err != nil {
		return nil, fmt.Errorf("快捷按钮已保存，但刷新内存缓存失败: %w", err)
	}
	return s.ListAll(ctx), nil
}

// normalizeRechargeQuickButtons 校验并归一化提交的按钮列表（sort_order 按提交顺序重排）。
func normalizeRechargeQuickButtons(req []UpdateRechargeQuickButton) ([]UpdateRechargeQuickButton, error) {
	if len(req) > MaxRechargeQuickButtons {
		return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("快捷按钮最多配置 %d 个", MaxRechargeQuickButtons))
	}
	items := make([]UpdateRechargeQuickButton, 0, len(req))
	for i, r := range req {
		item := UpdateRechargeQuickButton{
			Label:      strings.TrimSpace(r.Label),
			IconURL:    strings.TrimSpace(r.IconURL),
			URL:        strings.TrimSpace(r.URL),
			OpenMethod: strings.TrimSpace(r.OpenMethod),
			Enabled:    r.Enabled,
			SortOrder:  i,
		}
		if item.URL == "" {
			return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的跳转地址不能为空", i+1))
		}
		if len(item.URL) > maxRechargeQuickButtonURLLength {
			return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的跳转地址过长", i+1))
		}
		if u, err := url.Parse(item.URL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的跳转地址必须是合法的 http(s) 链接", i+1))
		}
		switch item.OpenMethod {
		case "":
			item.OpenMethod = RechargeQuickButtonOpenMethodNewTab
		case RechargeQuickButtonOpenMethodPopup, RechargeQuickButtonOpenMethodNewTab:
		default:
			return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的打开方式无效", i+1))
		}
		if utf8.RuneCountInString(item.Label) > maxRechargeQuickButtonLabelRunes {
			return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的名称过长（最多 %d 字）", i+1, maxRechargeQuickButtonLabelRunes))
		}
		if item.IconURL != "" {
			if len(item.IconURL) > maxRechargeQuickButtonIconLength {
				return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的图标过大（最多 %d KB）", i+1, maxRechargeQuickButtonIconLength/1024))
			}
			if strings.HasPrefix(item.IconURL, "data:image/") {
				if _, _, err := parseDataImageURL(item.IconURL); err != nil {
					return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的图标格式无效: %v", i+1, err))
				}
			} else if u, err := url.Parse(item.IconURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, infraerrors.BadRequest("RECHARGE_QUICK_BUTTON_INVALID", fmt.Sprintf("第 %d 个按钮的图标必须是图片上传结果或 http(s) 链接", i+1))
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// parseDataImageURL 粗校验 data:image/... 格式（mime;base64 或 mime,param）。
func parseDataImageURL(raw string) (mime string, data string, err error) {
	idx := strings.Index(raw, ",")
	if idx < 0 {
		return "", "", fmt.Errorf("缺少数据段")
	}
	header := raw[len("data:image/"):idx]
	meta := strings.SplitN(header, ";", 2)
	mime = "image/" + strings.TrimSpace(meta[0])
	if mime == "image/" {
		return "", "", fmt.Errorf("缺少 MIME 类型")
	}
	return mime, raw[idx+1:], nil
}
