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

// RemoveJob 移除定时任务
func (s *CronScheduler) RemoveJob(name string) {
	s.logger.Info("移除定时任务",
		slog.String("job_name", name),
	)
	if entryID, ok := s.jobs[name]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, name)
	}
}

// Start 启动调度器
func (s *CronScheduler) Start() {
	s.logger.Info("启动定时任务调度器")
	s.cron.Start()
}

// Stop 停止调度器
func (s *CronScheduler) Stop() {
	s.logger.Info("停止定时任务调度器")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("定时任务调度器已停止")
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
