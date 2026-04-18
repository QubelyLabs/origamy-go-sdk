package dispatcher

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// NoopDispatcher implements the Dispatcher interface but doesn't send data anywhere.
// Instead, it logs the payload in a human-readable format to the console.
// This is useful for development, debugging, and testing.
type NoopDispatcher struct {
	config  Config
	writer  *os.File
	colored bool
}

// NoopOption is a function that configures a NoopDispatcher.
type NoopOption func(*NoopDispatcher)

// WithOutput sets the output writer for the noop dispatcher.
// Defaults to os.Stdout.
func WithOutput(w *os.File) NoopOption {
	return func(d *NoopDispatcher) {
		d.writer = w
	}
}

// WithColor enables/disables colored output.
// Defaults to true when writing to a terminal.
func WithColor(enabled bool) NoopOption {
	return func(d *NoopDispatcher) {
		d.colored = enabled
	}
}

// NewNoopDispatcher creates a new noop dispatcher that logs to console.
func NewNoopDispatcher(config Config, opts ...NoopOption) *NoopDispatcher {
	d := &NoopDispatcher{
		config:  config,
		writer:  os.Stdout,
		colored: true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Send implements the Dispatcher interface by logging the payload.
func (d *NoopDispatcher) Send(payload []byte) error {
	// Parse the batch payload
	var batch struct {
		MessageId string          `json:"messageId"`
		SentAt    time.Time       `json:"sentAt"`
		Messages  json.RawMessage `json:"batch"`
		Context   json.RawMessage `json:"context"`
	}

	if err := json.Unmarshal(payload, &batch); err != nil {
		d.errorf("Failed to parse payload: %v", err)
		return nil // Still return nil since this is a noop
	}

	// Parse individual messages
	var messages []map[string]interface{}
	if err := json.Unmarshal(batch.Messages, &messages); err != nil {
		d.errorf("Failed to parse messages: %v", err)
		return nil
	}

	// Print header
	d.printHeader(batch.MessageId, batch.SentAt, len(messages))

	// Print each message
	for i, msg := range messages {
		d.printMessage(i+1, msg)
	}

	d.printFooter()

	return nil
}

// Close implements the Dispatcher interface.
func (d *NoopDispatcher) Close() error {
	return nil
}

func (d *NoopDispatcher) printHeader(batchId string, sentAt time.Time, count int) {
	d.println("")
	d.printLine("═", 60)
	d.printf("  %s ORIGAMY SDK - NOOP DISPATCHER %s\n", d.color("🔍", ""), d.color("", ""))
	d.printLine("─", 60)
	d.printf("  Batch ID:  %s\n", d.highlight(batchId))
	d.printf("  Sent At:   %s\n", d.highlight(sentAt.Format(time.RFC3339)))
	d.printf("  Messages:  %s\n", d.highlight(fmt.Sprintf("%d", count)))
	d.printLine("═", 60)
}

func (d *NoopDispatcher) printMessage(index int, msg map[string]interface{}) {
	msgType, _ := msg["type"].(string)
	msgId, _ := msg["messageId"].(string)
	userId, _ := msg["userId"].(string)
	anonymousId, _ := msg["anonymousId"].(string)

	d.println("")
	d.printf("  %s Message #%d: %s\n", d.icon(msgType), index, d.colorType(strings.ToUpper(msgType)))
	d.printLine("─", 50)

	// Common fields
	d.printf("    Message ID:   %s\n", d.dim(msgId))
	if userId != "" {
		d.printf("    User ID:      %s\n", d.highlight(userId))
	}
	if anonymousId != "" {
		d.printf("    Anonymous ID: %s\n", d.highlight(anonymousId))
	}

	// Type-specific fields
	switch msgType {
	case "track":
		if event, ok := msg["event"].(string); ok {
			d.printf("    Event:        %s\n", d.success(event))
		}
		if props, ok := msg["properties"].(map[string]interface{}); ok && len(props) > 0 {
			d.printf("    Properties:\n")
			d.printProperties(props, 6)
		}

	case "identify":
		if traits, ok := msg["traits"].(map[string]interface{}); ok && len(traits) > 0 {
			d.printf("    Traits:\n")
			d.printProperties(traits, 6)
		}

	case "page", "screen":
		if name, ok := msg["name"].(string); ok && name != "" {
			d.printf("    Name:         %s\n", d.success(name))
		}
		if props, ok := msg["properties"].(map[string]interface{}); ok && len(props) > 0 {
			d.printf("    Properties:\n")
			d.printProperties(props, 6)
		}

	case "group":
		if groupId, ok := msg["groupId"].(string); ok {
			d.printf("    Group ID:     %s\n", d.success(groupId))
		}
		if traits, ok := msg["traits"].(map[string]interface{}); ok && len(traits) > 0 {
			d.printf("    Traits:\n")
			d.printProperties(traits, 6)
		}

	case "alias":
		if prevId, ok := msg["previousId"].(string); ok {
			d.printf("    Previous ID:  %s\n", d.warn(prevId))
		}
	}

	// Timestamp
	if ts, ok := msg["timestamp"].(string); ok {
		d.printf("    Timestamp:    %s\n", d.dim(ts))
	}
}

func (d *NoopDispatcher) printProperties(props map[string]interface{}, indent int) {
	prefix := strings.Repeat(" ", indent)
	for k, v := range props {
		formatted, _ := json.Marshal(v)
		d.printf("%s%s: %s\n", prefix, d.dim(k), string(formatted))
	}
}

func (d *NoopDispatcher) printFooter() {
	d.println("")
	d.printLine("═", 60)
	d.printf("  %s This is a NOOP dispatcher - no data was sent\n", d.warn("⚠"))
	d.printLine("═", 60)
	d.println("")
}

func (d *NoopDispatcher) printLine(char string, width int) {
	d.println("  " + strings.Repeat(char, width))
}

func (d *NoopDispatcher) println(s string) {
	fmt.Fprintln(d.writer, s)
}

func (d *NoopDispatcher) printf(format string, args ...interface{}) {
	fmt.Fprintf(d.writer, format, args...)
}

func (d *NoopDispatcher) errorf(format string, args ...interface{}) {
	if d.config.Logger != nil {
		d.config.Logger.Errorf(format, args...)
	}
}

// Color helpers
func (d *NoopDispatcher) color(s, code string) string {
	if !d.colored {
		return s
	}
	return s
}

func (d *NoopDispatcher) highlight(s string) string {
	if !d.colored {
		return s
	}
	return "\033[1;36m" + s + "\033[0m" // Cyan bold
}

func (d *NoopDispatcher) success(s string) string {
	if !d.colored {
		return s
	}
	return "\033[1;32m" + s + "\033[0m" // Green bold
}

func (d *NoopDispatcher) warn(s string) string {
	if !d.colored {
		return s
	}
	return "\033[1;33m" + s + "\033[0m" // Yellow bold
}

func (d *NoopDispatcher) dim(s string) string {
	if !d.colored {
		return s
	}
	return "\033[2m" + s + "\033[0m" // Dim
}

func (d *NoopDispatcher) colorType(msgType string) string {
	if !d.colored {
		return msgType
	}
	colors := map[string]string{
		"TRACK":    "\033[1;35m", // Magenta
		"IDENTIFY": "\033[1;34m", // Blue
		"PAGE":     "\033[1;32m", // Green
		"SCREEN":   "\033[1;32m", // Green
		"GROUP":    "\033[1;33m", // Yellow
		"ALIAS":    "\033[1;31m", // Red
	}
	if c, ok := colors[msgType]; ok {
		return c + msgType + "\033[0m"
	}
	return msgType
}

func (d *NoopDispatcher) icon(msgType string) string {
	icons := map[string]string{
		"track":    "📊",
		"identify": "👤",
		"page":     "📄",
		"screen":   "📱",
		"group":    "👥",
		"alias":    "🔗",
	}
	if icon, ok := icons[msgType]; ok {
		return icon
	}
	return "📝"
}
