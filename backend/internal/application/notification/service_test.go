package notification

import (
	"context"
	"testing"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domainnotification "askflow/internal/domain/notification"
)

type fakeNotificationRepository struct {
	values  []*domainnotification.Notification
	created bool
}

func (r *fakeNotificationRepository) Create_Notification(_ context.Context, value *domainnotification.Notification) (bool, error) {
	if !r.created {
		value.ID = 9
		r.values = append(r.values, value)
		r.created = true
		return true, nil
	}
	return false, nil
}
func (r *fakeNotificationRepository) Find_NotificationsAfter(context.Context, uint64, uint64, int) ([]*domainnotification.Notification, error) {
	return r.values, nil
}
func (r *fakeNotificationRepository) Count_UnreadNotifications(context.Context, uint64) (int64, error) {
	return int64(len(r.values)), nil
}
func (r *fakeNotificationRepository) Mark_NotificationRead(context.Context, uint64, uint64) error {
	return nil
}
func (r *fakeNotificationRepository) Mark_AllNotificationsRead(context.Context, uint64) error {
	return nil
}

type fakeAnswerRepository struct{ answer *domainanswer.Answer }

func (r fakeAnswerRepository) Create_Answer(context.Context, *domainanswer.Answer) error { return nil }
func (r fakeAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	return r.answer, nil
}
func (r fakeAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return nil, nil
}

func TestCreateFromEventPersistsAndPushesNotification(t *testing.T) {
	repository := &fakeNotificationRepository{}
	hub := NewHub()
	service := NewService(repository, fakeAnswerRepository{answer: &domainanswer.Answer{ID: 2, AuthorID: 10}}, hub)
	events, unsubscribe := hub.Subscribe(10)
	defer unsubscribe()

	err := service.Create_FromEvent(context.Background(), CreateInput{EventID: "event-1", EventType: domainnotification.TypeAnswerLiked,
		ActorID: 20, AnswerID: 2, QuestionID: 3, OccurredAt: time.Now()})
	if err != nil || len(repository.values) != 1 {
		t.Fatalf("创建通知失败: values=%+v err=%v", repository.values, err)
	}
	select {
	case value := <-events:
		if value.ID != 9 || value.Type != domainnotification.TypeAnswerLiked {
			t.Fatalf("推送通知错误: %+v", value)
		}
	case <-time.After(time.Second):
		t.Fatal("未收到实时通知")
	}

	// 相同 event_id 由仓储判重后不能再次实时推送。
	if err := service.Create_FromEvent(context.Background(), CreateInput{EventID: "event-1", EventType: domainnotification.TypeAnswerLiked,
		ActorID: 20, AnswerID: 2, OccurredAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-events:
		t.Fatalf("重复事件不应再次推送: %+v", value)
	default:
	}
}

func TestCreateFromEventSkipsSelfNotification(t *testing.T) {
	repository := &fakeNotificationRepository{}
	service := NewService(repository, fakeAnswerRepository{answer: &domainanswer.Answer{ID: 2, AuthorID: 10}}, NewHub())
	if err := service.Create_FromEvent(context.Background(), CreateInput{EventID: "event-2", EventType: domainnotification.TypeAnswerCommented,
		ActorID: 10, AnswerID: 2, OccurredAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if len(repository.values) != 0 {
		t.Fatal("用户操作自己的回答不应产生通知")
	}
}

func TestHubDisconnectsSlowSubscriber(t *testing.T) {
	hub := NewHub()
	events, unsubscribe := hub.Subscribe(1)
	defer unsubscribe()
	for id := uint64(1); id <= 17; id++ {
		hub.Publish(1, Output{ID: id})
	}
	for range events {
	}
	// 已关闭的慢连接不会影响后续发布，也不会发生向关闭通道发送的 panic。
	hub.Publish(1, Output{ID: 18})
}
