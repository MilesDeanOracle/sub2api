package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type feedbackHandlerRepoStub struct {
	service.FeedbackRepository
	userID       int64
	input        service.FeedbackInput
	replyID      int64
	replyContent string
}

func (r *feedbackHandlerRepoStub) Create(_ context.Context, id int64, in service.FeedbackInput) (int64, error) {
	r.userID = id
	r.input = in
	return 23, nil
}
func (r *feedbackHandlerRepoStub) EnrichNext(context.Context) (bool, error) { return false, nil }
func (r *feedbackHandlerRepoStub) Get(_ context.Context, id, userID int64) (*service.FeedbackTicket, error) {
	r.userID = userID
	return nil, service.ErrFeedbackNotFound
}
func (r *feedbackHandlerRepoStub) List(_ context.Context, f service.FeedbackFilter) (*service.FeedbackList, error) {
	r.userID = f.UserID
	return &service.FeedbackList{Items: []service.FeedbackTicket{}}, nil
}
func (r *feedbackHandlerRepoStub) Summary(_ context.Context, userID int64) (*service.FeedbackSummary, error) {
	r.userID = userID
	return &service.FeedbackSummary{UnreadCount: 1}, nil
}
func (r *feedbackHandlerRepoStub) MarkRead(_ context.Context, id, userID, replyID int64) error {
	r.userID = userID
	r.replyID = replyID
	return nil
}
func (r *feedbackHandlerRepoStub) ReplyMine(_ context.Context, id, userID int64, content string) error {
	r.userID = userID
	r.replyContent = content
	return nil
}

func feedbackTestRouter(t *testing.T, authenticated bool) (*gin.Engine, *feedbackHandlerRepoStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &feedbackHandlerRepoStub{}
	s := service.NewFeedbackService(repo, nil)
	t.Cleanup(s.Stop)
	h := NewFeedbackHandler(s)
	router := gin.New()
	if authenticated {
		router.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			c.Next()
		})
	}
	router.POST("/feedback", h.Create)
	router.GET("/feedback", h.ListMine)
	router.GET("/feedback/summary", h.SummaryMine)
	router.POST("/feedback/:id/read", h.MarkRead)
	router.POST("/feedback/:id/replies", h.ReplyMine)
	router.GET("/feedback/:id", h.GetMine)
	return router, repo
}
func TestFeedbackSubmissionUsesAuthenticatedIdentity(t *testing.T) {
	router, repo := feedbackTestRouter(t, true)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(`{"user_id":999,"title":"test","category":"cooperation","content":"hello","rating":4,"snapshot":{"total_recharged":99999}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	require.EqualValues(t, 7, repo.userID)
	require.Equal(t, "cooperation", repo.input.Category)
}
func TestFeedbackReadCannotOverrideOwnerUsingQuery(t *testing.T) {
	for _, path := range []string{"/feedback?user_id=999", "/feedback/23?user_id=999"} {
		t.Run(path, func(t *testing.T) {
			router, repo := feedbackTestRouter(t, true)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.EqualValues(t, 7, repo.userID)
		})
	}
}
func TestFeedbackRequiresAuthenticatedSubject(t *testing.T) {
	router, _ := feedbackTestRouter(t, false)
	for _, path := range []string{"/feedback", "/feedback/23", "/feedback/summary"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestFeedbackReadReceiptAndSummaryUseAuthenticatedOwner(t *testing.T) {
	router, repo := feedbackTestRouter(t, true)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/feedback/summary?user_id=999", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 7, repo.userID)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback/23/read", strings.NewReader(`{"user_id":999,"reply_id":42}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 7, repo.userID)
	require.EqualValues(t, 42, repo.replyID)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/feedback/23/read", strings.NewReader(`{"reply_id":0}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeedbackUserReplyCannotSpoofAuthorOrResolveTicket(t *testing.T) {
	router, repo := feedbackTestRouter(t, true)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/feedback/23/replies", strings.NewReader(`{"user_id":999,"author_role":"admin","resolved":true,"content":" 继续追问 "}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	require.EqualValues(t, 7, repo.userID)
	require.Equal(t, "继续追问", repo.replyContent)
	unauthenticated, _ := feedbackTestRouter(t, false)
	w = httptest.NewRecorder()
	unauthenticated.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/feedback/23/replies", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
