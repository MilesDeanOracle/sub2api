package handler

import (
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ActivityHandler struct{ service *service.ActivityService }

func NewActivityHandler(s *service.ActivityService) *ActivityHandler {
	return &ActivityHandler{service: s}
}

func (h *ActivityHandler) Status(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.Status(c.Request.Context(), subject.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *ActivityHandler) Checkin(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.Checkin(c.Request.Context(), subject.UserID)
	if activityError(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *ActivityHandler) WalletStatus(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	wallet, err := h.service.WalletStatus(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
}

func (h *ActivityHandler) WithdrawWallet(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	wallet, err := h.service.WithdrawWallet(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, service.ErrActivityWalletEmpty) || errors.Is(err, service.ErrActivityReviewBlocked) {
			response.BadRequest(c, err.Error())
			return
		}
		if activityError(c, err) {
			return
		}
		return
	}
	response.Success(c, wallet)
}

func (h *ActivityHandler) ClaimSecondRecharge(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	credited, err := h.service.ClaimSecondRecharge(c.Request.Context(), subject.UserID)
	if activityError(c, err) {
		return
	}
	response.Success(c, gin.H{"credited": credited})
}

func (h *ActivityHandler) AdminConfig(c *gin.Context) {
	config, err := h.service.Config(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, config)
}

func (h *ActivityHandler) UpdateAdminConfig(c *gin.Context) {
	var config service.ActivityConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "Invalid activity configuration")
		return
	}
	if err := h.service.UpdateConfig(c.Request.Context(), config); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, config)
}

func (h *ActivityHandler) AdminRefundReviews(c *gin.Context) {
	reviews, err := h.service.RefundReviews(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, reviews)
}

func (h *ActivityHandler) MarkRefundReviewed(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("orderID"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid order ID")
		return
	}
	if err := h.service.MarkRefundReviewed(c.Request.Context(), id, actor); err != nil {
		if errors.Is(err, service.ErrActivityReviewBlocked) || errors.Is(err, service.ErrActivityRecoveryInsufficient) || errors.Is(err, service.ErrActivityRecoveryReview) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"reviewed": true})
}

func activityError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrActivityReviewBlocked) {
		response.BadRequest(c, err.Error())
		return true
	}
	if errors.Is(err, service.ErrActivityNoReward) {
		response.BadRequest(c, err.Error())
		return true
	}
	if errors.Is(err, service.ErrActivityDisabled) || errors.Is(err, service.ErrActivityIneligible) {
		response.Forbidden(c, err.Error())
		return true
	}
	return response.ErrorFrom(c, err)
}

func activitySubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func (h *ActivityHandler) ConsumptionStatus(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	selected, err := strconv.ParseInt(c.DefaultQuery("campaign_id", "0"), 10, 64)
	if err != nil || selected < 0 {
		response.BadRequest(c, "Invalid campaign ID")
		return
	}
	status, err := h.service.ConsumptionStatus(c.Request.Context(), uid, selected)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *ActivityHandler) ClaimConsumption(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		CampaignID int64 `json:"campaign_id"`
		Position   int   `json:"position"`
	}
	if c.ShouldBindJSON(&body) != nil || body.CampaignID <= 0 || body.Position < 0 || body.Position > 5 {
		response.BadRequest(c, "Invalid reward claim")
		return
	}
	if err := h.service.ClaimConsumption(c.Request.Context(), uid, body.CampaignID, body.Position); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	status, err := h.service.ConsumptionStatus(c.Request.Context(), uid, body.CampaignID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *ActivityHandler) ConsumptionConfig(c *gin.Context) {
	out, err := h.service.ConsumptionConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}

func (h *ActivityHandler) SetConsumptionEnabled(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if c.ShouldBindJSON(&body) != nil || body.Enabled == nil {
		response.BadRequest(c, "enabled must be a boolean")
		return
	}
	out, err := h.service.SetConsumptionEnabled(c.Request.Context(), actor, *body.Enabled)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}

func (h *ActivityHandler) ConsumptionCampaigns(c *gin.Context) {
	items, err := h.service.ConsumptionCampaigns(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
}

func (h *ActivityHandler) SaveConsumptionCampaign(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body service.ConsumptionCampaign
	if c.ShouldBindJSON(&body) != nil {
		response.BadRequest(c, "Invalid campaign configuration")
		return
	}
	id, err := h.service.SaveConsumptionCampaign(c.Request.Context(), actor, body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *ActivityHandler) ConsumptionState(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		ID    int64  `json:"id"`
		State string `json:"state"`
	}
	if c.ShouldBindJSON(&body) != nil || body.ID <= 0 {
		response.BadRequest(c, "Invalid campaign")
		return
	}
	if err := h.service.SetConsumptionState(c.Request.Context(), actor, body.ID, body.State); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

func (h *ActivityHandler) ConsumptionFunds(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || uid <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	status, err := h.service.ConsumptionStatus(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	wallet, err := h.service.WalletStatus(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	ledger, err := h.service.ConsumptionUsageLedger(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"status": status, "wallet": wallet, "ledger": ledger})
}

func (h *ActivityHandler) CorrectConsumption(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		LedgerID int64  `json:"ledger_id"`
		Note     string `json:"note"`
	}
	if c.ShouldBindJSON(&body) != nil || body.LedgerID <= 0 {
		response.BadRequest(c, "Invalid settlement")
		return
	}
	if err := h.service.CorrectConsumption(c.Request.Context(), actor, body.LedgerID, body.Note); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

func (h *ActivityHandler) ResolveConsumptionReview(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		UserID  int64  `json:"user_id"`
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if c.ShouldBindJSON(&body) != nil || body.UserID <= 0 {
		response.BadRequest(c, "Invalid review")
		return
	}
	if err := h.service.ResolveConsumptionReview(c.Request.Context(), actor, body.UserID, body.Approve, body.Note); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

func (h *ActivityHandler) ConsumptionAudit(c *gin.Context) {
	items, err := h.service.ConsumptionAudit(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
}

func (h *ActivityHandler) AdminHealth(c *gin.Context) {
	v, err := h.service.Health(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, v)
}

func (h *ActivityHandler) UpdateAdminRuntime(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		ProcessingEnabled bool `json:"processing_enabled"`
		AwardsEnabled     bool `json:"awards_enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid runtime configuration")
		return
	}
	health, err := h.service.UpdateRuntime(c.Request.Context(), actor, body.ProcessingEnabled, body.AwardsEnabled)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, health)
}

func (h *ActivityHandler) AdminAudience(c *gin.Context) {
	out, err := h.service.Audience(c.Request.Context(), c.Param("kind"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}

func (h *ActivityHandler) UpdateAdminAudience(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body service.ActivityAudience
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid audience configuration")
		return
	}
	if err := h.service.UpdateAudience(c.Request.Context(), actor, c.Param("kind"), body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

func (h *ActivityHandler) LotteryStatus(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	status, err := h.service.LotteryStatus(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, status)
}

func (h *ActivityHandler) LotteryDraw(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	var body struct {
		Count          int    `json:"count"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, service.ErrLotteryInvalidDraw.Error())
		return
	}
	res, err := h.service.DrawLottery(c.Request.Context(), uid, body.Count, body.IdempotencyKey)
	if err != nil {
		if errors.Is(err, service.ErrLotteryInvalidDraw) ||
			errors.Is(err, service.ErrLotteryInsufficientTickets) ||
			errors.Is(err, service.ErrLotteryBudgetInsufficient) ||
			errors.Is(err, service.ErrLotteryBatchBudgetInsufficient) ||
			errors.Is(err, service.ErrActivityReviewBlocked) {
			response.BadRequest(c, err.Error())
			return
		}
		if activityError(c, err) {
			return
		}
		return
	}
	response.Success(c, res)
}

func (h *ActivityHandler) LotteryHistory(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	res, err := h.service.LotteryHistory(c.Request.Context(), uid, page, pageSize)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, res)
}

func (h *ActivityHandler) AdminLotteryConfig(c *gin.Context) {
	cfg, err := h.service.AdminLotteryConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, cfg)
}

func (h *ActivityHandler) UpdateAdminLotteryConfig(c *gin.Context) {
	actor, ok := activitySubject(c)
	if !ok {
		return
	}
	var body service.AdminLotteryConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid lottery configuration")
		return
	}
	cfg, err := h.service.UpdateAdminLotteryConfig(c.Request.Context(), actor, body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, cfg)
}

func (h *ActivityHandler) AdminLotteryUser(c *gin.Context) {
	uid, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || uid <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	res, err := h.service.AdminLotteryUser(c.Request.Context(), uid)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}
