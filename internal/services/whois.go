package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// DomainInfo represents WHOIS query result
type DomainInfo struct {
	Domain      string    `json:"domain"`
	Registrar   string    `json:"registrar"`
	ExpiryDate  time.Time `json:"expiry_date"`
	CreatedDate time.Time `json:"created_date"`
	UpdatedDate time.Time `json:"updated_date"`
	Status      string    `json:"status"`
	NameServers []string  `json:"name_servers"`
	RawData     string    `json:"raw_data"`
}

// WhoisService handles WHOIS queries
type WhoisService struct {
	APIURL  string
	Timeout time.Duration
}

// NewWhoisService creates a new WHOIS service
func NewWhoisService(apiURL string, timeout time.Duration) *WhoisService {
	return &WhoisService{
		APIURL:  apiURL,
		Timeout: timeout,
	}
}

// QueryDomain queries WHOIS information for a domain
func (s *WhoisService) QueryDomain(domain string) (*DomainInfo, error) {
	// Build API URL with parameters
	apiURL, err := url.Parse(s.APIURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API URL: %w", err)
	}

	params := url.Values{}
	params.Add("domain", domain)
	apiURL.RawQuery = params.Encode()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: s.Timeout,
	}

	// Send GET request
	resp, err := client.Get(apiURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to query WHOIS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WHOIS API returned status %d", resp.StatusCode)
	}

	// Parse response (API returns {code, msg, data})
	var apiResponse struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse WHOIS response: %w", err)
	}

	// Check if query was successful
	if apiResponse.Code != 0 {
		return nil, fmt.Errorf("WHOIS API error: %s", apiResponse.Msg)
	}

	result := apiResponse.Data
	if result == nil {
		return nil, fmt.Errorf("no data in WHOIS response")
	}

	// Extract domain information
	domainInfo := &DomainInfo{
		Domain: domain,
	}

	// Parse common fields from the API response
	if registrar, ok := result["registrar"].(string); ok {
		domainInfo.Registrar = registrar
	}

	// API uses "status" as an array, get first element or join them
	if statusList, ok := result["status"].([]interface{}); ok && len(statusList) > 0 {
		if statusObj, ok := statusList[0].(map[string]interface{}); ok {
			if statusText, ok := statusObj["text"].(string); ok {
				domainInfo.Status = statusText
			}
		}
	}

	// Parse dates (API uses: expirationDate, creationDate, updatedDate)
	if expiryStr, ok := result["expirationDate"].(string); ok {
		if t, err := parseDate(expiryStr); err == nil {
			domainInfo.ExpiryDate = t
		}
	}

	if createdStr, ok := result["creationDate"].(string); ok {
		if t, err := parseDate(createdStr); err == nil {
			domainInfo.CreatedDate = t
		}
	}

	if updatedStr, ok := result["updatedDate"].(string); ok {
		if t, err := parseDate(updatedStr); err == nil {
			domainInfo.UpdatedDate = t
		}
	}

	// Parse name servers
	if nameServers, ok := result["nameServers"].([]interface{}); ok {
		for _, ns := range nameServers {
			if nsStr, ok := ns.(string); ok {
				domainInfo.NameServers = append(domainInfo.NameServers, nsStr)
			}
		}
	}

	// Store raw data
	if rawData, err := json.Marshal(result); err == nil {
		domainInfo.RawData = string(rawData)
	}

	return domainInfo, nil
}

// parseDate tries to parse various date formats
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
