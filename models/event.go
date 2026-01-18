package models

import (
	"time"
)

type Event struct {
	EventID    string                 `json:"event_id"`
	Timestamp  time.Time              `json:"timestamp"`
	LocalTime  time.Time              `json:"local_time"`
	AppID      string                 `json:"app_id"`
	AppVersion string                 `json:"app_version,omitempty"`
	EventType  string                 `json:"event_type"`
	EventName  string                 `json:"event_name"`
	User       UserInfo               `json:"user"`
	Device     DeviceInfo             `json:"device"`
	Location   *LocationInfo          `json:"location,omitempty"`
	Web        *WebInfo               `json:"web_specific,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type UserInfo struct {
	ID          string `json:"id,omitempty"`
	SessionID   string `json:"session_id"`
	AnonymousID string `json:"anonymous_id,omitempty"`
}

type DeviceInfo struct {
	Platform         string `json:"platform"`
	OSVersion        string `json:"os_version,omitempty"`
	DeviceModel      string `json:"device_model,omitempty"`
	ScreenResolution string `json:"screen_resolution,omitempty"`
	Locale           string `json:"locale,omitempty"`
	Timezone         string `json:"timezone,omitempty"`
}

type LocationInfo struct {
	Country string `json:"country,omitempty"`
	Region  string `json:"region,omitempty"`
	City    string `json:"city,omitempty"`
	IP      string `json:"ip,omitempty"`
}

type WebInfo struct {
	UserAgent   string `json:"user_agent,omitempty"`
	Referrer    string `json:"referrer,omitempty"`
	UTMSource   string `json:"utm_source,omitempty"`
	UTMMedium   string `json:"utm_medium,omitempty"`
	UTMCampaign string `json:"utm_campaign,omitempty"`
	PageURL     string `json:"page_url,omitempty"`
	PageTitle   string `json:"page_title,omitempty"`
}

// EventType constants
const (
	EventTypePageView = "page_view"
	EventTypeClick    = "click"
	EventTypeSignup   = "signup"
	EventTypePurchase = "purchase"
	EventTypeCustom   = "custom"
)
