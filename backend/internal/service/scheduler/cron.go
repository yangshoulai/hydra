package scheduler

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// CronScheduler 定时任务调度器
type CronScheduler struct {
	logger *slog.Logger
	cron   *cron.Cron
	jobs   map[string]cron.EntryID
}

// NewCronScheduler 创建定时任务调度器
func NewCronScheduler(logger *slog.Logger) *CronScheduler {
	// 创建秒级精度的cron调度器
	c := cron.New(cron.WithSeconds())

	return &CronScheduler{
		logger: logger,
		cron:   c,
		jobs:   make(map[string]cron.EntryID),
	}
}

// ScheduledTask 定时任务函数类型
type ScheduledTask func(ctx context.Context) error

// AddJob 添加定时任务
// name: 任务名称
// spec: cron表达式（支持秒级精度，例如："0 0 3 * * *" 表示每天凌晨3点）
// task: 任务执行函数
func (s *CronScheduler) AddJob(name string, spec string, task ScheduledTask) error {
	s.logger.Info("添加定时任务",
		slog.String("job_name", name),
		slog.String("cron_spec", spec),
	)
	if entryID, ok := s.jobs[name]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, name)
	}

	// 包装任务，添加context和错误处理
	wrappedTask := func() {
		ctx := context.Background()
		s.logger.Info("执行定时任务",
			slog.String("job_name", name),
		)

		if err := task(ctx); err != nil {
			s.logger.Error("定时任务执行失败",
				slog.String("job_name", name),
				slog.String("error", err.Error()),
			)
		} else {
			s.logger.Info("定时任务执行完成",
				slog.String("job_name", name),
			)
		}
	}

	// 添加任务到cron
	entryID, err := s.cron.AddFunc(spec, wrappedTask)
	if err != nil {
		s.logger.Error("添加定时任务失败",
			slog.String("job_name", name),
			slog.String("error", err.Error()),
		)
		return err
	}

	s.jobs[name] = entryID
	s.logger.Info("定时任务添加成功",
		slog.String("job_name", name),
	)

	return nil
}

// Start 启动调度器
func (s *CronScheduler) Start() {
	s.cron.Start()
}

// Stop 停止调度器
func (s *CronScheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("定时任务调度器已停止")
}

// Helper cron表达式常量
const (
	// EveryHour 每小时执行
	EveryHour = "0 0 * * * *"
)
