// Package translate 提供跨渠道的外部多语言翻译客户端（管理后台多语言表单
// 组件"一键翻译"）。语义对齐 PHP 上游 app/common/library/Translate：把一段
// 文本翻译成若干目标语言。
//
// 它是传输无关的领域服务：只做外部 HTTP 调用并把结果映射成 lan_to → text_to，
// 不触碰数据库；失败时返回带可读信息的错误（不 panic），由渠道 handler 映射
// HTTP 响应。
package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"buildadmin-go/internal/conf"
)

// defaultAPI 是翻译服务 base URL 的代码兜底。正式默认值在
// config.defaults.yaml 的 translate.api（开箱即用、业务可覆盖）；仅当配置
// 段缺失或为空时回退到此处，避免把唯一默认单点硬编码在代码里。
const defaultAPI = "https://translate.preview-dev.live/api/country/"

// defaultTimeout 是单次翻译请求的默认超时（config.translate.timeout <=0 时生效）。
const defaultTimeout = 120 * time.Second

// Client 是翻译服务 HTTP 客户端。API 地址与超时来自 conf.Translate；
// 配置缺失时回退到包内默认值。
type Client struct {
	api  string
	http *http.Client
}

// NewClient 构造翻译客户端。config 为 nil 或 translate 段为空时使用兜底值。
func NewClient(config *conf.Configuration) *Client {
	api := defaultAPI
	timeout := defaultTimeout
	if config != nil {
		if config.Translate.API != "" {
			api = config.Translate.API
		}
		if config.Translate.Timeout > 0 {
			timeout = time.Duration(config.Translate.Timeout) * time.Second
		}
	}
	return &Client{
		api:  strings.TrimRight(api, "/"),
		http: &http.Client{Timeout: timeout},
	}
}

// translateMultiRequest 是 POST {api}/translateMulti 的请求体。注意多目标
// 字段名是 lan_tos（对齐 PHP Translate::translateMulti），不是 lan_to。
type translateMultiRequest struct {
	LanFrom  string   `json:"lan_from"`
	LanTos   []string `json:"lan_tos"`
	TextFrom string   `json:"text_from"`
}

// translateMultiItem 是响应中单条目标语言的翻译结果。
type translateMultiItem struct {
	LanTo  string `json:"lan_to"`
	TextTo string `json:"text_to"`
}

// translateBody 是翻译服务的业务响应体：code=1 表示成功，data 为
// [{lan_to, text_to}]。部分部署/网关可能把业务体再包一层 res。
type translateBody struct {
	Code int                  `json:"code"`
	Msg  string               `json:"msg"`
	Data []translateMultiItem `json:"data"`
}

// translateMultiResponse 同时兼容两种响应形态：
//   - 直出 {code, msg, data}（PHP curl helper 剖出 res 后校验的 body 形态）
//   - 包裹 {code, res:{code, msg, data}}
type translateMultiResponse struct {
	translateBody
	Res *translateBody `json:"res"`
}

// TranslateMulti 把 text 从 from 语言翻译成 tos 指定的全部目标语言，返回
// lan_to → text_to 映射。HTTP 状态 / 业务 code / 空 data 任一不满足契约
// 都返回含可读信息的错误，不 panic。
func (c *Client) TranslateMulti(ctx context.Context, from string, tos []string, text string) (map[string]string, error) {
	if c == nil || c.http == nil {
		return nil, errors.New("translate: client is not initialized")
	}
	if c.api == "" {
		return nil, errors.New("translate: api base url is not configured")
	}
	if len(tos) == 0 {
		return nil, errors.New("translate: no target languages")
	}

	body, err := json.Marshal(translateMultiRequest{
		LanFrom:  from,
		LanTos:   tos,
		TextFrom: text,
	})
	if err != nil {
		return nil, fmt.Errorf("translate: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api+"/translateMulti", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("translate: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("translate: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("translate: service returned http %d: %s", resp.StatusCode, preview(raw))
	}

	var payload translateMultiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("translate: decode response: %w", err)
	}
	seg := payload.translateBody
	if payload.Res != nil {
		seg = *payload.Res
	}
	if seg.Code != 1 {
		msg := strings.TrimSpace(seg.Msg)
		if msg == "" {
			msg = fmt.Sprintf("business code %d", seg.Code)
		}
		return nil, fmt.Errorf("translate: service error: %s", msg)
	}
	if len(seg.Data) == 0 {
		return nil, errors.New("translate: 翻译失败")
	}

	result := make(map[string]string, len(seg.Data))
	for _, item := range seg.Data {
		result[item.LanTo] = item.TextTo
	}
	return result, nil
}

// preview 截断服务端返回体，避免把大段非预期响应直接拼进错误信息。
func preview(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "<empty body>"
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}