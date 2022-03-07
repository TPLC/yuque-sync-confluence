package config

type Config struct {
	Yuque        *YuqueConfig
	Confluence   *ConfluenceConfig
	Notification *NotificationConfig
}

type YuqueConfig struct {
	Domain       string   `json:"domain"`
	UserId       string   `json:"user_id"`
	Auth         string   `json:"auth"`
	SyncRepos    []string `json:"sync_repos"`
	OutSyncRepos []string `json:"out_sync_repos"`
}

type ConfluenceConfig struct {
	Domain string `json:"domain"`
	Space  string `json:"space"`
	Auth   string `json:"auth"`
}

type NotificationConfig struct {
	Url string `json:"url"`
}
