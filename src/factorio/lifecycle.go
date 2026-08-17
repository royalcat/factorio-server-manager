package factorio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

type StartupProfile struct {
	Savefile string `json:"savefile"`
	BindIP   string `json:"bindip"`
	Port     int    `json:"port"`
	ModPack  string `json:"mod_pack"`
	Public   bool   `json:"public"`
	LAN      bool   `json:"lan"`
}

type RestartSchedule struct {
	Enabled        bool   `json:"enabled"`
	IntervalHours  int    `json:"interval_hours"`
	WarningMinutes []int  `json:"warning_minutes"`
	LastRestart    string `json:"last_restart,omitempty"`
	NextRestart    string `json:"next_restart,omitempty"`
	LastWarningKey string `json:"last_warning_key,omitempty"`
}

type LifecycleEvent struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type CrashSummary struct {
	Time       string   `json:"time,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	RecentLogs []string `json:"recent_logs,omitempty"`
}

type LifecycleConfig struct {
	StartupProfile      StartupProfile   `json:"startup_profile"`
	RestartSchedule     RestartSchedule  `json:"restart_schedule"`
	GracefulStopTimeout int              `json:"graceful_stop_timeout"`
	Events              []LifecycleEvent `json:"events"`
	LastCrash           CrashSummary     `json:"last_crash"`
}

var lifecycleMu sync.Mutex
var lifecycleSchedulerOnce sync.Once
var expectedStop bool

func defaultLifecycleConfig() LifecycleConfig {
	return LifecycleConfig{
		StartupProfile: StartupProfile{
			BindIP: "0.0.0.0",
			Port:   34197,
		},
		RestartSchedule: RestartSchedule{
			IntervalHours:  24,
			WarningMinutes: []int{10, 5, 1},
		},
		GracefulStopTimeout: 30,
		Events:              []LifecycleEvent{},
	}
}

func lifecyclePath() string {
	config := bootstrap.GetConfig()
	return filepath.Join(filepath.Dir(config.ConfFile), "lifecycle.json")
}

func LoadLifecycleConfig() (LifecycleConfig, error) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	return loadLifecycleConfigLocked()
}

func SaveLifecycleConfig(config LifecycleConfig) (LifecycleConfig, error) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	current, _ := loadLifecycleConfigLocked()
	config.Events = current.Events
	config.LastCrash = current.LastCrash
	config.RestartSchedule = normalizeRestartSchedule(config.RestartSchedule)
	if config.GracefulStopTimeout <= 0 {
		config.GracefulStopTimeout = 30
	}
	return config, writeLifecycleConfigLocked(config)
}

func loadLifecycleConfigLocked() (LifecycleConfig, error) {
	config := defaultLifecycleConfig()
	data, err := os.ReadFile(lifecyclePath())
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	config.RestartSchedule = normalizeRestartSchedule(config.RestartSchedule)
	if config.Events == nil {
		config.Events = []LifecycleEvent{}
	}
	if config.GracefulStopTimeout <= 0 {
		config.GracefulStopTimeout = 30
	}
	return config, nil
}

func writeLifecycleConfigLocked(config LifecycleConfig) error {
	if err := os.MkdirAll(filepath.Dir(lifecyclePath()), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lifecyclePath(), data, 0664)
}

func normalizeRestartSchedule(schedule RestartSchedule) RestartSchedule {
	if schedule.IntervalHours <= 0 {
		schedule.IntervalHours = 24
	}
	if len(schedule.WarningMinutes) == 0 {
		schedule.WarningMinutes = []int{10, 5, 1}
	}
	if schedule.Enabled && schedule.NextRestart == "" {
		next := time.Now().Add(time.Duration(schedule.IntervalHours) * time.Hour).UTC()
		schedule.NextRestart = next.Format(time.RFC3339)
	}
	return schedule
}

func AppendLifecycleEvent(eventType, message string) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	config, _ := loadLifecycleConfigLocked()
	config.Events = append([]LifecycleEvent{{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Type:    eventType,
		Message: message,
	}}, config.Events...)
	if len(config.Events) > 50 {
		config.Events = config.Events[:50]
	}
	_ = writeLifecycleConfigLocked(config)
}

func RecordCrash(reason string, recentLogs []string) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	config, _ := loadLifecycleConfigLocked()
	config.LastCrash = CrashSummary{
		Time:       time.Now().UTC().Format(time.RFC3339),
		Reason:     reason,
		RecentLogs: recentLogs,
	}
	config.Events = append([]LifecycleEvent{{
		Time:    config.LastCrash.Time,
		Type:    "crash",
		Message: reason,
	}}, config.Events...)
	if len(config.Events) > 50 {
		config.Events = config.Events[:50]
	}
	_ = writeLifecycleConfigLocked(config)
}

func StartLifecycleScheduler() {
	lifecycleSchedulerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				runLifecycleScheduler()
			}
		}()
	})
}

func runLifecycleScheduler() {
	config, err := LoadLifecycleConfig()
	if err != nil || !config.RestartSchedule.Enabled || config.RestartSchedule.NextRestart == "" {
		return
	}

	next, err := time.Parse(time.RFC3339, config.RestartSchedule.NextRestart)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	server := GetFactorioServer()
	if !server.GetRunning() {
		return
	}

	for _, warning := range config.RestartSchedule.WarningMinutes {
		warnAt := next.Add(-time.Duration(warning) * time.Minute)
		key := fmt.Sprintf("%s-%d", next.Format(time.RFC3339), warning)
		if !now.Before(warnAt) && now.Before(next) && config.RestartSchedule.LastWarningKey != key {
			sendChatMessage(fmt.Sprintf("[FSM] Server restart in %d minute(s).", warning))
			config.RestartSchedule.LastWarningKey = key
			_, _ = SaveLifecycleConfig(config)
			return
		}
	}

	if now.Before(next) {
		return
	}

	AppendLifecycleEvent("restart", "Scheduled restart started")
	if err := server.RestartWithProfile(config); err != nil {
		AppendLifecycleEvent("restart_failed", err.Error())
		return
	}
	config.RestartSchedule.LastRestart = now.Format(time.RFC3339)
	config.RestartSchedule.NextRestart = now.Add(time.Duration(config.RestartSchedule.IntervalHours) * time.Hour).Format(time.RFC3339)
	config.RestartSchedule.LastWarningKey = ""
	_, _ = SaveLifecycleConfig(config)
}

func sendChatMessage(message string) {
	server := GetFactorioServer()
	if server.Rcon == nil {
		return
	}
	_, _ = server.Rcon.Write(fmt.Sprintf(`/silent-command game.print("%s")`, message))
}

func markExpectedStop(value bool) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	expectedStop = value
}

func wasExpectedStop() bool {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	return expectedStop
}
