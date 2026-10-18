// Package main 是 AskFlow HTTP 服务的启动入口。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	applicationanswer "askflow/internal/application/answer"
	applicationanswerfavorite "askflow/internal/application/answerfavorite"
	applicationanswerlike "askflow/internal/application/answerlike"
	applicationanswerranking "askflow/internal/application/answerranking"
	applicationcomment "askflow/internal/application/comment"
	applicationfeed "askflow/internal/application/feed"
	applicationfollow "askflow/internal/application/follow"
	applicationnotification "askflow/internal/application/notification"
	applicationquestion "askflow/internal/application/question"
	applicationuser "askflow/internal/application/user"
	"askflow/internal/infra/auth"
	infracache "askflow/internal/infra/cache"
	"askflow/internal/infra/config"
	"askflow/internal/infra/database"
	"askflow/internal/infra/messaging"
	infranotification "askflow/internal/infra/notification"
	"askflow/internal/infra/outbox"
	"askflow/internal/infra/repository"
	"askflow/internal/infra/security"
	httpapi "askflow/internal/interfaces/http"
	answerhttp "askflow/internal/interfaces/http/answer"
	answerfavoritehttp "askflow/internal/interfaces/http/answerfavorite"
	answerlikehttp "askflow/internal/interfaces/http/answerlike"
	commenthttp "askflow/internal/interfaces/http/comment"
	feedhttp "askflow/internal/interfaces/http/feed"
	followhttp "askflow/internal/interfaces/http/follow"
	"askflow/internal/interfaces/http/middleware"
	notificationhttp "askflow/internal/interfaces/http/notification"
	questionhttp "askflow/internal/interfaces/http/question"
	userhttp "askflow/internal/interfaces/http/user"
)

func main() {
	if err := run(); err != nil {
		slog.Error("AskFlow 运行失败", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		return err
	}

	mysqlDB, err := database.NewMySQL(cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		if err := mysqlDB.Close(); err != nil {
			slog.Error("关闭 MySQL 连接失败", "error", err)
		}
	}()
	if err := database.Migrate(mysqlDB.DB); err != nil {
		return err
	}
	slog.Info("MySQL 连接成功")

	redisStore := infracache.NewRedisStore(cfg.Redis)
	defer func() {
		if err := redisStore.Close(); err != nil {
			slog.Error("关闭 Redis 连接失败", "error", err)
		}
	}()
	mysqlUserRepository := repository.NewUserRepository(mysqlDB.DB)
	userRepository := infracache.NewUserRepository(mysqlUserRepository, redisStore)
	passwordManager := security.BCryptHasher{}
	registerService := applicationuser.NewRegisterService(userRepository, passwordManager)
	tokenIssuer, err := auth.NewJWTIssuer(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	if err != nil {
		return fmt.Errorf("创建 JWT 签发器: %w", err)
	}
	loginService := applicationuser.NewLoginService(userRepository, passwordManager, tokenIssuer)
	profileService := applicationuser.NewProfileService(userRepository)
	mysqlQuestionRepository := repository.NewQuestionRepository(mysqlDB.DB)
	questionRepository := infracache.NewQuestionRepository(mysqlQuestionRepository, redisStore)
	publishQuestionService := applicationquestion.NewPublishService(questionRepository)
	questionViewRepository := repository.NewQuestionViewRepository(mysqlDB.DB)
	questionDetailService := applicationquestion.NewDetailService(questionRepository, userRepository, questionViewRepository)
	mysqlAnswerRepository := repository.NewAnswerRepository(mysqlDB.DB)
	answerRepository := infracache.NewAnswerRepository(mysqlAnswerRepository, redisStore)
	mysqlAnswerRankingRepository := repository.NewAnswerRankingRepository(mysqlDB.DB)
	answerRankingRepository := infracache.NewAnswerRankingRepository(mysqlAnswerRankingRepository, answerRepository, userRepository, redisStore)
	answerRepository.SetAnswerRankInvalidator(answerRankingRepository)
	publishAnswerService := applicationanswer.NewPublishService(answerRepository, questionRepository)
	answerListService := applicationanswerranking.NewService(answerRankingRepository, questionRepository)
	mysqlFeedRepository := repository.NewFeedRepository(mysqlDB.DB)
	followingFeedRepository := infracache.NewFollowingFeedRepository(mysqlFeedRepository, mysqlFeedRepository, redisStore)
	recommendedFeedRepository := infracache.NewRecommendedFeedRepository(followingFeedRepository, mysqlFeedRepository, redisStore)
	feedRepository := infracache.NewHotFeedRepository(recommendedFeedRepository, mysqlFeedRepository, redisStore, redisStore)
	mysqlFollowRepository := repository.NewFollowRepository(mysqlDB.DB)
	followRepository := infracache.NewFollowRepository(mysqlFollowRepository, followingFeedRepository, recommendedFeedRepository)
	followService := applicationfollow.NewService(followRepository, userRepository)
	followingFeedService := applicationfeed.NewFollowingService(feedRepository)
	hotFeedService := applicationfeed.NewHotService(feedRepository)
	recommendedFeedService := applicationfeed.NewRecommendedService(feedRepository)
	mysqlAnswerLikeRepository := repository.NewAnswerLikeRepository(mysqlDB.DB)
	answerLikeRepository := infracache.NewAnswerLikeRepository(mysqlAnswerLikeRepository, answerRepository, answerRepository, answerRankingRepository)
	answerLikeService := applicationanswerlike.NewService(answerLikeRepository, answerRepository)
	mysqlCommentRepository := repository.NewCommentRepository(mysqlDB.DB)
	commentRepository := infracache.NewCommentRepository(mysqlCommentRepository, answerRepository, answerRepository, answerRankingRepository)
	commentService := applicationcomment.NewService(commentRepository, answerRepository, userRepository)
	mysqlAnswerFavoriteRepository := repository.NewAnswerFavoriteRepository(mysqlDB.DB)
	answerFavoriteRepository := infracache.NewAnswerFavoriteRepository(mysqlAnswerFavoriteRepository, answerRepository, answerRepository, answerRankingRepository)
	answerFavoriteService := applicationanswerfavorite.NewService(answerFavoriteRepository, answerRepository)
	notificationRepository := repository.NewNotificationRepository(mysqlDB.DB)
	notificationHub := applicationnotification.NewHub()
	notificationService := applicationnotification.NewService(notificationRepository, answerRepository, notificationHub)

	outboxStore := outbox.NewStore(mysqlDB.DB)
	outboxRetention, err := time.ParseDuration(cfg.RabbitMQ.OutboxRetention)
	if err != nil {
		return fmt.Errorf("解析 Outbox 保留时间 %q: %w", cfg.RabbitMQ.OutboxRetention, err)
	}
	if outboxRetention <= 0 {
		return fmt.Errorf("Outbox 保留时间必须大于 0")
	}
	outboxCleanupInterval, err := time.ParseDuration(cfg.RabbitMQ.OutboxCleanupInterval)
	if err != nil {
		return fmt.Errorf("解析 Outbox 清理间隔 %q: %w", cfg.RabbitMQ.OutboxCleanupInterval, err)
	}
	if outboxCleanupInterval <= 0 {
		return fmt.Errorf("Outbox 清理间隔必须大于 0")
	}
	if cfg.RabbitMQ.OutboxCleanupBatchSize <= 0 {
		return fmt.Errorf("Outbox 清理批次大小必须大于 0")
	}
	workerContext, stopWorkers := context.WithCancel(context.Background())
	rabbitPublisher := messaging.NewRabbitPublisher(cfg.RabbitMQ)
	outboxWorker := outbox.NewWorker(outboxStore, rabbitPublisher)
	outboxCleanupWorker := outbox.NewCleanupWorker(outboxStore, outboxRetention, outboxCleanupInterval, cfg.RabbitMQ.OutboxCleanupBatchSize)
	eventHandler := infracache.NewEventInvalidationHandler(answerRepository, answerRankingRepository, followingFeedRepository, recommendedFeedRepository, redisStore)
	rabbitConsumer := messaging.NewRabbitConsumer(cfg.RabbitMQ, "askflow.cache-invalidation.v1", eventHandler)
	notificationEventHandler := infranotification.NewEventHandler(notificationService)
	notificationConsumer := messaging.NewRabbitConsumer(cfg.RabbitMQ, "askflow.notifications.v1", notificationEventHandler)
	var workerGroup sync.WaitGroup
	workerGroup.Add(4)
	go func() {
		defer workerGroup.Done()
		outboxWorker.Run(workerContext)
	}()
	go func() {
		defer workerGroup.Done()
		rabbitConsumer.Run(workerContext)
	}()
	go func() {
		defer workerGroup.Done()
		notificationConsumer.Run(workerContext)
	}()
	go func() {
		defer workerGroup.Done()
		outboxCleanupWorker.Run(workerContext)
	}()
	defer func() {
		stopWorkers()
		workerGroup.Wait()
		if err := rabbitPublisher.Close(); err != nil {
			slog.Warn("关闭 RabbitMQ 发布器失败", "error", err)
		}
	}()

	router := httpapi.NewRouter()
	httpapi.RegisterMetricsRoute(router, infracache.DefaultMetrics)
	apiV1 := router.Group("/api/v1")
	userhttp.RegisterPublicRoutes(apiV1, registerService, loginService)
	questionhttp.RegisterPublicRoutes(apiV1, questionDetailService)
	answerhttp.RegisterPublicRoutes(apiV1, answerListService)
	commenthttp.RegisterPublicRoutes(apiV1, commentService)
	followhttp.RegisterPublicRoutes(apiV1, followService)
	feedhttp.RegisterPublicRoutes(apiV1, hotFeedService)
	recommended := apiV1.Group("")
	recommended.Use(middleware.OptionalAuthenticate(tokenIssuer))
	feedhttp.RegisterRecommendedRoutes(recommended, recommendedFeedService)
	authenticated := apiV1.Group("")
	authenticated.Use(middleware.Authenticate(tokenIssuer))
	userhttp.RegisterProtectedRoutes(authenticated, profileService)
	questionhttp.RegisterProtectedRoutes(authenticated, publishQuestionService)
	answerhttp.RegisterProtectedRoutes(authenticated, publishAnswerService)
	answerlikehttp.RegisterProtectedRoutes(authenticated, answerLikeService)
	commenthttp.RegisterProtectedRoutes(authenticated, commentService)
	answerfavoritehttp.RegisterProtectedRoutes(authenticated, answerFavoriteService)
	followhttp.RegisterProtectedRoutes(authenticated, followService)
	feedhttp.RegisterProtectedRoutes(authenticated, followingFeedService)
	notificationhttp.RegisterProtectedRoutes(authenticated, notificationService)

	address := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("AskFlow 服务启动", "address", address)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务启动失败: %w", err)
		}
	case sig := <-shutdownSignal:
		slog.Info("收到关闭信号", "signal", sig.String())
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("HTTP 服务关闭失败: %w", err)
	}

	slog.Info("AskFlow 服务已关闭")
	return nil

}
