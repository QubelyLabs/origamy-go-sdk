package core

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/QubelyLabs/origamy-go-sdk/internal/dispatcher"
	"github.com/QubelyLabs/origamy-go-sdk/internal/queue"
)

// Version of the client.
const Version = "0.0.1"

// This interface is the main API exposed by the analytics package.
// Values that satsify this interface are returned by the client constructors
// provided by the package and provide a way to send messages via the HTTP API.
type Client interface {
	io.Closer

	// Queues a message to be sent by the client when the conditions for a batch
	// upload are met.
	// This is the main method you'll be using, a typical flow would look like
	// this:
	//
	//	client := analytics.New(writeKey)
	//	...
	//	client.Enqueue(analytics.Track{ ... })
	//	...
	//	client.Close()
	//
	// The method returns an error if the message queue not be queued, which
	// happens if the client was already closed at the time the method was
	// called or if the message was malformed.
	Enqueue(Message) error
}

type client struct {
	Config
	key string

	// The message queue used to buffer messages before sending.
	queue queue.Queue[Message]

	// These two channels are used to synchronize the client shutting down when
	// `Close` is called.
	// The first channel is closed to signal the backend goroutine that it has
	// to stop, then the second one is closed by the backend goroutine to signal
	// that it has finished flushing all queued messages.
	quit     chan struct{}
	shutdown chan struct{}

	// The dispatcher used to send requests to the backend.
	dispatcher dispatcher.Dispatcher
}

// Instantiate a new client that uses the write key passed as first argument to
// send messages to the backend.
// The client is created with the default configuration.
func New(writeKey string) Client {
	// Here we can ignore the error because the default config is always valid.
	c, _ := NewWithConfig(writeKey, Config{})
	return c
}

// Instantiate a new client that uses the write key and configuration passed as
// arguments to send messages to the backend.
// The function will return an error if the configuration contained impossible
// values (like a negative flush interval for example).
// When the function returns an error the returned client will always be nil.
func NewWithConfig(writeKey string, config Config) (cli Client, err error) {
	if err = config.validate(); err != nil {
		return
	}

	config = makeConfig(config)

	// Create dispatcher if not provided
	disp := config.Dispatcher
	if disp == nil {
		disp = makeDefaultDispatcher(writeKey, config)
	}

	// Create queue if not provided
	msgQueue := config.Queue
	if msgQueue == nil {
		msgQueue = makeDefaultQueue(config)
	}

	c := &client{
		Config:     config,
		key:        writeKey,
		queue:      msgQueue,
		quit:       make(chan struct{}),
		shutdown:   make(chan struct{}),
		dispatcher: disp,
	}

	go c.loop()

	cli = c
	return
}

// makeDefaultQueue creates a channel-based queue with the given configuration.
func makeDefaultQueue(config Config) queue.Queue[Message] {
	capacity := config.QueueCapacity
	if capacity <= 0 {
		capacity = 100
	}
	return queue.NewChannelQueue[Message](queue.WithCapacity[Message](capacity))
}

// makeDefaultDispatcher creates an HTTP dispatcher with the given configuration.
func makeDefaultDispatcher(writeKey string, config Config) dispatcher.Dispatcher {
	dispConfig := dispatcher.Config{
		Endpoint: config.Endpoint,
		WriteKey: writeKey,
		Version:  Version,
		Verbose:  config.Verbose,
		Logger:   config.Logger,
	}

	var opts []dispatcher.HTTPOption
	if config.Transport != nil {
		opts = append(opts, dispatcher.WithTransport(config.Transport))
	}

	return dispatcher.NewHTTPDispatcher(dispConfig, opts...)
}

func dereferenceMessage(msg Message) Message {
	switch m := msg.(type) {
	case *Alias:
		if m == nil {
			return nil
		}
		return *m
	case *Group:
		if m == nil {
			return nil
		}
		return *m
	case *Identify:
		if m == nil {
			return nil
		}
		return *m
	case *Page:
		if m == nil {
			return nil
		}
		return *m
	case *Screen:
		if m == nil {
			return nil
		}
		return *m
	case *Track:
		if m == nil {
			return nil
		}
		return *m
	}

	return msg
}

func (c *client) Enqueue(msg Message) (err error) {
	msg = dereferenceMessage(msg)
	if err = msg.Validate(); err != nil {
		return
	}

	var id = c.uid()
	var ts = c.now()

	switch m := msg.(type) {
	case Alias:
		m.Type = "alias"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	case Group:
		m.Type = "group"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	case Identify:
		m.Type = "identify"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	case Page:
		m.Type = "page"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	case Screen:
		m.Type = "screen"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	case Track:
		m.Type = "track"
		m.MessageId = makeMessageId(m.MessageId, id)
		m.Timestamp = makeTimestamp(m.Timestamp, ts)
		if m.Context == nil {
			m.Context = c.DefaultContext
		}
		msg = m

	default:
		err = fmt.Errorf("messages with custom types cannot be enqueued: %T", msg)
		return
	}

	if err = c.queue.Enqueue(msg); err != nil {
		// Convert queue errors to client errors
		if c.queue.IsClosed() {
			err = ErrClosed
		}
		return
	}

	return
}

// Close and flush metrics.
func (c *client) Close() (err error) {
	defer func() {
		// Always recover, a panic could be raised if `c`.quit was closed which
		// means the method was called more than once.
		if recover() != nil {
			err = ErrClosed
		}
	}()
	close(c.quit)
	<-c.shutdown
	// Close the dispatcher to release any resources
	if c.dispatcher != nil {
		c.dispatcher.Close()
	}
	return
}

// Asychronously send a batched requests.
func (c *client) sendAsync(msgs []message, wg *sync.WaitGroup, ex *executor) {
	wg.Add(1)

	if !ex.do(func() {
		defer wg.Done()
		defer func() {
			// In case a bug is introduced in the send function that triggers
			// a panic, we don't want this to ever crash the application so we
			// catch it here and log it instead.
			if err := recover(); err != nil {
				c.errorf("panic - %s", err)
			}
		}()
		c.send(msgs)
	}) {
		wg.Done()
		c.errorf("sending messages failed - %s", ErrTooManyRequests)
		c.notifyFailure(msgs, ErrTooManyRequests)
	}
}

// Send batch request.
func (c *client) send(msgs []message) {
	const attempts = 10

	b, err := json.Marshal(batch{
		SentAt:   c.now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Messages: msgs,
	})

	if err != nil {
		c.errorf("marshalling messages - %s", err)
		c.notifyFailure(msgs, err)
		return
	}

	for i := 0; i != attempts; i++ {
		if err = c.dispatcher.Send(b); err == nil {
			c.notifySuccess(msgs)
			return
		}

		// Wait for either a retry timeout or the client to be closed.
		select {
		case <-time.After(c.RetryAfter(i)):
		case <-c.quit:
			c.errorf("%d messages dropped because they failed to be sent and the client was closed", len(msgs))
			c.notifyFailure(msgs, err)
			return
		}
	}

	c.errorf("%d messages dropped because they failed to be sent after %d attempts", len(msgs), attempts)
	c.notifyFailure(msgs, err)
}

// Batch loop.
func (c *client) loop() {
	defer close(c.shutdown)

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	tick := time.NewTicker(c.Interval)
	defer tick.Stop()

	ex := newExecutor(c.maxConcurrentRequests)
	defer ex.close()

	mq := messageQueue{
		maxBatchSize:  c.BatchSize,
		maxBatchBytes: c.maxBatchBytes(),
	}

	// Get the channel for select statement
	msgChan := c.queue.Channel()

	for {
		select {
		case msg, ok := <-msgChan:
			if ok {
				c.push(&mq, msg, wg, ex)
			}

		case <-tick.C:
			c.flush(&mq, wg, ex)

		case <-c.quit:
			c.debugf("exit requested – draining messages")

			// Close the queue to prevent new messages and drain remaining ones
			c.queue.Close()
			for msg := range c.queue.Drain() {
				c.push(&mq, msg, wg, ex)
			}

			c.flush(&mq, wg, ex)
			c.debugf("exit")
			return
		}
	}
}

func (c *client) push(q *messageQueue, m Message, wg *sync.WaitGroup, ex *executor) {
	var msg message
	var err error

	if msg, err = makeMessage(m, maxMessageBytes); err != nil {
		c.errorf("%s - %v", err, m)
		c.notifyFailure([]message{{m, nil}}, err)
		return
	}

	c.debugf("buffer (%d/%d) %v", len(q.pending), c.BatchSize, m)

	if msgs := q.push(msg); msgs != nil {
		c.debugf("exceeded messages batch limit with batch of %d messages – flushing", len(msgs))
		c.sendAsync(msgs, wg, ex)
	}
}

func (c *client) flush(q *messageQueue, wg *sync.WaitGroup, ex *executor) {
	if msgs := q.flush(); msgs != nil {
		c.debugf("flushing %d messages", len(msgs))
		c.sendAsync(msgs, wg, ex)
	}
}

func (c *client) debugf(format string, args ...interface{}) {
	if c.Verbose {
		c.logf(format, args...)
	}
}

func (c *client) logf(format string, args ...interface{}) {
	c.Logger.Logf(format, args...)
}

func (c *client) errorf(format string, args ...interface{}) {
	c.Logger.Errorf(format, args...)
}

func (c *client) maxBatchBytes() int {
	b, _ := json.Marshal(batch{
		SentAt: c.now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	return maxBatchBytes - len(b)
}

func (c *client) notifySuccess(msgs []message) {
	if c.Callback != nil {
		for _, m := range msgs {
			c.Callback.Success(m.msg)
		}
	}
}

func (c *client) notifyFailure(msgs []message, err error) {
	if c.Callback != nil {
		for _, m := range msgs {
			c.Callback.Failure(m.msg, err)
		}
	}
}
