package services

import (
	"domain-monitor/internal/database"
	"domain-monitor/internal/models"
	"fmt"
	"log"
	"time"
)

// MonitorService handles domain monitoring
type MonitorService struct {
	whoisService *WhoisService
	notifyService *NotifyService
	alertDays    []int
}

// NewMonitorService creates a new monitoring service
func NewMonitorService(whoisService *WhoisService, notifyService *NotifyService, alertDays []int) *MonitorService {
	return &MonitorService{
		whoisService:  whoisService,
		notifyService: notifyService,
		alertDays:     alertDays,
	}
}

// CheckAllDomains checks all active domains
func (s *MonitorService) CheckAllDomains() error {
	db := database.GetDB()

	var domains []models.Domain
	if err := db.Where("is_active = ?", true).Find(&domains).Error; err != nil {
		return fmt.Errorf("failed to fetch domains: %w", err)
	}

	log.Printf("Checking %d domains...", len(domains))

	for _, domain := range domains {
		if err := s.CheckDomain(&domain); err != nil {
			log.Printf("Error checking domain %s: %v", domain.Name, err)
			continue
		}
	}

	return nil
}

// CheckDomain checks a single domain and updates its information
func (s *MonitorService) CheckDomain(domain *models.Domain) error {
	// Query WHOIS information
	info, err := s.whoisService.QueryDomain(domain.Name)
	if err != nil {
		return fmt.Errorf("WHOIS query failed: %w", err)
	}

	// Update domain information
	domain.Registrar = info.Registrar
	domain.ExpiryDate = info.ExpiryDate
	domain.CreatedDate = info.CreatedDate
	domain.UpdatedDate = info.UpdatedDate
	domain.Status = info.Status
	domain.LastChecked = time.Now()

	// Calculate days remaining
	if !info.ExpiryDate.IsZero() {
		domain.DaysRemaining = int(time.Until(info.ExpiryDate).Hours() / 24)
	}

	// Save to database
	db := database.GetDB()
	if err := db.Save(domain).Error; err != nil {
		return fmt.Errorf("failed to save domain: %w", err)
	}

	log.Printf("Updated domain %s: %d days remaining", domain.Name, domain.DaysRemaining)

	// Check if notification is needed
	s.CheckAndNotify(domain)

	return nil
}

// CheckAndNotify checks if notification should be sent
func (s *MonitorService) CheckAndNotify(domain *models.Domain) {
	// Skip if notification service is not available
	if s.notifyService == nil {
		return
	}

	// Check if domain is about to expire
	for _, threshold := range s.alertDays {
		if domain.DaysRemaining == threshold {
			log.Printf("Sending notification for domain %s (%d days remaining)", domain.Name, domain.DaysRemaining)
			if err := s.notifyService.SendNotification(domain, threshold); err != nil {
				log.Printf("Failed to send notification for %s: %v", domain.Name, err)
			}
			break
		}
	}
}

// TriggerNotification manually triggers a notification for testing
func (s *MonitorService) TriggerNotification(domain *models.Domain) error {
	// Skip if notification service is not available
	if s.notifyService == nil {
		return fmt.Errorf("notification service not available")
	}

	log.Printf("Triggering test notification for domain %s (%d days remaining)", domain.Name, domain.DaysRemaining)

	return s.notifyService.SendNotification(domain, domain.DaysRemaining)
}
