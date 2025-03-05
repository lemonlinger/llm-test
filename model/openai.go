package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lemonlinger/llm-test/config"
)

// OpenAIModel OpenAI模型实现
type OpenAIModel struct {
	BaseModel
	defaultClient *http.Client
	proxyClients  map[string]*http.Client // 代理名称到对应HTTP客户端的映射
}

// OpenAIRequest 定义OpenAI API请求结构
type OpenAIRequest struct {
	Model       string                 `json:"model"`
	Messages    []OpenAIMessage        `json:"messages"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"max_tokens"`
	Stream      bool                   `json:"stream,omitempty"`
	Params      map[string]interface{} `json:"-"`
}

// OpenAIMessage 定义OpenAI消息结构
type OpenAIMessage struct {
	Role    string                 `json:"role"`
	Content []OpenAIMessageContent `json:"content,omitempty"`
	Text    string                 `json:"text,omitempty"`
}

// MarshalJSON 自定义序列化方法
func (m OpenAIMessage) MarshalJSON() ([]byte, error) {
	// 创建一个临时结构体用于序列化
	type Alias OpenAIMessage

	// 确保只有一个字段不为空
	if m.Text != "" && len(m.Content) > 0 {
		return nil, fmt.Errorf("both Text and Content cannot be set")
	}

	if m.Text != "" {
		text := m.Text
		m.Text = ""
		return json.Marshal(&struct {
			*Alias
			Content string `json:"content"`
		}{
			Alias:   (*Alias)(&m),
			Content: text,
		})
	} else if len(m.Content) > 0 {
		return json.Marshal(&struct {
			*Alias
			Content []OpenAIMessageContent `json:"content"`
		}{
			Alias:   (*Alias)(&m),
			Content: m.Content,
		})
	}

	// 如果两个字段都为空，返回空的 JSON 对象
	return json.Marshal(&struct {
		*Alias
		Content string `json:"content,omitempty"`
	}{
		Alias:   (*Alias)(&m),
		Content: "",
	})
}

// OpenAIMessageContent 定义消息内容结构
type OpenAIMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// OpenAIResponse 定义OpenAI API响应结构
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// OpenAIChoice 定义OpenAI响应选择结构
type OpenAIChoice struct {
	Index   int `json:"index"`
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// OpenAIStreamResponse 定义OpenAI流式响应的单个消息
type OpenAIStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Delta 表示部分响应内容
type Delta struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
	Role      string `json:"role,omitempty"`
}

// NewOpenAIModel 创建新的OpenAI模型
func NewOpenAIModel(cfg config.ModelConfig, proxies []config.ProxyConfig) (*OpenAIModel, error) {
	// 创建默认客户端
	defaultClient := &http.Client{
		Timeout: 600 * time.Second,
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
			Timeout:   600 * time.Second,
		}
	}

	return &OpenAIModel{
		BaseModel: BaseModel{
			config: cfg,
		},
		defaultClient: defaultClient,
		proxyClients:  proxyClients,
	}, nil
}

// GenerateResponse 生成响应，发送实际的API请求
func (m *OpenAIModel) GenerateResponse(ctx context.Context, systemMessage, userMessage string, stream bool) (*LLMResponse, error) {
	// 选择合适的HTTP客户端
	client := m.defaultClient

	// 如果模型配置了代理，并且代理客户端存在，则使用代理客户端
	if m.config.ProxyName != "" {
		if proxyClient, ok := m.proxyClients[m.config.ProxyName]; ok {
			client = proxyClient
			log.Printf("使用代理: %s", m.config.ProxyName)
		} else {
			log.Printf("未找到配置的代理: %s，使用默认客户端", m.config.ProxyName)
		}
	}

	// 构建请求
	reqBody := OpenAIRequest{
		Model: m.config.Params["model"].(string),
		Messages: []OpenAIMessage{
			{
				Role: "system",
				Text: systemMessage,
				// Content: []OpenAIMessageContent{
				// 	{
				// 		Type: "text",
				// 		Text: systemMessage,
				// 	},
				// },
			},
			{
				Role: "user",
				Text: userMessage,
				// Content: []OpenAIMessageContent{
				// 	{
				// 		Type: "text",
				// 		Text: userMessage,
				// 	},
				// },
			},
		},
		Temperature: m.config.Params["temperature"].(float64),
		MaxTokens:   int(m.config.Params["max_tokens"].(int)),
		Stream:      stream,
	}

	// 序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		m.config.BaseURL+"/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.config.APIKey))

	// 发送请求
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API请求失败: 状态码=%d, 响应=%s", resp.StatusCode, string(body))
	}

	// 初始化返回结果
	result := &LLMResponse{
		Content:      "",
		InputTokens:  0,
		OutputTokens: 0,
	}

	// 非流式响应处理
	if !stream {
		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		requestLatency := time.Since(startTime) // 对于非流式响应，在读取完整响应后测量延迟
		if err != nil {
			return nil, fmt.Errorf("读取响应体失败: %w", err)
		}

		// 打印请求延迟（可选，用于调试）
		log.Printf("OpenAI API请求延迟(非流式): %s", requestLatency)

		// 解析响应
		var openAIResp OpenAIResponse
		if err := json.Unmarshal(body, &openAIResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		// 构建返回结果
		result.InputTokens = openAIResp.Usage.PromptTokens
		result.OutputTokens = openAIResp.Usage.CompletionTokens

		// 提取内容
		if len(openAIResp.Choices) > 0 {
			content := openAIResp.Choices[0].Message.Content
			result.Content = content
		}
	} else {
		// 流式响应处理
		var fullContent string
		var tokenCount int
		var firstTokenReceived bool
		var firstTokenTime time.Duration
		var tokenStartTime time.Time

		// 创建一个新的reader
		reader := bufio.NewReader(resp.Body)

	LOOP:
		for {
			// 检查是否需要取消
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// 读取一行数据，格式是 data: {...}
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break LOOP
					}
					return nil, fmt.Errorf("读取流式响应失败: %w", err)
				}

				// 去除前缀和空行
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if line == "[DONE]" {
					break LOOP
				}

				// 解析JSON，前缀通常是 "data: "
				if strings.HasPrefix(line, "data: ") {
					dataJSON := strings.TrimPrefix(line, "data: ")
					if dataJSON == "" {
						continue
					}

					if dataJSON == "[DONE]" {
						break LOOP
					}

					var streamResp OpenAIStreamResponse
					if err := json.Unmarshal([]byte(dataJSON), &streamResp); err != nil {
						log.Printf("解析流响应块失败: %v, 数据: %s", err, dataJSON)
						continue
					}

					if streamResp.Usage != nil {
						tokenCount += streamResp.Usage.TotalTokens
						result.InputTokens += streamResp.Usage.PromptTokens
						result.OutputTokens += streamResp.Usage.CompletionTokens
					}

					// 累加内容
					if len(streamResp.Choices) > 0 {
						// 记录首个token接收时间
						if !firstTokenReceived {
							firstTokenReceived = true
							firstTokenTime = time.Since(startTime)
							tokenStartTime = time.Now()
						}

						content := streamResp.Choices[0].Delta.Content
						if content != "" {
							fullContent += content
						}
					}

				}
			}

		}

		// 流式响应结束，计算总延迟时间
		// 打印请求延迟（可选，用于调试）
		log.Printf("OpenAI API流式请求延迟(包含所有流式数据): %s", time.Since(startTime))

		// 设置流式响应结果
		result.Content = fullContent

		// 设置流式特定指标
		if firstTokenReceived {
			result.TimeToFirstToken = firstTokenTime
			if tokenCount > 0 {
				tokensPerSecond := float64(tokenCount) / time.Since(tokenStartTime).Seconds()
				result.TokensPerSecond = tokensPerSecond
				log.Printf("流式响应速率: %.2f tokens/sec", tokensPerSecond)
			}
		}

	}

	return result, nil
}
