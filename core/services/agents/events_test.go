package agents

import (
	"encoding/json"
	"time"

	"github.com/mudler/LocalAI/core/services/messaging"
	"github.com/mudler/cogito"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeMessagingClient implements messaging.MessagingClient and captures the
// last published payload so tests can assert on it.
type fakeMessagingClient struct {
	lastSubject string
	lastData    any
}

func (f *fakeMessagingClient) Publish(subject string, data any) error {
	f.lastSubject = subject
	f.lastData = data
	return nil
}

func (f *fakeMessagingClient) Subscribe(string, func([]byte)) (messaging.Subscription, error) {
	return &fakeSub{}, nil
}

func (f *fakeMessagingClient) QueueSubscribe(string, string, func([]byte)) (messaging.Subscription, error) {
	return &fakeSub{}, nil
}

func (f *fakeMessagingClient) QueueSubscribeReply(string, string, func([]byte, func([]byte))) (messaging.Subscription, error) {
	return &fakeSub{}, nil
}

func (f *fakeMessagingClient) SubscribeReply(string, func([]byte, func([]byte))) (messaging.Subscription, error) {
	return &fakeSub{}, nil
}

func (f *fakeMessagingClient) Request(string, []byte, time.Duration) ([]byte, error) {
	return nil, nil
}

func (f *fakeMessagingClient) IsConnected() bool { return true }
func (f *fakeMessagingClient) Close()            {}

type fakeSub struct{}

func (s *fakeSub) Unsubscribe() error { return nil }

var _ = Describe("EventBridge", func() {
	Describe("PublishEvent timestamp", func() {
		// Regression for #9867: agent chat messages rendered a broken
		// timestamp ("Invalid Timestamp" / "12:00 AM") in the web UI because
		// this path emitted Unix nanoseconds while the local dispatcher and the
		// React UI both expect Unix milliseconds. Nanoseconds also overflow JS's
		// safe-integer range. The timestamp must be in milliseconds.
		It("emits the timestamp in Unix milliseconds", func() {
			fake := &fakeMessagingClient{}
			bridge := NewEventBridge(fake, nil, "instance-1")

			before := time.Now().UnixMilli()
			err := bridge.PublishMessage("agent", "user", "agent", "hello", "msg-1")
			after := time.Now().UnixMilli()

			Expect(err).ToNot(HaveOccurred())

			evt, ok := fake.lastData.(AgentEvent)
			Expect(ok).To(BeTrue(), "published payload should be an AgentEvent")

			// A millisecond timestamp falls within [before, after]; a nanosecond
			// one (~1e6 larger) would be far outside this window.
			Expect(evt.Timestamp).To(BeNumerically(">=", before))
			Expect(evt.Timestamp).To(BeNumerically("<=", after))
		})
	})

	Describe("request correlation", func() {
		It("carries the request ID in status event metadata and the event envelope", func() {
			fake := &fakeMessagingClient{}
			bridge := NewEventBridge(fake, nil, "instance-1")

			Expect(bridge.PublishStatus("agent", "user", "request-1", "processing")).To(Succeed())

			evt, ok := fake.lastData.(AgentEvent)
			Expect(ok).To(BeTrue())
			Expect(evt.MessageID).To(Equal("request-1"))
			var metadata map[string]any
			Expect(json.Unmarshal([]byte(evt.Metadata), &metadata)).To(Succeed())
			Expect(metadata).To(HaveKeyWithValue("message_id", "request-1"))
			Expect(metadata).To(HaveKeyWithValue("status", "processing"))
		})

		It("records and publishes the governed case scope for status lookup", func() {
			fake := &fakeMessagingClient{}
			bridge := NewEventBridge(fake, nil, "instance-1")

			Expect(bridge.PublishStatus("agent", "user", "request-scoped", "timed_out", "case-alpha")).To(Succeed())
			status, ok := bridge.RequestStatus("agent", "user", "case-alpha", "request-scoped")
			Expect(ok).To(BeTrue())
			Expect(status.Status).To(Equal("timed_out"))
			Expect(status.CaseID).To(Equal("case-alpha"))
			_, crossCase := bridge.RequestStatus("agent", "user", "case-beta", "request-scoped")
			Expect(crossCase).To(BeFalse())

			evt := fake.lastData.(AgentEvent)
			var metadata map[string]any
			Expect(json.Unmarshal([]byte(evt.Metadata), &metadata)).To(Succeed())
			Expect(metadata).To(HaveKeyWithValue("case_id", "case-alpha"))
			Expect(metadata).To(HaveKeyWithValue("collection_id", "case-alpha"))
		})

		It("carries the request ID through dispatcher stream callbacks", func() {
			fake := &fakeMessagingClient{}
			bridge := NewEventBridge(fake, nil, "instance-1")
			dispatcher := &NATSDispatcher{eventBridge: bridge}
			callbacks := dispatcher.buildNATSCallbacks(AgentChatEvent{
				AgentName: "agent",
				UserID:    "user",
				Message:   "question",
				MessageID: "request-2",
			}, "case-alpha")

			callbacks.OnStream(cogito.StreamEvent{Type: cogito.StreamEventContent, Content: "bounded answer"})

			evt, ok := fake.lastData.(AgentEvent)
			Expect(ok).To(BeTrue())
			Expect(evt.EventType).To(Equal("stream_event"))
			Expect(evt.MessageID).To(Equal("request-2"))
			var metadata map[string]any
			Expect(json.Unmarshal([]byte(evt.Metadata), &metadata)).To(Succeed())
			Expect(metadata).To(HaveKeyWithValue("message_id", "request-2"))
			Expect(metadata).To(HaveKeyWithValue("content", "bounded answer"))
		})
	})
})
