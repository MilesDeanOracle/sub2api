package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *ActivityHandler) RewardMoments(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	out, err := h.service.RewardMoments(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}
func (h *ActivityHandler) RewardSharing(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	out, err := h.service.RewardSharing(c.Request.Context(), uid)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}
func (h *ActivityHandler) SetRewardSharing(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	var in struct {
		Allowed *bool `json:"allowed"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Allowed == nil {
		response.BadRequest(c, "allowed must be a boolean")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	out, err := h.service.SetRewardSharing(c.Request.Context(), uid, *in.Allowed)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}
func (h *ActivityHandler) RewardMomentsConfig(c *gin.Context) {
	out, err := h.service.RewardMomentsConfig(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, out)
}
func (h *ActivityHandler) SetRewardMomentsConfig(c *gin.Context) {
	uid, ok := activitySubject(c)
	if !ok {
		return
	}
	var in service.RewardMomentsConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "invalid reward moments configuration")
		return
	}
	if err := h.service.UpdateRewardMomentsConfig(c.Request.Context(), uid, in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.RewardMomentsConfig(c)
}
