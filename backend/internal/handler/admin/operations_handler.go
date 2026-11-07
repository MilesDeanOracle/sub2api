package admin

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) GetOperationsBalances(c *gin.Context) {
	// Bound the entire refresh, including deployments with many upstream sites.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	accounts := make([]service.Account, 0)
	for page := 1; ; page++ {
		batch, total, err := h.adminService.ListAccounts(ctx, page, 200, "", "", "", "", 0, "", "id", "asc")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		accounts = append(accounts, batch...)
		if len(batch) == 0 || int64(len(accounts)) >= total {
			break
		}
	}
	response.Success(c, gin.H{"items": h.accountTestService.GetOperationsBalances(ctx, accounts)})
}
