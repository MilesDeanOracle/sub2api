package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

func registerActivityUserRoutes(authenticated *gin.RouterGroup, h *handler.Handlers, panelRateLimiter *middleware.PanelRateLimiter) {
	activity := authenticated.Group("/activity")
	activity.GET("/status", h.Activity.Status)
	activity.GET("/reward-moments", panelRateLimiter.Heavy(), h.Activity.RewardMoments)
	activity.GET("/reward-sharing", h.Activity.RewardSharing)
	activity.PUT("/reward-sharing", h.Activity.SetRewardSharing)
	activity.GET("/wallet", h.Activity.WalletStatus)
	activity.POST("/wallet/withdraw", h.Activity.WithdrawWallet)
	activity.GET("/consumption", h.Activity.ConsumptionStatus)
	activity.POST("/consumption/claim", h.Activity.ClaimConsumption)
	activity.POST("/checkin", h.Activity.Checkin)
	activity.GET("/lottery", h.Activity.LotteryStatus)
	activity.POST("/lottery/draw", h.Activity.LotteryDraw)
	activity.GET("/lottery/history", h.Activity.LotteryHistory)
	activity.POST("/second-recharge/claim", h.Activity.ClaimSecondRecharge)
}

func registerActivityAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	admin.GET("/activity/audience/:kind", h.Activity.AdminAudience)
	admin.PUT("/activity/audience/:kind", h.Activity.UpdateAdminAudience)
	admin.GET("/activity/health", h.Activity.AdminHealth)
	admin.PUT("/activity/runtime", h.Activity.UpdateAdminRuntime)
	admin.GET("/activity/config", h.Activity.AdminConfig)
	admin.GET("/activity/reward-moments", h.Activity.RewardMomentsConfig)
	admin.PUT("/activity/reward-moments", h.Activity.SetRewardMomentsConfig)
	admin.GET("/activity/consumption/config", h.Activity.ConsumptionConfig)
	admin.PUT("/activity/consumption/config", h.Activity.SetConsumptionEnabled)
	admin.GET("/activity/consumption/campaigns", h.Activity.ConsumptionCampaigns)
	admin.PUT("/activity/consumption/campaigns", h.Activity.SaveConsumptionCampaign)
	admin.POST("/activity/consumption/state", h.Activity.ConsumptionState)
	admin.GET("/activity/consumption/funds/:userID", h.Activity.ConsumptionFunds)
	admin.POST("/activity/consumption/correct", h.Activity.CorrectConsumption)
	admin.POST("/activity/consumption/review", h.Activity.ResolveConsumptionReview)
	admin.GET("/activity/consumption/audit", h.Activity.ConsumptionAudit)
	admin.PUT("/activity/config", h.Activity.UpdateAdminConfig)
	admin.GET("/activity/refund-reviews", h.Activity.AdminRefundReviews)
	admin.POST("/activity/refund-reviews/:orderID/review", h.Activity.MarkRefundReviewed)
	admin.GET("/activity/lottery", h.Activity.AdminLotteryConfig)
	admin.PUT("/activity/lottery", h.Activity.UpdateAdminLotteryConfig)
	admin.GET("/activity/lottery/users/:userID", h.Activity.AdminLotteryUser)
}
