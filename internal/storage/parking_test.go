package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestParkingLifecycle(t *testing.T) {
	ctx := context.Background()
	repo, err := NewTrafficRepository(filepath.Join(t.TempDir(), "traffic.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	defer repo.Close()

	platform, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{
		Name: "Netflix",
		Note: "流媒体平台",
	})
	if err != nil {
		t.Fatalf("create parking platform: %v", err)
	}
	if platform.ID == 0 || platform.Name != "Netflix" {
		t.Fatalf("unexpected platform: %+v", platform)
	}
	if _, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "netflix"}); !errors.Is(err, ErrParkingPlatformExists) {
		t.Fatalf("expected duplicate platform error, got %v", err)
	}
	paused, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "Paused", Status: "paused"})
	if err != nil {
		t.Fatalf("create paused platform: %v", err)
	}
	if _, err := repo.CreateParkingSpace(ctx, ParkingSpace{Name: "blocked", PlatformID: paused.ID}); !errors.Is(err, ErrParkingPlatformPaused) {
		t.Fatalf("expected paused platform error, got %v", err)
	}

	space, err := repo.CreateParkingSpace(ctx, ParkingSpace{
		Name:         "A区08号",
		Password:     "stream-secret",
		PlatformID:   platform.ID,
		BillingDay:   18,
		CardLast4:    "6109",
		Location:     "地下二层",
		Tags:         []string{"固定车位", "月付", "月付"},
		TotalSlots:   2,
		MonthlyPrice: 120,
	})
	if err != nil {
		t.Fatalf("create parking space: %v", err)
	}
	if space.ID == 0 || space.CreatedAt.IsZero() || space.Platform != "Netflix" || !space.PasswordSet || space.BillingDay != 18 || space.CardLast4 != "6109" || space.SlotNumber != 1 {
		t.Fatalf("expected persisted parking space with timestamps, got %+v", space)
	}
	password, err := repo.GetParkingSpacePassword(ctx, space.ID)
	if err != nil || password != "stream-secret" {
		t.Fatalf("reveal parking password: password=%q err=%v", password, err)
	}
	var storedPassword string
	if err := repo.db.QueryRow(`SELECT password_cipher FROM parking_spaces WHERE id = ?`, space.ID).Scan(&storedPassword); err != nil {
		t.Fatalf("read encrypted password: %v", err)
	}
	if storedPassword == "stream-secret" || storedPassword == "" {
		t.Fatalf("password was not encrypted at rest: %q", storedPassword)
	}
	if len(space.Tags) != 2 || space.Tags[0] != "固定车位" || space.Tags[1] != "月付" {
		t.Fatalf("expected normalized parking tags, got %+v", space.Tags)
	}
	space.Note = "已调整价格"
	space.MonthlyPrice = 138
	space.Tags = []string{"季度", "靠近电梯"}
	updatedSpace, err := repo.UpdateParkingSpace(ctx, space)
	if err != nil {
		t.Fatalf("update parking space: %v", err)
	}
	if updatedSpace.MonthlyPrice != 138 || updatedSpace.Note != "已调整价格" || len(updatedSpace.Tags) != 2 {
		t.Fatalf("unexpected updated parking space: %+v", updatedSpace)
	}
	if err := repo.DeleteParkingPlatform(ctx, platform.ID); !errors.Is(err, ErrParkingPlatformInUse) {
		t.Fatalf("expected platform in use error, got %v", err)
	}

	member, err := repo.CreateParkingMember(ctx, ParkingMember{
		Name:        "张三",
		Contact:     "微信 zhangsan",
		ContactType: "wechat",
		Telegram:    "@zhangsan",
		CarPlate:    "沪A12345",
		SpaceID:     space.ID,
		SlotLabel:   "1号位",
		StartDate:   time.Now(),
		ExpireDate:  time.Now().AddDate(0, 1, 0),
		Amount:      120,
		Payment:     "微信",
	})
	if err != nil {
		t.Fatalf("create parking member: %v", err)
	}
	if member.RenewalCount != 1 || member.ContactType != "wechat" {
		t.Fatalf("expected initial renewal and contact type, got %+v", member)
	}
	member.CarPlate = "沪B67890"
	updatedMember, err := repo.UpdateParkingMember(ctx, member)
	if err != nil {
		t.Fatalf("update parking member: %v", err)
	}
	if updatedMember.CarPlate != "沪B67890" {
		t.Fatalf("unexpected updated parking member: %+v", updatedMember)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{
		Name:       "李四",
		SpaceID:    space.ID,
		StartDate:  time.Now(),
		ExpireDate: time.Now().AddDate(0, 1, 0),
		Status:     "active",
	}); err != nil {
		t.Fatalf("create second parking member: %v", err)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{
		Name:       "王五",
		SpaceID:    space.ID,
		StartDate:  time.Now(),
		ExpireDate: time.Now().AddDate(0, 1, 0),
		Status:     "active",
	}); !errors.Is(err, ErrParkingSpaceFull) {
		t.Fatalf("expected full parking space error, got %v", err)
	}
	space.TotalSlots = 1
	if _, err := repo.UpdateParkingSpace(ctx, space); !errors.Is(err, ErrParkingSpaceFull) {
		t.Fatalf("expected shrink below active members to fail, got %v", err)
	}

	renewal, err := repo.RenewParkingMember(ctx, member.ID, 1, 120, "微信", "admin", "测试续费")
	if err != nil {
		t.Fatalf("renew parking member: %v", err)
	}
	if renewal.NewExpireDate.Before(renewal.OldExpireDate) {
		t.Fatalf("expected renewal to extend expiry, got %+v", renewal)
	}
	customExpire := renewal.NewExpireDate.AddDate(0, 2, 0)
	customRenewal, err := repo.RenewParkingMemberTo(ctx, member.ID, 0, 0, "微信", "admin", "自定义到期", customExpire)
	if err != nil || !customRenewal.NewExpireDate.Equal(customExpire) {
		t.Fatalf("expected custom renewal date, got %+v err=%v", customRenewal, err)
	}
	if _, err := repo.RenewParkingMember(ctx, member.ID, 0, 120, "微信", "admin", ""); !errors.Is(err, ErrInvalidParkingMonths) {
		t.Fatalf("expected invalid renewal months error, got %v", err)
	}
	if _, err := repo.RenewParkingMember(ctx, member.ID, 1, -1, "微信", "admin", ""); !errors.Is(err, ErrInvalidParkingAmount) {
		t.Fatalf("expected invalid renewal amount error, got %v", err)
	}

	summary, err := repo.ParkingSummary(ctx)
	if err != nil {
		t.Fatalf("parking summary: %v", err)
	}
	if summary.TotalSlots != 2 || summary.ActiveMembers != 2 || summary.AvailableSlots != 0 || summary.MonthlyRevenue != 240 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	exited, err := repo.ExitParkingMember(ctx, member.ID)
	if err != nil {
		t.Fatalf("exit parking member: %v", err)
	}
	if exited.Status != "exited" {
		t.Fatalf("expected exited member, got %+v", exited)
	}
}

func TestParkingPlatformAndSpaceOrdering(t *testing.T) {
	ctx := context.Background()
	repo, err := NewTrafficRepository(filepath.Join(t.TempDir(), "traffic.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	defer repo.Close()

	platform, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "Netflix", Icon: "data:image/webp;base64,test"})
	if err != nil {
		t.Fatalf("create platform: %v", err)
	}
	if platform.DefaultSlots != 1 || platform.PasswordLength != 8 {
		t.Fatalf("unexpected defaults: %+v", platform)
	}
	if platform.Icon != "data:image/webp;base64,test" || platform.SortOrder != 1 {
		t.Fatalf("expected icon and initial order, got %+v", platform)
	}
	secondPlatform, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "ChatGPT"})
	if err != nil {
		t.Fatalf("create second platform: %v", err)
	}
	if err := repo.ReorderParkingPlatforms(ctx, []int64{secondPlatform.ID, platform.ID}); err != nil {
		t.Fatalf("reorder platforms: %v", err)
	}
	if err := repo.ReorderParkingPlatforms(ctx, []int64{platform.ID}); !errors.Is(err, ErrInvalidParkingOrder) {
		t.Fatalf("expected incomplete platform order to fail, got %v", err)
	}
	platforms, err := repo.ListParkingPlatforms(ctx)
	if err != nil || len(platforms) != 2 || platforms[0].ID != secondPlatform.ID || platforms[1].ID != platform.ID {
		t.Fatalf("unexpected platform order: %+v err=%v", platforms, err)
	}
	if _, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "invalid", DefaultSlots: 101}); !errors.Is(err, ErrInvalidPlatformDefaults) {
		t.Fatalf("expected invalid seat default, got %v", err)
	}
	if _, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "invalid", PasswordLength: 129}); !errors.Is(err, ErrInvalidPlatformDefaults) {
		t.Fatalf("expected invalid password length, got %v", err)
	}
	first, err := repo.CreateParkingSpace(ctx, ParkingSpace{Name: "account-a", PlatformID: platform.ID, TotalSlots: 1})
	if err != nil {
		t.Fatalf("create first space: %v", err)
	}
	second, err := repo.CreateParkingSpace(ctx, ParkingSpace{Name: "account-b", PlatformID: platform.ID, TotalSlots: 2})
	if err != nil {
		t.Fatalf("create second space: %v", err)
	}
	if err := repo.ReorderParkingSpaces(ctx, []int64{second.ID, first.ID}); err != nil {
		t.Fatalf("reorder spaces: %v", err)
	}
	if err := repo.ReorderParkingSpaces(ctx, []int64{first.ID, first.ID}); !errors.Is(err, ErrInvalidParkingOrder) {
		t.Fatalf("expected duplicate order to fail, got %v", err)
	}
	if err := repo.ReorderParkingSpaces(ctx, []int64{first.ID}); !errors.Is(err, ErrInvalidParkingOrder) {
		t.Fatalf("expected incomplete order to fail, got %v", err)
	}
	spaces, err := repo.ListParkingSpaces(ctx)
	if err != nil {
		t.Fatalf("list spaces: %v", err)
	}
	if len(spaces) != 2 || spaces[0].ID != second.ID || spaces[1].ID != first.ID {
		t.Fatalf("unexpected space order: %+v", spaces)
	}

	platform.Name = "Netflix Premium"
	platform.DefaultSlots = 6
	platform.PasswordLength = 12
	updated, err := repo.UpdateParkingPlatform(ctx, platform)
	if err != nil {
		t.Fatalf("update platform: %v", err)
	}
	if updated.Name != "Netflix Premium" || updated.DefaultSlots != 6 || updated.PasswordLength != 12 {
		t.Fatalf("unexpected updated platform: %+v", updated)
	}
	spaces, err = repo.ListParkingSpaces(ctx)
	if err != nil {
		t.Fatalf("list renamed spaces: %v", err)
	}
	if spaces[0].Platform != "Netflix Premium" || spaces[1].Platform != "Netflix Premium" {
		t.Fatalf("platform rename did not propagate: %+v", spaces)
	}
}

func TestParkingMemberSlotAssignment(t *testing.T) {
	ctx := context.Background()
	repo, err := NewTrafficRepository(filepath.Join(t.TempDir(), "traffic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	platform, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "Netflix"})
	if err != nil {
		t.Fatal(err)
	}
	space, err := repo.CreateParkingSpace(ctx, ParkingSpace{Name: "account", PlatformID: platform.ID, TotalSlots: 3})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "first", SpaceID: space.ID, SlotLabel: "2号位"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "duplicate", SpaceID: space.ID, SlotLabel: "2号位"}); !errors.Is(err, ErrParkingSlotOccupied) {
		t.Fatalf("expected occupied slot error, got %v", err)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "invalid", SpaceID: space.ID, SlotLabel: "4号位"}); !errors.Is(err, ErrInvalidParkingSlot) {
		t.Fatalf("expected invalid slot error, got %v", err)
	}
	second, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "second", SpaceID: space.ID})
	if err != nil || second.SlotLabel != "1号位" {
		t.Fatalf("expected first free slot, got %+v, %v", second, err)
	}
	first.SlotLabel = "1号位"
	if _, err := repo.UpdateParkingMember(ctx, first); !errors.Is(err, ErrParkingSlotOccupied) {
		t.Fatalf("expected occupied slot on update, got %v", err)
	}
	if _, err := repo.ExitParkingMember(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	first.SlotLabel = "1号位"
	if _, err := repo.UpdateParkingMember(ctx, first); err != nil {
		t.Fatalf("expected exited member slot to be reusable: %v", err)
	}
}

func TestParkingMemberLegacyLabelAssignment(t *testing.T) {
	ctx := context.Background()
	repo, err := NewTrafficRepository(filepath.Join(t.TempDir(), "traffic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	platform, err := repo.CreateParkingPlatform(ctx, ParkingPlatform{Name: "Netflix"})
	if err != nil {
		t.Fatal(err)
	}
	space, err := repo.CreateParkingSpace(ctx, ParkingSpace{Name: "account", PlatformID: platform.ID, TotalSlots: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "legacy", SpaceID: space.ID, SlotLabel: "朋友"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "numbered", SpaceID: space.ID, SlotLabel: "1号位"}); err != nil {
		t.Fatalf("explicit slot should take precedence over legacy label: %v", err)
	}
	last, err := repo.CreateParkingMember(ctx, ParkingMember{Name: "last", SpaceID: space.ID})
	if err != nil || last.SlotLabel != "3号位" {
		t.Fatalf("expected remaining slot 3, got %+v, %v", last, err)
	}
}
