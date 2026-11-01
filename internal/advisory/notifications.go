package advisory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

// A validated positive finding remains useful when other groups are unassessed.
// Only collection failure/age/unavailability breaks continuity.
func (c *connection) notificationObservation() bool {
	return c.snapshot != nil && (c.state == "fresh" || c.state == "partial")
}

// admit runs under the connection lock. Startup/recovery establish silent baselines.
func (c *connection) admit() {
	if !c.config.NotificationsEnabled || c.publisher == nil {
		return
	}
	if !c.notificationObservation() {
		c.baseline = false
		return
	}
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.securitySeen == nil {
		c.securitySeen = map[string]bool{}
	}
	dropped := 0
	// Security flags take the bounded queue slots before regular new candidates.
	for _, security := range []bool{true, false} {
		for _, d := range c.snapshot.Dependencies {
			if d.Assessment != "updates_found" {
				continue
			}
			for _, u := range d.Candidates {
				if u.IsVulnerabilityAlert != security {
					continue
				}
				known := c.seen[u.ID]
				if security {
					known = c.securitySeen[u.ID]
				}
				c.seen[u.ID] = true
				if security {
					c.securitySeen[u.ID] = true
				}
				if known || !c.baseline {
					continue
				}
				if len(c.pending) >= 64 && security {
					// Prefer a new security notice over an unaccepted ordinary notice.
					for index := len(c.pending) - 1; index >= 0; index-- {
						if c.pending[index].Severity == notifications.SeverityInfo {
							c.pending = append(c.pending[:index], c.pending[index+1:]...)
							dropped++
							break
						}
					}
				}
				if len(c.pending) >= 64 {
					dropped++
					continue
				}
				eventID, title, severity := "image-upgrade:"+u.ID, "Image upgrade: "+d.PackageName, notifications.SeverityInfo
				body := fmt.Sprintf("%s · %s. Git reference %s; Renovate candidate %s. Update type %s.", c.snapshot.Repository.Name, d.PackageFile, imageReference(d.CurrentValue, d.CurrentDigest), imageReference(u.NewValue, u.NewDigest), u.UpdateType)
				if security {
					eventID = "image-upgrade-security:" + u.ID
					title = "Renovate security flag: " + d.PackageName
					severity = notifications.SeverityWarning
					body = "Renovate flag only; CVE and severity are not established. " + body + " Renovate reported isVulnerabilityAlert=true for this candidate."
				}
				if c.state == "partial" {
					body += " Collection coverage is partial; this notice concerns this candidate only."
				}
				m := notifications.Message{EventID: eventID, Severity: severity, Title: limitText(title, 200), Body: limitText(body, 4000), OccurredAt: c.snapshot.CompletedAt, Destination: &notifications.Destination{Kind: notifications.DestinationPlugin, PluginID: "araihu.image-upgrades", ConnectionID: c.id, Page: "detail", Params: map[string]string{"dependency-id": d.ID}}}
				for _, mapping := range c.config.ServiceMappings {
					if mapping.DependencyID == d.ID {
						m.ServiceID = mapping.ServiceID
					}
				}
				if m.Validate() == nil {
					c.pending = append(c.pending, m)
				}
			}
		}
	}
	if dropped > 0 {
		slog.Warn("Image upgrade notification queue limit", "connection_id", c.id, "dropped", dropped)
	}
	c.baseline = true
}
func limitText(s string, n int) string {
	value := []rune(s)
	if len(value) > n {
		return string(value[:n-1]) + "…"
	}
	return s
}
func imageReference(value, digest *string) string {
	ref := "not reported"
	if value != nil {
		ref = *value
	}
	if digest != nil {
		if value == nil {
			return *digest
		}
		ref += "@" + *digest
	}
	return ref
}

// Publication has its own bounded deadline and never runs on the query path.
func (c *connection) deliver(ctx context.Context) {
	c.mu.Lock()
	c.refresh(time.Now())
	if !c.notificationObservation() || c.publisher == nil {
		c.mu.Unlock()
		return
	}
	queue := append([]notifications.Message(nil), c.pending...)
	c.mu.Unlock()
	for _, m := range queue {
		call, cancel := context.WithTimeout(ctx, notifications.PublishTimeout)
		_, err := c.publisher.Publish(call, m)
		cancel()
		if err != nil && (notifications.Code(err) == "unavailable" || notifications.Code(err) == "deadline_exceeded" || notifications.Code(err) == "resource_exhausted") {
			return
		}
		c.mu.Lock()
		for i, p := range c.pending {
			if p.EventID == m.EventID {
				c.pending = append(c.pending[:i], c.pending[i+1:]...)
				break
			}
		}
		c.mu.Unlock()
	}
}
