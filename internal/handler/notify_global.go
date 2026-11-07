package handler

import "miaomiaowu/internal/notify"

var globalNotifier *notify.Notifier

// InitNotifier initializes the global notifier with the given config.
func InitNotifier(cfg notify.Config) {
	globalNotifier = notify.New(cfg)
}

// GetNotifier returns the global notifier instance.
func GetNotifier() *notify.Notifier {
	return globalNotifier
}
