package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domainanswerranking "askflow/internal/domain/answerranking"
	domainuser "askflow/internal/domain/user"

	"golang.org/x/sync/singleflight"
)

const (
	answerRankSize   = 51
	answerRankTTL    = 15 * time.Second
	answerRankJitter = 15 * time.Second
)

type rankedAnswer struct {
	AnswerID     uint64  `json:"answer_id"`
	QualityScore float64 `json:"quality_score"`
}

type answerRankSnapshot struct {
	RankedAt  time.Time      `json:"ranked_at"`
	Answers   []rankedAnswer `json:"answers"`
	Exhausted bool           `json:"exhausted"`
}

// AnswerRankInvalidator 删除问题的基础回答排序快照。
type AnswerRankInvalidator interface {
	Invalidate_AnswerRank(ctx context.Context, questionID uint64)
}

// AnswerRankingRepository 缓存轻量排序结果，并通过回答、统计和用户卡片组装响应。
type AnswerRankingRepository struct {
	next             domainanswerranking.Repository
	answerRepository domainanswer.Repository
	userRepository   domainuser.Repository
	store            Store
	group            singleflight.Group
}

func NewAnswerRankingRepository(next domainanswerranking.Repository, answers domainanswer.Repository, users domainuser.Repository, store Store) *AnswerRankingRepository {
	return &AnswerRankingRepository{next: next, answerRepository: answers, userRepository: users, store: store}
}

func (r *AnswerRankingRepository) Find_AnswerForQuestion(ctx context.Context, questionID, answerID uint64) (*domainanswerranking.Item, error) {
	answer, err := r.answerRepository.Find_AnswerByID(ctx, answerID)
	if errors.Is(err, domainanswer.ErrNotFound) {
		return nil, domainanswerranking.ErrAnchorNotFound
	}
	if err != nil {
		return nil, err
	}
	if answer.QuestionID != questionID {
		return nil, domainanswerranking.ErrAnchorWrongQuestion
	}
	return r.hydrate(ctx, answer, 0, time.Time{})
}

func (r *AnswerRankingRepository) Find_RankedAnswers(ctx context.Context, questionID, excludeAnswerID uint64, asOf time.Time, cursor *domainanswerranking.Cursor, limit int) ([]domainanswerranking.Item, error) {
	snapshot, ok := r.read(ctx, questionID)
	if ok && cursor != nil && !snapshot.RankedAt.Equal(asOf) {
		return r.findNext(ctx, questionID, excludeAnswerID, asOf, cursor, limit)
	}
	if !ok {
		loaded, err := r.load(ctx, questionID, asOf)
		if err != nil {
			return nil, err
		}
		snapshot = loaded
	}
	if cursor != nil && !snapshot.RankedAt.Equal(asOf) {
		return r.findNext(ctx, questionID, excludeAnswerID, asOf, cursor, limit)
	}

	selected := selectRanked(snapshot.Answers, excludeAnswerID, cursor, limit)
	if len(selected) < limit && !snapshot.Exhausted {
		return r.findNext(ctx, questionID, excludeAnswerID, asOf, cursor, limit)
	}
	items := make([]domainanswerranking.Item, 0, len(selected))
	for _, ranked := range selected {
		answer, err := r.answerRepository.Find_AnswerByID(ctx, ranked.AnswerID)
		if err != nil {
			return nil, err
		}
		item, err := r.hydrate(ctx, answer, ranked.QualityScore, snapshot.RankedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

func (r *AnswerRankingRepository) Invalidate_AnswerRank(ctx context.Context, questionID uint64) {
	key := r.key(questionID)
	if err := r.store.Delete(ctx, key); err != nil {
		slog.Warn("删除 Redis 回答排序缓存失败", "key", key, "error", err)
	}
}

func (r *AnswerRankingRepository) load(ctx context.Context, questionID uint64, asOf time.Time) (answerRankSnapshot, error) {
	key := r.key(questionID)
	value, err, shared := r.group.Do(key, func() (any, error) {
		if snapshot, ok := r.read(ctx, questionID); ok {
			return snapshot, nil
		}
		items, err := r.findNext(ctx, questionID, 0, asOf, nil, answerRankSize)
		if err != nil {
			return answerRankSnapshot{}, err
		}
		snapshot := answerRankSnapshot{RankedAt: asOf, Answers: make([]rankedAnswer, 0, len(items)), Exhausted: len(items) < answerRankSize}
		for _, item := range items {
			snapshot.Answers = append(snapshot.Answers, rankedAnswer{AnswerID: item.ID, QualityScore: item.QualityScore})
			r.primeCards(ctx, item)
		}
		r.write(ctx, questionID, snapshot)
		return snapshot, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("answer_rank")
	}
	if err != nil {
		return answerRankSnapshot{}, err
	}
	return value.(answerRankSnapshot), nil
}

func (r *AnswerRankingRepository) findNext(ctx context.Context, questionID, excludeAnswerID uint64, asOf time.Time, cursor *domainanswerranking.Cursor, limit int) ([]domainanswerranking.Item, error) {
	DefaultMetrics.CacheFallback("answer_rank")
	startedAt := time.Now()
	items, err := r.next.Find_RankedAnswers(ctx, questionID, excludeAnswerID, asOf, cursor, limit)
	DefaultMetrics.ObserveLoad("answer_rank", time.Since(startedAt))
	return items, err
}

func (r *AnswerRankingRepository) read(ctx context.Context, questionID uint64) (answerRankSnapshot, bool) {
	key := r.key(questionID)
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 回答排序失败，降级到 MySQL", "key", key, "error", err)
		return answerRankSnapshot{}, false
	}
	if !ok {
		return answerRankSnapshot{}, false
	}
	var snapshot answerRankSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil || snapshot.RankedAt.IsZero() || snapshot.Answers == nil {
		_ = r.store.Delete(ctx, key)
		return answerRankSnapshot{}, false
	}
	return snapshot, true
}

func (r *AnswerRankingRepository) write(ctx context.Context, questionID uint64, snapshot answerRankSnapshot) {
	key := r.key(questionID)
	content, err := json.Marshal(snapshot)
	if err == nil {
		if err := r.store.Set(ctx, key, content, ttlWithJitter(key, answerRankTTL, answerRankJitter)); err != nil {
			slog.Warn("回填 Redis 回答排序失败", "key", key, "error", err)
		}
	}
}

func (r *AnswerRankingRepository) primeCards(ctx context.Context, item domainanswerranking.Item) {
	if cached, ok := r.answerRepository.(*AnswerRepository); ok {
		cached.writeCard(ctx, &domainanswer.Answer{ID: item.ID, QuestionID: item.QuestionID, AuthorID: item.AuthorID, Content: item.Content, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
		cached.writeStats(ctx, item.ID, answerStats{LikeCount: item.LikeCount, CommentCount: item.CommentCount, FavoriteCount: item.FavoriteCount})
	}
}

func (r *AnswerRankingRepository) hydrate(ctx context.Context, answer *domainanswer.Answer, score float64, rankedAt time.Time) (*domainanswerranking.Item, error) {
	author, err := r.userRepository.Find_UserByID(ctx, answer.AuthorID)
	if err != nil {
		return nil, err
	}
	return &domainanswerranking.Item{ID: answer.ID, QuestionID: answer.QuestionID, Content: answer.Content, AuthorID: answer.AuthorID, AuthorName: author.Username, AuthorNick: author.Nickname, AuthorAvatar: author.AvatarURL, LikeCount: answer.LikeCount, CommentCount: answer.CommentCount, FavoriteCount: answer.FavoriteCount, CreatedAt: answer.CreatedAt, UpdatedAt: answer.UpdatedAt, QualityScore: score, RankedAt: rankedAt}, nil
}

func (r *AnswerRankingRepository) key(questionID uint64) string {
	return fmt.Sprintf("askflow:answer:rank:v1:%d", questionID)
}

func selectRanked(answers []rankedAnswer, excludeID uint64, cursor *domainanswerranking.Cursor, limit int) []rankedAnswer {
	selected := make([]rankedAnswer, 0, limit)
	for _, answer := range answers {
		if answer.AnswerID == excludeID || (cursor != nil && cursor.Score > 0 && !afterCursor(answer, cursor)) {
			continue
		}
		selected = append(selected, answer)
		if len(selected) == limit {
			break
		}
	}
	return selected
}

func afterCursor(answer rankedAnswer, cursor *domainanswerranking.Cursor) bool {
	return answer.QualityScore < cursor.Score || (answer.QualityScore == cursor.Score && answer.AnswerID < cursor.AnswerID)
}

var _ domainanswerranking.Repository = (*AnswerRankingRepository)(nil)
