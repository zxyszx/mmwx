package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/storage"
)

type ParkingHandler struct {
	repo *storage.TrafficRepository
}

func NewParkingHandler(repo *storage.TrafficRepository) http.Handler {
	if repo == nil {
		panic("parking handler requires repository")
	}
	return &ParkingHandler{repo: repo}
}

func (h *ParkingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/parking"), "/")
	switch path {
	case "summary":
		h.summary(w, r)
	case "platforms":
		h.platforms(w, r)
	case "platforms/update":
		h.updatePlatform(w, r)
	case "platforms/delete":
		h.deletePlatform(w, r)
	case "platforms/reorder":
		h.reorderPlatforms(w, r)
	case "spaces":
		h.spaces(w, r)
	case "spaces/update":
		h.updateSpace(w, r)
	case "spaces/password":
		h.spacePassword(w, r)
	case "spaces/reorder":
		h.reorderSpaces(w, r)
	case "members":
		h.members(w, r)
	case "members/update":
		h.updateMember(w, r)
	case "members/exit":
		h.exitMember(w, r)
	case "members/renew":
		h.renewMember(w, r)
	case "renewals":
		h.renewals(w, r)
	case "subscriptions":
		h.subscriptions(w, r)
	default:
		writeError(w, http.StatusNotFound, errors.New("parking endpoint not found"))
	}
}

func (h *ParkingHandler) reorderPlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.repo.ReorderParkingPlatforms(r.Context(), payload.IDs); err != nil {
		if errors.Is(err, storage.ErrParkingPlatformNotFound) || errors.Is(err, storage.ErrInvalidParkingOrder) {
			writeError(w, http.StatusBadRequest, errors.New("平台顺序包含无效项目"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *ParkingHandler) spacePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only GET is supported"))
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择车位"))
		return
	}
	password, err := h.repo.GetParkingSpacePassword(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrParkingSpaceNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"password": password})
}

func (h *ParkingHandler) platforms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		platforms, err := h.repo.ListParkingPlatforms(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"platforms": platforms})
	case http.MethodPost:
		var payload storage.ParkingPlatform
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(payload.Name) == "" {
			writeError(w, http.StatusBadRequest, errors.New("平台名称不能为空"))
			return
		}
		platform, err := h.repo.CreateParkingPlatform(r.Context(), payload)
		if err != nil {
			if errors.Is(err, storage.ErrInvalidPlatformDefaults) {
				writeError(w, http.StatusBadRequest, errors.New("默认车位数需为 1-100，随机密码位数需为 1-128"))
				return
			}
			if errors.Is(err, storage.ErrParkingPlatformExists) {
				writeError(w, http.StatusConflict, errors.New("平台名称已存在"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"platform": platform})
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (h *ParkingHandler) updatePlatform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload storage.ParkingPlatform
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.ID <= 0 || strings.TrimSpace(payload.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("平台名称不能为空"))
		return
	}
	platform, err := h.repo.UpdateParkingPlatform(r.Context(), payload)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidPlatformDefaults):
			writeError(w, http.StatusBadRequest, errors.New("默认车位数需为 1-100，随机密码位数需为 1-128"))
		case errors.Is(err, storage.ErrParkingPlatformNotFound):
			writeError(w, http.StatusNotFound, errors.New("平台不存在"))
		case errors.Is(err, storage.ErrParkingPlatformExists):
			writeError(w, http.StatusConflict, errors.New("平台名称已存在"))
		default:
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, map[string]any{"platform": platform})
}

func (h *ParkingHandler) deletePlatform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.repo.DeleteParkingPlatform(r.Context(), payload.ID); err != nil {
		switch {
		case errors.Is(err, storage.ErrParkingPlatformInUse):
			writeError(w, http.StatusConflict, errors.New("平台已有车位，不能删除"))
		case errors.Is(err, storage.ErrParkingPlatformNotFound):
			writeError(w, http.StatusNotFound, errors.New("平台不存在"))
		default:
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *ParkingHandler) reorderSpaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(payload.IDs) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("车位顺序不能为空"))
		return
	}
	if err := h.repo.ReorderParkingSpaces(r.Context(), payload.IDs); err != nil {
		if errors.Is(err, storage.ErrParkingSpaceNotFound) || errors.Is(err, storage.ErrInvalidParkingOrder) {
			writeError(w, http.StatusBadRequest, errors.New("车位顺序包含无效项目"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *ParkingHandler) updateSpace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload storage.ParkingSpace
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.ID <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择车位"))
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("车位名称不能为空"))
		return
	}
	if payload.PlatformID <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择平台"))
		return
	}
	space, err := h.repo.UpdateParkingSpace(r.Context(), payload)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrParkingSpaceNotFound):
			writeError(w, http.StatusNotFound, err)
			return
		case errors.Is(err, storage.ErrParkingSpaceFull):
			writeError(w, http.StatusBadRequest, errors.New("容量不能小于当前在位车友数"))
			return
		case errors.Is(err, storage.ErrInvalidParkingAmount):
			writeError(w, http.StatusBadRequest, errors.New("月费不能小于 0"))
			return
		case errors.Is(err, storage.ErrParkingPlatformNotFound):
			writeError(w, http.StatusBadRequest, errors.New("所选平台不存在"))
			return
		case errors.Is(err, storage.ErrParkingPlatformPaused):
			writeError(w, http.StatusBadRequest, errors.New("所选平台已暂停"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"space": space})
}

func (h *ParkingHandler) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only GET is supported"))
		return
	}
	summary, err := h.repo.ParkingSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"summary": summary})
}

func (h *ParkingHandler) updateMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload parkingMemberPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	member, err := payload.toStorage()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	member.ID = payload.ID
	updated, err := h.repo.UpdateParkingMember(r.Context(), member)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrParkingMemberNotFound):
			writeError(w, http.StatusNotFound, err)
			return
		case errors.Is(err, storage.ErrParkingSpaceNotFound):
			writeError(w, http.StatusBadRequest, errors.New("所选车位不存在"))
			return
		case errors.Is(err, storage.ErrParkingSpacePaused):
			writeError(w, http.StatusBadRequest, errors.New("所选车位已暂停，不能分配车友"))
			return
		case errors.Is(err, storage.ErrParkingSpaceFull):
			writeError(w, http.StatusBadRequest, errors.New("所选车位已满，不能继续添加车友"))
			return
		case errors.Is(err, storage.ErrParkingSlotOccupied):
			writeError(w, http.StatusConflict, errors.New("该席位已有车友"))
			return
		case errors.Is(err, storage.ErrInvalidParkingSlot):
			writeError(w, http.StatusBadRequest, errors.New("位号超出车位容量"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"member": updated})
}

func (h *ParkingHandler) exitMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload struct {
		MemberID int64 `json:"member_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.MemberID <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择车友"))
		return
	}
	member, err := h.repo.ExitParkingMember(r.Context(), payload.MemberID)
	if err != nil {
		if errors.Is(err, storage.ErrParkingMemberNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"member": member})
}

func (h *ParkingHandler) spaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		spaces, err := h.repo.ListParkingSpaces(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"spaces": spaces})
	case http.MethodPost:
		var payload storage.ParkingSpace
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(payload.Name) == "" {
			writeError(w, http.StatusBadRequest, errors.New("车位名称不能为空"))
			return
		}
		if payload.PlatformID <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("请选择平台"))
			return
		}
		space, err := h.repo.CreateParkingSpace(r.Context(), payload)
		if err != nil {
			if errors.Is(err, storage.ErrInvalidParkingAmount) {
				writeError(w, http.StatusBadRequest, errors.New("月费不能小于 0"))
				return
			}
			if errors.Is(err, storage.ErrParkingPlatformNotFound) {
				writeError(w, http.StatusBadRequest, errors.New("所选平台不存在"))
				return
			}
			if errors.Is(err, storage.ErrParkingPlatformPaused) {
				writeError(w, http.StatusBadRequest, errors.New("所选平台已暂停"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"space": space})
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (h *ParkingHandler) members(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		members, err := h.repo.ListParkingMembers(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"members": members})
	case http.MethodPost:
		var payload parkingMemberPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		member, err := payload.toStorage()
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		created, err := h.repo.CreateParkingMember(r.Context(), member)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrParkingSpaceNotFound):
				writeError(w, http.StatusBadRequest, errors.New("所选车位不存在"))
				return
			case errors.Is(err, storage.ErrParkingSpacePaused):
				writeError(w, http.StatusBadRequest, errors.New("所选车位已暂停，不能分配车友"))
				return
			case errors.Is(err, storage.ErrParkingSpaceFull):
				writeError(w, http.StatusBadRequest, errors.New("所选车位已满，不能继续添加车友"))
				return
			case errors.Is(err, storage.ErrParkingSlotOccupied):
				writeError(w, http.StatusConflict, errors.New("该席位已有车友"))
				return
			case errors.Is(err, storage.ErrInvalidParkingSlot):
				writeError(w, http.StatusBadRequest, errors.New("位号超出车位容量"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"member": created})
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (h *ParkingHandler) renewMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only POST is supported"))
		return
	}
	var payload struct {
		MemberID      int64   `json:"member_id"`
		Months        int     `json:"months"`
		Amount        float64 `json:"amount"`
		Payment       string  `json:"payment"`
		Note          string  `json:"note"`
		NewExpireDate string  `json:"new_expire_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.MemberID <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("请选择车友"))
		return
	}
	operator := auth.UsernameFromContext(r.Context())
	var requestedExpire time.Time
	if strings.TrimSpace(payload.NewExpireDate) != "" {
		var parseErr error
		requestedExpire, parseErr = parseParkingDate(payload.NewExpireDate, time.Time{})
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, errors.New("新到期时间格式不正确"))
			return
		}
	}
	renewal, err := h.repo.RenewParkingMemberTo(r.Context(), payload.MemberID, payload.Months, payload.Amount, payload.Payment, operator, payload.Note, requestedExpire)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrParkingMemberNotFound):
			writeError(w, http.StatusNotFound, err)
			return
		case errors.Is(err, storage.ErrInvalidParkingMonths):
			writeError(w, http.StatusBadRequest, errors.New("续费月数应为 1 到 120 个月"))
			return
		case errors.Is(err, storage.ErrInvalidParkingAmount):
			writeError(w, http.StatusBadRequest, errors.New("续费金额不能小于 0"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"renewal": renewal})
}

func (h *ParkingHandler) renewals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("only GET is supported"))
		return
	}
	renewals, err := h.repo.ListParkingRenewals(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"renewals": renewals})
}

func (h *ParkingHandler) subscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.repo.ListParkingSubscriptions(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"subscriptions": items})
	case http.MethodPost:
		var payload struct {
			ServiceName   string  `json:"service_name"`
			AccountName   string  `json:"account_name"`
			StartDate     string  `json:"start_date"`
			ExpireDate    string  `json:"expire_date"`
			RenewalMonths int     `json:"renewal_months"`
			Amount        float64 `json:"amount"`
			ReminderDays  int     `json:"reminder_days"`
			Note          string  `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(payload.ServiceName) == "" {
			writeError(w, http.StatusBadRequest, errors.New("服务名称不能为空"))
			return
		}
		startDate, err := parseParkingDate(payload.StartDate, time.Time{})
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("开始时间格式不正确"))
			return
		}
		expireDate, err := parseParkingDate(payload.ExpireDate, time.Time{})
		if err != nil || expireDate.Before(startDate) {
			writeError(w, http.StatusBadRequest, errors.New("到期时间格式不正确或早于开始时间"))
			return
		}
		if payload.RenewalMonths < 1 || payload.RenewalMonths > 120 || payload.Amount < 0 || payload.ReminderDays < 0 || payload.ReminderDays > 365 {
			writeError(w, http.StatusBadRequest, errors.New("续费周期、金额或提醒天数无效"))
			return
		}
		created, err := h.repo.CreateParkingSubscription(r.Context(), storage.ParkingSubscription{
			ServiceName: payload.ServiceName, AccountName: payload.AccountName,
			StartDate: startDate, ExpireDate: expireDate, RenewalMonths: payload.RenewalMonths,
			Amount: payload.Amount, ReminderDays: payload.ReminderDays, Note: payload.Note,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{"subscription": created})
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

type parkingMemberPayload struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Contact     string  `json:"contact"`
	ContactType string  `json:"contact_type"`
	Telegram    string  `json:"telegram"`
	CarPlate    string  `json:"car_plate"`
	SpaceID     int64   `json:"space_id"`
	SlotLabel   string  `json:"slot_label"`
	StartDate   string  `json:"start_date"`
	ExpireDate  string  `json:"expire_date"`
	Status      string  `json:"status"`
	Amount      float64 `json:"amount"`
	Payment     string  `json:"payment"`
	Note        string  `json:"note"`
}

func (p parkingMemberPayload) toStorage() (storage.ParkingMember, error) {
	if strings.TrimSpace(p.Name) == "" {
		return storage.ParkingMember{}, errors.New("车友名称不能为空")
	}
	if p.SpaceID <= 0 {
		return storage.ParkingMember{}, errors.New("请选择车位")
	}
	if p.Amount < 0 {
		return storage.ParkingMember{}, errors.New("月费不能小于 0")
	}
	start, err := parseParkingDate(p.StartDate, time.Now())
	if err != nil {
		return storage.ParkingMember{}, err
	}
	expireDefault := start.AddDate(0, 1, 0)
	expire, err := parseParkingDate(p.ExpireDate, expireDefault)
	if err != nil {
		return storage.ParkingMember{}, err
	}
	if expire.Before(start) {
		return storage.ParkingMember{}, errors.New("到期时间不能早于开始时间")
	}
	return storage.ParkingMember{
		Name:        p.Name,
		Contact:     p.Contact,
		ContactType: p.ContactType,
		Telegram:    p.Telegram,
		CarPlate:    p.CarPlate,
		SpaceID:     p.SpaceID,
		SlotLabel:   p.SlotLabel,
		StartDate:   start,
		ExpireDate:  expire,
		Status:      p.Status,
		Amount:      p.Amount,
		Payment:     p.Payment,
		Note:        p.Note,
	}, nil
}

func parseParkingDate(value string, fallback time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unix, 0), nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, errors.New("日期格式应为 YYYY-MM-DD")
	}
	return date, nil
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
