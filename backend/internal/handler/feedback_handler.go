package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct{ service *service.FeedbackService }

func NewFeedbackHandler(s *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{service: s}
}

func (h *FeedbackHandler) Create(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var in service.FeedbackInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid feedback")
		return
	}
	id, err := h.service.Create(c.Request.Context(), subject.UserID, in)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{"id": id})
}

func (h *FeedbackHandler) ListMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	h.list(c, subject.UserID)
}
func (h *FeedbackHandler) ListAdmin(c *gin.Context) { h.list(c, 0) }
func (h *FeedbackHandler) list(c *gin.Context, userID int64) {
	page, size := response.ParsePagination(c)
	result, err := h.service.List(c.Request.Context(), service.FeedbackFilter{UserID: userID, Category: c.Query("category"), Status: c.Query("status"), Page: page, PageSize: size, UnreadOnly: c.Query("unread_only") == "true"})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}
func (h *FeedbackHandler) GetMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	h.get(c, subject.UserID)
}
func (h *FeedbackHandler) GetAdmin(c *gin.Context) { h.get(c, 0) }
func feedbackID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return 0, false
	}
	return id, true
}
func (h *FeedbackHandler) get(c *gin.Context, userID int64) {
	id, ok := feedbackID(c)
	if !ok {
		return
	}
	ticket, err := h.service.Get(c.Request.Context(), id, userID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, ticket)
}
func (h *FeedbackHandler) Reply(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var in struct {
		Content  string `json:"content"`
		Resolved bool   `json:"resolved"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid reply")
		return
	}
	if response.ErrorFrom(c, h.service.Reply(c.Request.Context(), id, subject.UserID, in.Content, in.Resolved)) {
		return
	}
	response.Created(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) SummaryMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	h.summary(c, subject.UserID)
}

func (h *FeedbackHandler) ReplyMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var in struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid reply")
		return
	}
	if !response.ErrorFrom(c, h.service.ReplyMine(c.Request.Context(), id, subject.UserID, in.Content)) {
		response.Created(c, gin.H{"message": "ok"})
	}
}
func (h *FeedbackHandler) SummaryAdmin(c *gin.Context) { h.summary(c, 0) }
func (h *FeedbackHandler) summary(c *gin.Context, userID int64) {
	summary, err := h.service.Summary(c.Request.Context(), userID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, summary)
	}
}
func (h *FeedbackHandler) MarkRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var in struct {
		ReplyID int64 `json:"reply_id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid read receipt")
		return
	}
	if !response.ErrorFrom(c, h.service.MarkRead(c.Request.Context(), id, subject.UserID, in.ReplyID)) {
		response.Success(c, gin.H{"message": "ok"})
	}
}
func (h *FeedbackHandler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, settings)
	}
}
func (h *FeedbackHandler) UpdateSettings(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var in service.FeedbackSettings
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid feedback settings")
		return
	}
	if !response.ErrorFrom(c, h.service.UpdateSettings(c.Request.Context(), in)) {
		response.Success(c, gin.H{"message": "ok"})
	}
}
