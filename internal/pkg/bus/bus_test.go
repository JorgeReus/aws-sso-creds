package bus

import "testing"

func TestBusSendAndRecv(t *testing.T) {
	b := NewBus()
	want := BusMsg{MsgType: MSG_TYPE_INFO, Contents: "hello"}

	go b.Send(want)

	got := b.Recv()
	if got != want {
		t.Fatalf("Recv() = %#v, want %#v", got, want)
	}
}
