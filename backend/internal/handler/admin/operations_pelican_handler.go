package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OperationsPelicanHandler struct {
	svc  *service.OperationsPelicanService
	repo service.OperationsRepository
}

func NewOperationsPelicanHandler(svc *service.OperationsPelicanService, repo service.OperationsRepository) *OperationsPelicanHandler {
	return &OperationsPelicanHandler{svc: svc, repo: repo}
}

// Guard supplements the parent admin-auth middleware: restricted admin keys
// have no operations-wide artifact or account permission by default.
func (h *OperationsPelicanHandler) Guard(c *gin.Context) {
	if adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "Operations requires full administrator access")
		c.Abort()
		return
	}
	c.Next()
}
func operationsReply(c *gin.Context, v any, e error) {
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}
func operationsID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "Invalid resource ID")
		return 0, false
	}
	return id, true
}
func operationsActor(c *gin.Context) int64 {
	a, _ := middleware.GetAuthSubjectFromContext(c)
	return a.UserID
}
func (h *OperationsPelicanHandler) Accounts(c *gin.Context) {
	v, e := h.svc.Accounts(c.Request.Context())
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Models(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	v, e := h.svc.Models(c.Request.Context(), id, c.Query("refresh") == "true")
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) StorageSources(c *gin.Context) {
	response.Success(c, h.svc.StorageSources(c.Request.Context()))
}
func (h *OperationsPelicanHandler) TestStorage(c *gin.Context) {
	operationsReply(c, gin.H{"ok": true}, h.svc.TestStorage(c.Request.Context(), c.Param("source")))
}
func (h *OperationsPelicanHandler) Tasks(c *gin.Context) {
	v, e := h.repo.ListTasks(c.Request.Context())
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Save(c *gin.Context) {
	var t service.OperationsTask
	if e := c.ShouldBindJSON(&t); e != nil {
		response.BadRequest(c, "Invalid task configuration")
		return
	}
	t.ID = 0
	if c.Param("id") != "" {
		id, ok := operationsID(c)
		if !ok {
			return
		}
		t.ID = id
	}
	t.CreatedBy = operationsActor(c)
	t.UpdatedBy = t.CreatedBy
	v, e := h.svc.SaveTask(c.Request.Context(), &t)
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Enable(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	var in struct {
		Enabled       bool  `json:"enabled"`
		ConfigVersion int64 `json:"config_version"`
	}
	if e := c.ShouldBindJSON(&in); e != nil {
		response.BadRequest(c, "Invalid task state")
		return
	}
	v, e := h.svc.Enable(c.Request.Context(), id, in.Enabled, in.ConfigVersion, operationsActor(c))
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Delete(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	operationsReply(c, gin.H{"ok": true}, h.repo.DeleteTask(c.Request.Context(), id))
}
func (h *OperationsPelicanHandler) Trigger(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	var in struct {
		RequestID string `json:"request_id"`
	}
	if e := c.ShouldBindJSON(&in); e != nil {
		response.BadRequest(c, "Request ID required")
		return
	}
	v, e := h.svc.Trigger(c.Request.Context(), id, in.RequestID, operationsActor(c), nil)
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Run(c *gin.Context) {
	v, e := h.repo.RunResults(c.Request.Context(), c.Param("runId"))
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Status(c *gin.Context) {
	v, e := h.repo.Status(c.Request.Context())
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Filters(c *gin.Context) {
	v, e := h.svc.Filters(c.Request.Context())
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Groups(c *gin.Context) {
	v, e := h.repo.WallGroups(c.Request.Context())
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) CleanGroup(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if e != nil || id == 0 || id < -1 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	v, e := h.svc.CleanGroup(c.Request.Context(), id)
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Results(c *gin.Context) {
	parse := func(key string) int64 { n, _ := strconv.ParseInt(c.Query(key), 10, 64); return n }
	f := service.OperationsResultFilter{History: c.Query("view") == "history", GroupID: parse("group_id"), AccountID: parse("account_id"), TaskID: parse("task_id"), Status: c.Query("status"), BeforeID: parse("before_id"), Limit: int(parse("limit"))}
	f.GroupByAccount = c.Query("group_by") == "account"
	f.BeforeAccountID = parse("before_account_id")
	f.Scope = c.Query("scope")
	if f.Scope != "" && f.Scope != "current" && f.Scope != "past" {
		response.BadRequest(c, "Invalid group scope")
		return
	}
	for _, v := range []struct {
		key string
		dst **time.Time
	}{{"from", &f.From}, {"to", &f.To}} {
		if value := c.Query(v.key); value != "" {
			date, e := time.Parse(time.RFC3339, value)
			if e != nil {
				response.BadRequest(c, "Invalid date filter")
				return
			}
			*v.dst = &date
		}
	}
	out, e := h.repo.ListResults(c.Request.Context(), f)
	operationsReply(c, out, e)
}
func (h *OperationsPelicanHandler) Result(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	v, e := h.repo.GetResult(c.Request.Context(), id)
	if e == nil && !v.ExpiresAt.After(time.Now()) {
		e = service.ErrOperationsNotFound
	}
	operationsReply(c, v, e)
}
func (h *OperationsPelicanHandler) Content(c *gin.Context) {
	id, ok := operationsID(c)
	if !ok {
		return
	}
	v, e := h.svc.Content(c.Request.Context(), id)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	operationsReply(c, gin.H{"html": v}, e)
}
