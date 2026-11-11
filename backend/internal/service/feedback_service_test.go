package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type feedbackServiceRepoStub struct {
	FeedbackRepository
	create    func(context.Context, int64, FeedbackInput) (int64, error)
	enrich    func(context.Context) (bool, error)
	replyMine func(context.Context, int64, int64, string) error
}

func (r *feedbackServiceRepoStub) Create(ctx context.Context, id int64, in FeedbackInput) (int64, error) {
	return r.create(ctx, id, in)
}
func (r *feedbackServiceRepoStub) EnrichNext(ctx context.Context) (bool, error) { return r.enrich(ctx) }
func (r *feedbackServiceRepoStub) ReplyMine(ctx context.Context, id, userID int64, content string) error {
	return r.replyMine(ctx, id, userID, content)
}

func TestFeedbackUserReplyValidatesAndTrimsContent(t *testing.T) {
	calls := 0
	s := &FeedbackService{repo: &feedbackServiceRepoStub{replyMine: func(_ context.Context, id, userID int64, content string) error {
		calls++
		require.EqualValues(t, 23, id)
		require.EqualValues(t, 7, userID)
		require.Equal(t, "继续追问", content)
		return ErrFeedbackAwaitingAdmin
	}}}
	for _, content := range []string{"", " \n\t", strings.Repeat("中", 10001)} {
		require.Error(t, s.ReplyMine(context.Background(), 23, 7, content))
	}
	require.Zero(t, calls)
	require.ErrorIs(t, s.ReplyMine(context.Background(), 23, 7, " 继续追问 \n"), ErrFeedbackAwaitingAdmin)
	require.Equal(t, 1, calls)
}

func TestFeedbackSubmissionDoesNotWaitForEnrichment(t *testing.T) {
	entered := make(chan struct{}, 1)
	repo := &feedbackServiceRepoStub{
		create: func(_ context.Context, id int64, in FeedbackInput) (int64, error) {
			require.EqualValues(t, 7, id)
			require.Equal(t, "标题", in.Title)
			require.Equal(t, "内容", in.Content)
			return 12, nil
		},
		enrich: func(ctx context.Context) (bool, error) {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return false, ctx.Err()
		},
	}
	s := NewFeedbackService(repo, nil)
	defer s.Stop()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("background worker did not start")
	}
	result := make(chan int64, 1)
	go func() {
		id, _ := s.Create(context.Background(), 7, FeedbackInput{Title: " 标题 ", Category: "usage", Content: " 内容 ", Rating: 5})
		result <- id
	}()
	select {
	case id := <-result:
		require.EqualValues(t, 12, id)
	case <-time.After(time.Second):
		t.Fatal("submission waited for background enrichment")
	}
}

func TestFeedbackRejectsInvalidSubmissionBeforeSaving(t *testing.T) {
	s := &FeedbackService{repo: &feedbackServiceRepoStub{create: func(context.Context, int64, FeedbackInput) (int64, error) {
		t.Fatal("invalid feedback reached database")
		return 0, nil
	}}}
	valid := FeedbackInput{Title: "正常标题", Category: "suggestion", Content: "内容", Rating: 1}
	tests := []struct {
		name   string
		change func(*FeedbackInput)
	}{
		{"blank title", func(in *FeedbackInput) { in.Title = " \n " }},
		{"long title", func(in *FeedbackInput) { in.Title = strings.Repeat("中", 121) }},
		{"blank content", func(in *FeedbackInput) { in.Content = "\t" }},
		{"long content", func(in *FeedbackInput) { in.Content = strings.Repeat("中", 10001) }},
		{"unknown category", func(in *FeedbackInput) { in.Category = "admin" }},
		{"no rating", func(in *FeedbackInput) { in.Rating = 0 }},
		{"high rating", func(in *FeedbackInput) { in.Rating = 6 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid
			tt.change(&in)
			_, err := s.Create(context.Background(), 1, in)
			require.Error(t, err)
		})
	}
}

func TestFeedbackDoesNotAcknowledgeFailedPersistence(t *testing.T) {
	failure := errors.New("database unavailable")
	s := &FeedbackService{repo: &feedbackServiceRepoStub{create: func(context.Context, int64, FeedbackInput) (int64, error) { return 0, failure }}}
	_, err := s.Create(context.Background(), 1, FeedbackInput{Title: "t", Category: "usage", Content: "c", Rating: 3})
	require.ErrorIs(t, err, failure)
}

type feedbackSettingsStub struct {
	SettingRepository
	value   string
	missing bool
	err     error
}

func (r *feedbackSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if r.missing {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}
func (r *feedbackSettingsStub) Set(_ context.Context, key, value string) error {
	r.value = value
	r.missing = false
	return r.err
}
func TestFeedbackSettingsDefaultsBlankAndValidation(t *testing.T) {
	repo := &feedbackSettingsStub{missing: true}
	s := &FeedbackService{settings: repo}
	settings, err := s.GetSettings(context.Background())
	require.NoError(t, err)
	require.Nil(t, settings.ContextNotice)
	for _, value := range []string{"  自定义反馈提示  ", ""} {
		err = s.UpdateSettings(context.Background(), FeedbackSettings{ContextNotice: &value})
		require.NoError(t, err)
		settings, err = s.GetSettings(context.Background())
		require.NoError(t, err)
		require.NotNil(t, settings.ContextNotice)
		require.Equal(t, strings.TrimSpace(value), *settings.ContextNotice)
	}
	long := strings.Repeat("中", 2001)
	require.Error(t, s.UpdateSettings(context.Background(), FeedbackSettings{ContextNotice: &long}))
	require.Error(t, s.UpdateSettings(context.Background(), FeedbackSettings{}))
	repo.err = errors.New("database unavailable")
	_, err = s.GetSettings(context.Background())
	require.Error(t, err)
}
