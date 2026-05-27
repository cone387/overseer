package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/overseer/overseer/internal/model"
)

const scheduleParsePrompt = `你是一个日程解析助手。用户会用自然语言描述一个定时提醒，你需要从中提取出结构化的调度配置。

当前时间: %s
时区: %s

请将用户输入解析为以下 JSON 格式（只返回 JSON，不要其他内容）：

{
  "title": "提醒标题（从用户输入中提取）",
  "schedule": {
    "type": "调度类型",
    "config": { ... },
    "timezone": "时区"
  }
}

支持的调度类型和对应 config：
- "once": {"datetime": "2024-01-15T09:00:00+08:00"} — 一次性，指定具体时间
- "interval": {"period": 3600} — 间隔（秒）
- "daily": {"time": "09:00:00"} — 每天指定时间
- "weekly": {"days": [1,3,5], "time": "09:00:00"} — 每周指定天（1=周一,7=周日）
- "monthly": {"days": [1,15], "time": "09:00:00"} — 每月指定日
- "yearly": {"month": 1, "day": 15, "time": "09:00:00"} — 每年指定日期
- "workday": {"time": "09:00:00"} — 工作日（周一到周五）
- "weekend": {"time": "09:00:00"} — 周末（周六周日）
- "crontab": {"expression": "0 9 * * 1-5"} — cron 表达式

规则：
1. 如果用户说"明天早上9点"，用 once 类型，datetime 设为明天 09:00
2. 如果用户说"每天早上9点"，用 daily 类型
3. 如果用户说"每周一三五下午3点"，用 weekly 类型，days=[1,3,5]
4. 如果用户说"工作日早上9点"，用 workday 类型
5. 如果用户说"每隔2小时"，用 interval 类型，period=7200
6. 时间格式统一用 HH:MM:SS
7. title 从用户输入中提取核心提醒内容，如"提醒我开会"→"开会"
8. 如果无法确定具体时间，使用合理的默认值`

// ParseResult is the result of parsing natural language into a schedule.
type ParseResult struct {
	Title    string               `json:"title"`
	Schedule model.ScheduleConfig `json:"schedule"`
}

// ParseSchedule uses LLM to parse natural language into a ScheduleConfig.
func ParseSchedule(ctx context.Context, client *Client, input string, timezone string) (*ParseResult, error) {
	if !client.IsConfigured() {
		return nil, fmt.Errorf("LLM not configured: missing API key")
	}

	if timezone == "" {
		timezone = "Asia/Shanghai"
	}

	now := time.Now().In(mustLoadLocation(timezone))
	prompt := fmt.Sprintf(scheduleParsePrompt, now.Format("2006-01-02 15:04:05"), timezone)

	messages := []ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: input},
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Clean response (remove markdown code blocks if present)
	response = cleanJSON(response)

	var result ParseResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse: %s", err, response)
	}

	// Validate the schedule type
	if !model.ValidScheduleTypes[result.Schedule.Type] {
		return nil, fmt.Errorf("invalid schedule type from LLM: %s", result.Schedule.Type)
	}

	// Set timezone and raw input
	result.Schedule.Timezone = timezone
	result.Schedule.RawInput = input

	return &result, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown code block markers
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
