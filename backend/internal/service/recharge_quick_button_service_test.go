//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRechargeQuickButtons(t *testing.T) {
	t.Run("normalizes sort order and defaults open method", func(t *testing.T) {
		items, err := normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{
			{Label: " 客服充值 ", URL: "https://example.com/recharge", OpenMethod: "", Enabled: true},
			{Label: "代充", URL: "https://example.com/agent", OpenMethod: RechargeQuickButtonOpenMethodPopup, Enabled: false},
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
		require.Equal(t, "客服充值", items[0].Label)
		require.Equal(t, RechargeQuickButtonOpenMethodNewTab, items[0].OpenMethod)
		require.Equal(t, 0, items[0].SortOrder)
		require.Equal(t, 1, items[1].SortOrder)
	})

	t.Run("accepts data image and https icons", func(t *testing.T) {
		items, err := normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{
			{URL: "https://example.com", IconURL: "data:image/svg+xml;base64,PHN2Zy8+", OpenMethod: RechargeQuickButtonOpenMethodPopup},
			{URL: "https://example.com/2", IconURL: "https://cdn.example.com/icon.png"},
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
	})

	t.Run("rejects invalid url scheme and empty url", func(t *testing.T) {
		for _, req := range []UpdateRechargeQuickButton{
			{URL: "javascript:alert(1)"},
			{URL: ""},
			{URL: "example.com/no-scheme"},
		} {
			_, err := normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{req})
			require.Error(t, err, "url=%q should be rejected", req.URL)
		}
	})

	t.Run("rejects invalid open method and icon", func(t *testing.T) {
		_, err := normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{{URL: "https://example.com", OpenMethod: "iframe"}})
		require.Error(t, err)

		_, err = normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{{URL: "https://example.com", IconURL: "data:text/html;base64,PGI+"}})
		require.Error(t, err)

		_, err = normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{{URL: "https://example.com", IconURL: strings.Repeat("a", maxRechargeQuickButtonIconLength+1)}})
		require.Error(t, err)
	})

	t.Run("rejects oversized list and long label", func(t *testing.T) {
		tooMany := make([]UpdateRechargeQuickButton, MaxRechargeQuickButtons+1)
		for i := range tooMany {
			tooMany[i] = UpdateRechargeQuickButton{URL: "https://example.com"}
		}
		_, err := normalizeRechargeQuickButtons(tooMany)
		require.Error(t, err)

		_, err = normalizeRechargeQuickButtons([]UpdateRechargeQuickButton{{URL: "https://example.com", Label: strings.Repeat("长", maxRechargeQuickButtonLabelRunes+1)}})
		require.Error(t, err)
	})
}
