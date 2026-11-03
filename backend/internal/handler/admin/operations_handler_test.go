package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsAdminStub struct {
	service.AdminService
	pages []int
}

func (s *operationsAdminStub) ListAccounts(_ context.Context, page, size int, platform, kind, status, search string, groupID int64, privacy, sortBy, order string) ([]service.Account, int64, error) {
	s.pages = append(s.pages, page)
	count := size
	if page == 2 {
		count = 1
	}
	items := make([]service.Account, count)
	for i := range items {
		items[i] = service.Account{ID: int64((page-1)*size + i + 1), Credentials: map[string]any{"base_url": "https://example.com/v1", "api_key": "secret"}}
	}
	return items, 201, nil
}

func TestOperationsBalancesLoadsAllAccountPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admin := &operationsAdminStub{}
	h := &AccountHandler{adminService: admin}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/accounts/operations-balances", nil)
	h.GetOperationsBalances(c)
	require.Equal(t, 200, w.Code)
	require.Equal(t, []int{1, 2}, admin.pages)
	var response struct {
		Data struct {
			Items []service.OperationsBalance `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, 201, response.Data.Items[0].AccountCount)
	require.Nil(t, response.Data.Items[0].Balance)
	require.Equal(t, "transport_unavailable", response.Data.Items[0].Error)
	require.NotContains(t, w.Body.String(), "secret")
}
