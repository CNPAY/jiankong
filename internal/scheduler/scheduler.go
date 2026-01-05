package scheduler

import (
	"domain-monitor/internal/services"
	"log"

	"github.com/robfig/cron/v3"
)

// Scheduler handles scheduled tasks
type Scheduler struct {
	cron           *cron.Cron
	monitorService *services.MonitorService
}

// NewScheduler creates a new scheduler
func NewScheduler(monitorService *services.MonitorService) *Scheduler {
	return &Scheduler{
		cron:           cron.New(),
		monitorService: monitorService,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(checkInterval string) error {
	// Add scheduled job to check all domains
	_, err := s.cron.AddFunc(checkInterval, func() {
		log.Println("Starting scheduled domain check...")
		if err := s.monitorService.CheckAllDomains(); err != nil {
			log.Printf("Scheduled check failed: %v", err)
		}
		log.Println("Scheduled domain check completed")
	})

	if err != nil {
		return err
	}

	s.cron.Start()
	log.Printf("Scheduler started with interval: %s", checkInterval)
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("Scheduler stopped")
}
