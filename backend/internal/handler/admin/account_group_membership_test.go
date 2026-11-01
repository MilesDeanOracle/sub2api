package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type membershipAdminService struct {
	*stubAdminService
	setAccountID     int64
	setGroupID       int64
	setPriority      int
	setConfirmRisk   bool
	removedAccountID int64
	removedGroupID   int64
}

func (s *membershipAdminService) SetAccountGroupMembership(_ context.Context, accountID, groupID int64, priority int, skipMixedChannelCheck bool) (*service.Account, error) {
	s.setAccountID = accountID
	s.setGroupID = groupID
	s.setPriority = priority
	s.setConfirmRisk = skipMixedChannelCheck
	return &service.Account{ID: accountID, GroupIDs: []int64{groupID}}, nil
}

func (s *membershipAdminService) RemoveAccountGroupMembership(_ context.Context, accountID, groupID int64) (*service.Account, error) {
	s.removedAccountID = accountID
	s.removedGroupID = groupID
	return &service.Account{ID: accountID}, nil
}

func membershipContext(method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "42"}, {Key: "group_id", Value: "7"}}
	return ctx, recorder
}

func TestSetGroupMembershipUsesRelationshipMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &membershipAdminService{stubAdminService: newStubAdminService()}
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, recorder := membershipContext(http.MethodPut, `{"priority":9,"confirm_mixed_channel_risk":true}`)

	handler.SetGroupMembership(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), svc.setAccountID)
	require.Equal(t, int64(7), svc.setGroupID)
	require.Equal(t, 9, svc.setPriority)
	require.True(t, svc.setConfirmRisk)
}

func TestRemoveGroupMembershipUsesRelationshipMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &membershipAdminService{stubAdminService: newStubAdminService()}
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, recorder := membershipContext(http.MethodDelete, "")

	handler.RemoveGroupMembership(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), svc.removedAccountID)
	require.Equal(t, int64(7), svc.removedGroupID)
}
