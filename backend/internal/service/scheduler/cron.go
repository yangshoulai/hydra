package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// CronScheduler 定时任务调度器
type CronScheduler struct {
	logger *slog.Logger
	cron   *cron.Cron
	jobs   map[string]*cron.Job
}

// NewCronScheduler 创建定时任务调度器
func NewCronScheduler(logger *slog.Logger) *CronScheduler {
	// 创建秒级精度的cron调度器
	c := cron.New(cron.WithSeconds())

	return &CronScheduler{
		logger: logger,
		cron:   c,
		jobs:   make(map[string]*cron.Job),
	}
}

// ScheduledTask 定时任务函数类型
type ScheduledTask func(ctx context.Context) error

// AddJob 添加定时任务
// name: 任务名称
// spec: cron表达式（支持秒级精度，例如："0 0 3 * * *" 表示每天凌晨3点）
// task: 任务执行函数
func (s *CronScheduler) AddJob(name string, spec string, task ScheduledTask) error {
	s.logger.Info("adding scheduled job",
		slog.String("job_name", name),
		slog.String("cron_spec", spec),
	)

	// 包装任务，添加context和错误处理
	wrappedTask := func() {
		ctx := context.Background()
		s.logger.Info("executing scheduled job",
			slog.String("job_name", name),
		)

		if err := task(ctx); err != nil {
			s.logger.Error("scheduled job execution failed",
				slog.String("job_name", name),
				slog.String("error", err.Error()),
			)
		} else {
			s.logger.Info("scheduled job execution completed",
				slog.String("job_name", name),
			)
		}
	}

	// 添加任务到cron
	_, err := s.cron.AddFunc(spec, wrappedTask)
	if err != nil {
		s.logger.Error("failed to add scheduled job",
			slog.String("job_name", name),
			slog.String("error", err.Error()),
		)
		return err
	}

	s.logger.Info("scheduled job added successfully",
		slog.String("job_name", name),
	)

	return nil
}

// RemoveJob 移除定时任务
func (s *CronScheduler) RemoveJob(name string) {
	s.logger.Info("removing scheduled job",
		slog.String("job_name", name),
	)
	// cron库没有直接支持通过名称移除任务的API
	// 这里我们记录日志，实际使用时可以通过Stop()停止所有任务然后重新添加
	delete(s.jobs, name)
}

// Start 启动调度器
func (s *CronScheduler) Start() {
	s.logger.Info("starting cron scheduler")
	s.cron.Start()
}

// Stop 停止调度器
func (s *CronScheduler) Stop() {
	s.logger.Info("stopping cron scheduler")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("cron scheduler stopped")
}

// GetNextRunTime 获取任务下次运行时间
func (s *CronScheduler) GetNextRunTime(spec string) (time.Time, error) {
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(time.Now()), nil
}

// Helper cron表达式常量
const (
	// EveryHour 每小时执行
	EveryHour = "0 0 * * * *"
	// EveryDayAt3AM 每天凌晨3点执行
	EveryDayAt3AM = "0 0 3 * * *"
	// EveryDayAtMidnight 每天午夜执行
	EveryDayAtMidnight = "0 0 0 * * *"
	// EveryMonday 凌晨3点 周一凌晨3点执行
	EveryMondayAt3AM = "0 0 3 * * 1"
	// EveryDayAt6AM 每天6点执行
	EveryDayAt6AM = "0 0 6 * * *"
	// EveryDayAt9PM 每天21点执行
	EveryDayAt9PM = "0 0 21 * * *"
	// Every15Minutes 每15分钟执行
	Every15Minutes = "0 */15 * * * *"
	// Every30Minutes 每30分钟执行
	Every30Minutes = "0 */30 * * * *"
)
