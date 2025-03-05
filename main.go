package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lemonlinger/llm-test/config"
	"github.com/lemonlinger/llm-test/engine"
	"github.com/lemonlinger/llm-test/model"
	"github.com/lemonlinger/llm-test/report"
)

func main() {
	// 解析命令行参数
	configFile := flag.String("config", "config.yaml", "配置文件路径")
	concurrency := flag.Int("concurrency", 0, "并发数 (覆盖配置文件)")
	duration := flag.Duration("duration", 0, "测试持续时间 (覆盖配置文件)")
	outputFormat := flag.String("output", "text", "输出格式: text, json, csv")

	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 如果命令行参数指定了值，则覆盖配置文件中的设置
	if *concurrency > 0 {
		cfg.Test.Concurrency = *concurrency
	}
	if *duration > 0 {
		cfg.Test.Duration = *duration
	}

	// 初始化模型
	models, err := model.InitializeModels(cfg.Models, cfg.Proxies)
	if err != nil {
		log.Fatalf("初始化模型失败: %v", err)
	}

	// 选择合适的提示词配置
	// 否则使用常规配置
	promptConfig := cfg.Prompt
	fmt.Println("开始LLM API性能测试")
	fmt.Printf("测试配置: 并发数=%d, 持续时间=%s, 流式测试=%v\n",
		cfg.Test.Concurrency,
		cfg.Test.Duration.String(),
		promptConfig.Stream)
	fmt.Printf("测试模型: %v\n", getModelNames(models))

	// 创建并启动测试引擎
	testEngine := engine.NewTestEngine(cfg.Test, models, promptConfig, cfg.Proxies)
	results, err := testEngine.Run()
	if err != nil {
		log.Fatalf("测试执行失败: %v", err)
	}

	// 生成报告
	reporter := report.NewReporter(*outputFormat)
	reportContent, err := reporter.GenerateReport(results)
	if err != nil {
		log.Fatalf("生成报告失败: %v", err)
	}

	// 输出报告
	fmt.Println("\n测试结果:")
	fmt.Println(reportContent)

	// 保存报告到文件
	reportFile := fmt.Sprintf("llm_test_report_%s_%s.%s",
		time.Now().Format("20060102_150405"),
		map[bool]string{true: "stream", false: "standard"}[promptConfig.Stream],
		*outputFormat)
	err = os.WriteFile(reportFile, []byte(reportContent), 0644)
	if err != nil {
		log.Printf("保存报告失败: %v", err)
	} else {
		fmt.Printf("报告已保存至: %s\n", reportFile)
	}
}

func getModelNames(models []model.LLMModel) []string {
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.GetName()
	}
	return names
}
