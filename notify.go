package main

// Desktop notifications over the session bus.
//
// Shelling out to notify-send cannot dismiss a notification once it is up, and
// an unanswered fingerprint prompt has to disappear the moment it is answered.
// Speaking to org.freedesktop.Notifications directly gives us the id back, so a
// prompt can replace the previous one instead of stacking and can be closed
// when the scan finishes.

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	notifyService = "org.freedesktop.Notifications"
	notifyPath    = "/org/freedesktop/Notifications"
	notifyIface   = "org.freedesktop.Notifications"

	urgencyNormal   = byte(1)
	urgencyCritical = byte(2)

	expireDefault = int32(5000) // ms
	expireNever   = int32(0)
)

var (
	notifyMu sync.Mutex
	// promptID is the id of the live fingerprint prompt. Reusing it means a
	// second request replaces the first banner rather than adding to a pile
	// that never expires.
	promptID uint32
)

// post sends a notification and returns its id, or 0 if the desktop has no
// notification service. Failures are swallowed: losing a banner must never
// fail an authentication.
func post(summary, body string, urgency byte, expire int32, replaces uint32) uint32 {
	conn, err := dbus.SessionBus()
	if err != nil {
		return 0
	}
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(urgency),
		// Collapse repeats on daemons that honour a stack tag, which covers
		// the case where a replaces-id is not enough.
		"x-dunst-stack-tag": dbus.MakeVariant("llavero"),
	}
	var id uint32
	call := conn.Object(notifyService, notifyPath).Call(
		notifyIface+".Notify", 0,
		"Llavero", replaces, "", summary, body, []string{}, hints, expire,
	)
	if call.Err != nil {
		return 0
	}
	if err := call.Store(&id); err != nil {
		return 0
	}
	return id
}

// notify posts a transient, self-expiring message.
func notify(summary, body string) {
	post(summary, body, urgencyNormal, expireDefault, 0)
}

// notifyPrompt raises the sticky "touch the sensor" banner. It replaces any
// previous prompt so repeated requests cannot stack.
func notifyPrompt(summary, body string) {
	notifyMu.Lock()
	defer notifyMu.Unlock()
	promptID = post(summary, body, urgencyCritical, expireNever, promptID)
}

// dismissPrompt takes the prompt down once the scan has been answered, one way
// or the other. Without this a critical banner would sit there forever.
func dismissPrompt() {
	notifyMu.Lock()
	id := promptID
	promptID = 0
	notifyMu.Unlock()
	if id == 0 {
		return
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	conn.Object(notifyService, notifyPath).Call(notifyIface+".CloseNotification", 0, id)
}
