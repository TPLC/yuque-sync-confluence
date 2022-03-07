package notification

import (
	"encoding/json"
	"fmt"
	"yuque-sync-confluence/config"
	"yuque-sync-confluence/internal/httputil"
)

type Api struct {
	url string
}

func NewClient(cfg *config.NotificationConfig) *Api {
	return &Api{
		url: cfg.Url,
	}
}

func (a *Api) Notify(err error) {
	url := a.url
	options := map[string]string{
		"Content-Type": "application/json",
	}
	// req struct类型根据不同webhook需要修改，这里是目前用的通讯软件的webhook的body格式
	req := struct {
		Tag  string `json:"tag"`
		Text struct {
			Content string `json:"content"`
		} `json:"text"`
	}{
		"text",
		struct {
			Content string `json:"content"`
		}{
			Content: func() string {
				if err != nil {
					return fmt.Sprintf("语雀同步文档失败😅\n错误信息: %v", err)
				}
				return "语雀同步文档成功"
			}()},
	}
	reqBytes, _ := json.Marshal(req)
	_, _ = httputil.Post(url, options, nil, reqBytes)
}
