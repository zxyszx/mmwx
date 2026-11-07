package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"miaomiaowu/internal/logger"
	"miaomiaowu/internal/notify"
	"miaomiaowu/internal/storage"
)

func StartNotifyScheduler(ctx context.Context, repo *storage.TrafficRepository, trafficHandler *TrafficSummaryHandler) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	var lastDailyRun string

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			n := GetNotifier()
			if n == nil {
				continue
			}
			cfg := n.GetConfig()

			if cfg.NotifyDailyTraffic {
				today := now.Format("2006-01-02")
				nowTime := now.Format("15:04")
				targetTime := cfg.DailyTrafficTime
				if targetTime == "" {
					targetTime = "08:00"
				}
				if nowTime == targetTime && lastDailyRun != today {
					lastDailyRun = today
					go sendDailyTrafficNotification(ctx, trafficHandler, n)
				}
			}

			if cfg.NotifyExpiry && now.Format("15:04") == "09:00" {
				go sendExpiryNotification(ctx, repo, n)
			}
		}
	}
}

func sendDailyTrafficNotification(ctx context.Context, th *TrafficSummaryHandler, n *notify.Notifier) {
	totalLimitGB, totalUsedGB, probeServers, extSubs, err := th.FetchTrafficSummaryForNotify(ctx)
	if err != nil {
		logger.Warn("[Notify] 获取流量数据失败", "error", err)
		return
	}

	if totalLimitGB == 0 && totalUsedGB == 0 && len(probeServers) == 0 && len(extSubs) == 0 {
		return
	}

	var b strings.Builder

	pct := 0.0
	if totalLimitGB > 0 {
		pct = totalUsedGB / totalLimitGB * 100
	}
	remainGB := totalLimitGB - totalUsedGB
	if remainGB < 0 {
		remainGB = 0
	}
	fmt.Fprintf(&b, "总计: %.2f / %.2f GB (%.1f%%)\n剩余: %.2f GB", totalUsedGB, totalLimitGB, pct, remainGB)

	if len(probeServers) > 0 {
		b.WriteString("\n\n— 服务器 —")
		for _, s := range probeServers {
			fmt.Fprintf(&b, "\n• %s: %.2f / %.2f GB", s.Name, s.UsedGB, s.LimitGB)
		}
	}

	if len(extSubs) > 0 {
		b.WriteString("\n\n— 外部订阅 —")
		for _, s := range extSubs {
			fmt.Fprintf(&b, "\n• %s: %.2f / %.2f GB", s.Name, s.UsedGB, s.LimitGB)
		}
	}

	_ = n.Send(ctx, notify.Event{
		Type:    notify.EventDailyTraffic,
		Title:   "每日流量统计",
		Message: b.String(),
	})
}

func sendExpiryNotification(ctx context.Context, repo *storage.TrafficRepository, n *notify.Notifier) {
	files, err := repo.ListSubscribeFiles(ctx)
	if err != nil {
		logger.Warn("[Notify] 获取订阅文件失败", "error", err)
	}

	now := time.Now()
	threeDaysLater := now.Add(3 * 24 * time.Hour)

	var sections []string
	var subscriptionLines []string
	if err == nil {
		for _, f := range files {
			if f.ExpireAt == nil {
				continue
			}
			if f.ExpireAt.After(now) && f.ExpireAt.Before(threeDaysLater) {
				days := int(f.ExpireAt.Sub(now).Hours() / 24)
				subscriptionLines = append(subscriptionLines, fmt.Sprintf("• %s: %d 天后到期", f.Name, days))
			}
		}
	}
	if len(subscriptionLines) > 0 {
		sections = append(sections, "— 订阅到期 —\n"+strings.Join(subscriptionLines, "\n"))
	}

	members, err := repo.ListParkingMembers(ctx)
	if err != nil {
		logger.Warn("[Notify] 获取车友到期数据失败", "error", err)
	} else if lines := parkingExpiryLines(members, now); len(lines) > 0 {
		sections = append(sections, "— 车友到期 —\n"+strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return
	}

	msg := strings.Join(sections, "\n\n")
	_ = n.Send(ctx, notify.Event{
		Type:    notify.EventExpiry,
		Title:   "到期提醒",
		Message: msg,
	})
}

func parkingExpiryLines(members []storage.ParkingMember, now time.Time) []string {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var lines []string
	for _, member := range members {
		if member.Status != "active" {
			continue
		}
		expire := time.Date(member.ExpireDate.Year(), member.ExpireDate.Month(), member.ExpireDate.Day(), 0, 0, 0, 0, start.Location())
		days := int(expire.Sub(start).Hours() / 24)
		if days > 30 {
			continue
		}

		status := ""
		switch {
		case days < 0:
			status = fmt.Sprintf("已过期 %d 天", -days)
		case days == 0:
			status = "今日到期"
		default:
			status = fmt.Sprintf("%d 天后到期", days)
		}

		space := strings.TrimSpace(strings.Join([]string{member.SpaceName, member.SlotLabel}, " "))
		if space == "" {
			space = "未填车位"
		}
		contact := strings.TrimSpace(member.Telegram)
		if contact == "" {
			contact = strings.TrimSpace(member.Contact)
		}
		if contact == "" {
			contact = "未填联系方式"
		}
		carPlate := strings.TrimSpace(member.CarPlate)
		if carPlate == "" {
			carPlate = "未填车牌"
		}

		lines = append(lines, fmt.Sprintf("• %s（%s，%s）: %s，联系 %s",
			telegramMarkdownText(member.Name),
			telegramMarkdownText(space),
			telegramMarkdownText(carPlate),
			status,
			telegramMarkdownText(contact),
		))
	}
	return lines
}

func telegramMarkdownText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"`", "\\`",
	)
	return replacer.Replace(value)
}
