package handler

import (
	"strings"
	"testing"
	"time"

	"miaomiaowu/internal/storage"
)

func TestParkingExpiryLines(t *testing.T) {
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.Local)
	members := []storage.ParkingMember{
		{Name: "已过期", SpaceName: "A区08号", SlotLabel: "1号位", CarPlate: "沪A11111", Telegram: "@old", ExpireDate: now.AddDate(0, 0, -2), Status: "active"},
		{Name: "今日", SpaceName: "A区09号", Contact: "微信 today", ExpireDate: now, Status: "active"},
		{Name: "七天", SpaceName: "B区_01号", Telegram: "@qa_user", ExpireDate: now.AddDate(0, 0, 7), Status: "active"},
		{Name: "三十天", SpaceName: "B区02号", ExpireDate: now.AddDate(0, 0, 30), Status: "active"},
		{Name: "三十一天", SpaceName: "C区01号", ExpireDate: now.AddDate(0, 0, 31), Status: "active"},
		{Name: "已退出", SpaceName: "C区02号", ExpireDate: now.AddDate(0, 0, -1), Status: "exited"},
	}

	lines := parkingExpiryLines(members, now)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"已过期 2 天", "今日到期", "7 天后到期", "30 天后到期"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in lines:\n%s", want, joined)
		}
	}
	for _, want := range []string{"B区\\_01号", "@qa\\_user"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected markdown escaped %q in lines:\n%s", want, joined)
		}
	}
	for _, notWant := range []string{"三十一天", "已退出"} {
		if strings.Contains(joined, notWant) {
			t.Fatalf("did not expect %q in lines:\n%s", notWant, joined)
		}
	}
}
