package repository

import (
	"context"
	"time"

	domainfeed "askflow/internal/domain/feed"

	"gorm.io/gorm"
)

// FeedRepository 使用 MySQL 查询 Feed 读取投影。
type FeedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{db: db}
}

func (r *FeedRepository) Find_FollowingFeed(ctx context.Context, userID uint64, cursor *domainfeed.Cursor, limit int) ([]domainfeed.FollowingItem, error) {
	query := r.db.WithContext(ctx).Table("(?) AS ranked_answers", r.followingAnswers(userID)).
		Select(`
			ranked_answers.question_id,
			questions.title AS question_title,
			question_authors.id AS question_author_id,
			question_authors.username AS question_author_name,
			question_authors.nickname AS question_author_nick,
			question_authors.avatar_url AS question_author_avatar,
			ranked_answers.id AS answer_id,
			ranked_answers.content AS answer_content,
			answer_authors.id AS answer_author_id,
			answer_authors.username AS answer_author_name,
			answer_authors.nickname AS answer_author_nick,
			answer_authors.avatar_url AS answer_author_avatar,
			ranked_answers.like_count,
			ranked_answers.comment_count,
			ranked_answers.favorite_count,
			ranked_answers.created_at AS answer_created_at`).
		Joins("JOIN questions ON questions.id = ranked_answers.question_id").
		Joins("JOIN users AS question_authors ON question_authors.id = questions.author_id").
		Joins("JOIN users AS answer_authors ON answer_authors.id = ranked_answers.author_id").
		Where("ranked_answers.feed_rank = 1")

	if cursor != nil {
		query = query.Where(
			"ranked_answers.created_at < ? OR (ranked_answers.created_at = ? AND ranked_answers.id < ?)",
			cursor.CreatedAt, cursor.CreatedAt, cursor.AnswerID,
		)
	}

	var items []domainfeed.FollowingItem
	err := query.Order("ranked_answers.created_at DESC").
		Order("ranked_answers.id DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

// Find_FollowingItemsByAnswerIDs 批量补齐关注 Feed 缓存中的轻量 ID 列表，不再执行窗口排序。
func (r *FeedRepository) Find_FollowingItemsByAnswerIDs(ctx context.Context, answerIDs []uint64) ([]domainfeed.FollowingItem, error) {
	if len(answerIDs) == 0 {
		return []domainfeed.FollowingItem{}, nil
	}
	var items []domainfeed.FollowingItem
	err := r.db.WithContext(ctx).Table("answers AS a").
		Select(`
			a.question_id,
			questions.title AS question_title,
			question_authors.id AS question_author_id,
			question_authors.username AS question_author_name,
			question_authors.nickname AS question_author_nick,
			question_authors.avatar_url AS question_author_avatar,
			a.id AS answer_id,
			a.content AS answer_content,
			answer_authors.id AS answer_author_id,
			answer_authors.username AS answer_author_name,
			answer_authors.nickname AS answer_author_nick,
			answer_authors.avatar_url AS answer_author_avatar,
			a.like_count, a.comment_count, a.favorite_count,
			a.created_at AS answer_created_at`).
		Joins("JOIN questions ON questions.id = a.question_id").
		Joins("JOIN users AS question_authors ON question_authors.id = questions.author_id").
		Joins("JOIN users AS answer_authors ON answer_authors.id = a.author_id").
		Where("a.id IN ?", answerIDs).
		Scan(&items).Error
	return items, err
}

func (r *FeedRepository) followingAnswers(userID uint64) *gorm.DB {
	return r.db.Table("answers").
		Select(`answers.*, ROW_NUMBER() OVER (
			PARTITION BY answers.question_id
			ORDER BY answers.created_at DESC, answers.id DESC
		) AS feed_rank`).
		Joins("JOIN user_follows ON user_follows.followee_id = answers.author_id").
		Where("user_follows.follower_id = ?", userID)
}

var _ domainfeed.Repository = (*FeedRepository)(nil)

func (r *FeedRepository) Find_HotFeed(ctx context.Context, since, asOf time.Time, cursor *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	query := `
WITH
answer_metrics AS (
    SELECT question_id, COUNT(*) AS answer_count_24h, MAX(created_at) AS latest_at
    FROM answers WHERE created_at >= ? GROUP BY question_id
),
like_metrics AS (
    SELECT a.question_id, COUNT(*) AS like_count_24h, MAX(l.created_at) AS latest_at
    FROM answer_likes l JOIN answers a ON a.id = l.answer_id
    WHERE l.created_at >= ? GROUP BY a.question_id
),
comment_metrics AS (
    SELECT a.question_id, COUNT(*) AS comment_count_24h, MAX(c.created_at) AS latest_at
    FROM answer_comments c JOIN answers a ON a.id = c.answer_id
    WHERE c.created_at >= ? GROUP BY a.question_id
),
view_metrics AS (
    SELECT question_id, COUNT(*) AS view_count_24h, MAX(created_at) AS latest_at
    FROM question_views WHERE created_at >= ? GROUP BY question_id
),
hot_rows AS (
    SELECT
        q.id AS question_id, q.title AS question_title, q.content AS question_content,
        u.id AS question_author_id, u.username AS question_author_name,
        u.nickname AS question_author_nick, u.avatar_url AS question_author_avatar,
        COALESCE(am.answer_count_24h, 0) AS answer_count_24h,
        COALESCE(lm.like_count_24h, 0) AS like_count_24h,
        COALESCE(cm.comment_count_24h, 0) AS comment_count_24h,
        COALESCE(vm.view_count_24h, 0) AS view_count_24h,
        q.created_at AS question_created_at,
        (
            COALESCE(am.answer_count_24h, 0) * 5 +
            COALESCE(lm.like_count_24h, 0) * 2 +
            COALESCE(cm.comment_count_24h, 0) * 3 +
            COALESCE(vm.view_count_24h, 0) * 0.1
        ) / POW(
            2 + TIMESTAMPDIFF(SECOND, GREATEST(
                COALESCE(am.latest_at, q.created_at), COALESCE(lm.latest_at, q.created_at),
                COALESCE(cm.latest_at, q.created_at), COALESCE(vm.latest_at, q.created_at)
            ), ?) / 86400,
            1.2
        ) AS hot_score
    FROM questions q
    JOIN users u ON u.id = q.author_id
    LEFT JOIN answer_metrics am ON am.question_id = q.id
    LEFT JOIN like_metrics lm ON lm.question_id = q.id
    LEFT JOIN comment_metrics cm ON cm.question_id = q.id
    LEFT JOIN view_metrics vm ON vm.question_id = q.id
    WHERE am.question_id IS NOT NULL OR lm.question_id IS NOT NULL
       OR cm.question_id IS NOT NULL OR vm.question_id IS NOT NULL
)
SELECT * FROM hot_rows`
	args := []any{since, since, since, since, asOf}
	if cursor != nil {
		query += " WHERE hot_score < ? OR (hot_score = ? AND question_id < ?)"
		args = append(args, cursor.Score, cursor.Score, cursor.QuestionID)
	}
	query += " ORDER BY hot_score DESC, question_id DESC LIMIT ?"
	args = append(args, limit)

	var items []domainfeed.HotItem
	err := r.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error
	for i := range items {
		items[i].RankedAt = asOf
	}
	return items, err
}

// Find_HotQuestionsByIDs 批量补齐热门快照中的问题展示字段，不重新执行 24 小时聚合。
func (r *FeedRepository) Find_HotQuestionsByIDs(ctx context.Context, questionIDs []uint64) ([]domainfeed.HotItem, error) {
	if len(questionIDs) == 0 {
		return []domainfeed.HotItem{}, nil
	}
	var items []domainfeed.HotItem
	err := r.db.WithContext(ctx).Table("questions AS q").
		Select(`q.id AS question_id, q.title AS question_title, q.content AS question_content,
			u.id AS question_author_id, u.username AS question_author_name,
			u.nickname AS question_author_nick, u.avatar_url AS question_author_avatar,
			q.created_at AS question_created_at`).
		Joins("JOIN users u ON u.id = q.author_id").
		Where("q.id IN ?", questionIDs).
		Scan(&items).Error
	return items, err
}

// Find_HotItemsByQuestionIDs 为 Redis 热榜候选一次性补齐问题信息和 24 小时统计。
func (r *FeedRepository) Find_HotItemsByQuestionIDs(ctx context.Context, questionIDs []uint64, since, asOf time.Time) ([]domainfeed.HotItem, error) {
	if len(questionIDs) == 0 {
		return []domainfeed.HotItem{}, nil
	}
	query := `
SELECT
    q.id AS question_id, q.title AS question_title, q.content AS question_content,
    u.id AS question_author_id, u.username AS question_author_name,
    u.nickname AS question_author_nick, u.avatar_url AS question_author_avatar,
    q.created_at AS question_created_at,
    (SELECT COUNT(*) FROM answers a WHERE a.question_id = q.id AND a.created_at >= ?) AS answer_count_24h,
    (SELECT COUNT(*) FROM answer_likes l JOIN answers a ON a.id = l.answer_id
        WHERE a.question_id = q.id AND l.created_at >= ?) AS like_count_24h,
    (SELECT COUNT(*) FROM answer_comments c JOIN answers a ON a.id = c.answer_id
        WHERE a.question_id = q.id AND c.created_at >= ?) AS comment_count_24h,
    (SELECT COUNT(*) FROM question_views v WHERE v.question_id = q.id AND v.created_at >= ?) AS view_count_24h
FROM questions q
JOIN users u ON u.id = q.author_id
WHERE q.id IN ?`
	var items []domainfeed.HotItem
	err := r.db.WithContext(ctx).Raw(query, since, since, since, since, questionIDs).Scan(&items).Error
	for i := range items {
		items[i].RankedAt = asOf
	}
	return items, err
}

func (r *FeedRepository) Find_RecommendedFeed(ctx context.Context, userID uint64, asOf time.Time, cursor *domainfeed.RecommendedCursor, limit int) ([]domainfeed.RecommendedItem, error) {
	query := `
WITH scored_answers AS (
    SELECT
        q.id AS question_id, q.title AS question_title,
        qu.id AS question_author_id, qu.username AS question_author_name,
        qu.nickname AS question_author_nick, qu.avatar_url AS question_author_avatar,
        a.id AS answer_id, a.content AS answer_content, a.created_at AS answer_created_at,
        au.id AS answer_author_id, au.username AS answer_author_name,
        au.nickname AS answer_author_nick, au.avatar_url AS answer_author_avatar,
        a.like_count, a.comment_count, a.favorite_count,
        CASE WHEN uf.id IS NULL THEN FALSE ELSE TRUE END AS following_author,
        (
            1 + a.like_count * 2 + a.comment_count * 3 + a.favorite_count * 2 +
            CASE WHEN uf.id IS NULL THEN 0 ELSE 5 END
        ) / POW(2 + TIMESTAMPDIFF(SECOND, a.created_at, ?) / 86400, 1.2) AS recommend_score
    FROM answers a
    JOIN questions q ON q.id = a.question_id
    JOIN users qu ON qu.id = q.author_id
    JOIN users au ON au.id = a.author_id
    LEFT JOIN user_follows uf ON uf.followee_id = a.author_id AND uf.follower_id = ?
),
ranked_answers AS (
    SELECT scored_answers.*,
           ROW_NUMBER() OVER (
               PARTITION BY question_id
               ORDER BY recommend_score DESC, answer_id DESC
           ) AS feed_rank
    FROM scored_answers
)
SELECT * FROM ranked_answers WHERE feed_rank = 1`
	args := []any{asOf, userID}
	if cursor != nil {
		query += " AND (recommend_score < ? OR (recommend_score = ? AND answer_id < ?))"
		args = append(args, cursor.Score, cursor.Score, cursor.AnswerID)
	}
	query += " ORDER BY recommend_score DESC, answer_id DESC LIMIT ?"
	args = append(args, limit)

	var items []domainfeed.RecommendedItem
	err := r.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error
	for i := range items {
		items[i].RankedAt = asOf
	}
	return items, err
}

// Find_RecommendedItemsByAnswerIDs 批量补齐推荐缓存中的轻量 ID 列表，不重新计算推荐分。
func (r *FeedRepository) Find_RecommendedItemsByAnswerIDs(ctx context.Context, userID uint64, answerIDs []uint64) ([]domainfeed.RecommendedItem, error) {
	if len(answerIDs) == 0 {
		return []domainfeed.RecommendedItem{}, nil
	}
	var items []domainfeed.RecommendedItem
	err := r.db.WithContext(ctx).Table("answers AS a").
		Select(`
			q.id AS question_id, q.title AS question_title,
			qu.id AS question_author_id, qu.username AS question_author_name,
			qu.nickname AS question_author_nick, qu.avatar_url AS question_author_avatar,
			a.id AS answer_id, a.content AS answer_content, a.created_at AS answer_created_at,
			au.id AS answer_author_id, au.username AS answer_author_name,
			au.nickname AS answer_author_nick, au.avatar_url AS answer_author_avatar,
			a.like_count, a.comment_count, a.favorite_count,
			CASE WHEN uf.id IS NULL THEN FALSE ELSE TRUE END AS following_author`).
		Joins("JOIN questions q ON q.id = a.question_id").
		Joins("JOIN users qu ON qu.id = q.author_id").
		Joins("JOIN users au ON au.id = a.author_id").
		Joins("LEFT JOIN user_follows uf ON uf.followee_id = a.author_id AND uf.follower_id = ?", userID).
		Where("a.id IN ?", answerIDs).
		Scan(&items).Error
	return items, err
}
