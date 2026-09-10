package alerting

import (
	"fmt"
	"time"
)

// Message builds the subject and body for a notification. Kept separate so the
// wording can be reviewed and tested without touching delivery code.
func Message(kind, monitorName, target, tenantSlug, publicURL, cause string, since time.Time) (subject, body string) {
	statusURL := fmt.Sprintf("%s/%s", publicURL, tenantSlug)

	if kind == "down" {
		subject = fmt.Sprintf("[Pulse] %s is DOWN", monitorName)
		body = fmt.Sprintf(
			"%s is not responding.\n\nTarget: %s\nDetected: %s\nReason: %s\n\nStatus page: %s\n",
			monitorName, target, since.UTC().Format(time.RFC1123), cause, statusURL)
		return
	}

	dur := time.Since(since).Round(time.Second)
	subject = fmt.Sprintf("[Pulse] %s has RECOVERED", monitorName)
	body = fmt.Sprintf(
		"%s is responding again.\n\nTarget: %s\nDowntime: %s\n\nStatus page: %s\n",
		monitorName, target, dur, statusURL)
	return
}
