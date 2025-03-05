package model

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lemonlinger/llm-test/config"
)

// AnthropicModel Anthropic模型实现
type AnthropicModel struct {
	BaseModel
	defaultClient *http.Client
	proxyClients  map[string]*http.Client // 代理名称到对应HTTP客户端的映射
}

// NewAnthropicModel 创建新的Anthropic模型
func NewAnthropicModel(cfg config.ModelConfig, proxies []config.ProxyConfig) (*AnthropicModel, error) {
	// 创建默认客户端
	defaultClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 创建代理客户端映射
	proxyClients := make(map[string]*http.Client)

	// 为每个代理创建对应的HTTP客户端
	for _, proxy := range proxies {
		// 解析代理URL
		parsedURL, err := url.Parse(proxy.URL)
		if err != nil {
			log.Printf("解析代理URL失败 (%s): %v", proxy.Name, err)
			continue
		}

		// 创建带有代理的Transport
		transport := &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
		}

		// 创建客户端并存储
		proxyClients[proxy.Name] = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		}
	}

	return &AnthropicModel{
		BaseModel: BaseModel{
			config: cfg,
		},
		defaultClient: defaultClient,
		proxyClients:  proxyClients,
	}, nil
}

// GenerateResponse 生成响应
func (m *AnthropicModel) GenerateResponse(ctx context.Context, systemMessage, userMessage string, stream bool) (*LLMResponse, error) {
	// 选择合适的HTTP客户端
	var client *http.Client

	// 如果模型配置了代理，并且代理客户端存在，则使用代理客户端
	if m.config.ProxyName != "" {
		if proxyClient, ok := m.proxyClients[m.config.ProxyName]; ok {
			client = proxyClient
			log.Printf("使用代理: %s", m.config.ProxyName)
		} else {
			log.Printf("未找到配置的代理: %s，使用默认客户端", m.config.ProxyName)
			client = m.defaultClient
		}
	} else {
		client = m.defaultClient
	}

	// 这里应该实现真正的API调用，使用选择的client
	// 目前只是记录一下使用的客户端，避免编译器警告
	_ = client

	// 作为示例，我们只是模拟一个延迟并返回一个固定的响应

	// 计算输入token
	inputTokens, err := m.CountTokens(systemMessage + userMessage)
	if err != nil {
		return nil, err
	}

	// 模拟API延迟
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(250+inputTokens/8) * time.Millisecond):
		// 模拟响应
		response := "这是来自Anthropic模型的示例响应。"

		// 计算输出token
		outputTokens, err := m.CountTokens(response)
		if err != nil {
			return nil, err
		}

		return &LLMResponse{
			Content:      response,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}, nil
	}
}

// CountTokens 计算文本的token数量
func (m *AnthropicModel) CountTokens(text string) (int, error) {
	// 简单估算，实际应使用Claude的tokenizer
	words := strings.Fields(text)
	return len(words) + len(text)/4, nil
}
