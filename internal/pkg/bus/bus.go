package bus

func NewBus() *Bus {
	return &Bus{
		Channel: make(chan BusMsg),
	}
}

func (b *Bus) Send(msg BusMsg) {
	b.Channel <- msg
}

func (b *Bus) Recv() BusMsg {
	return (<-b.Channel)
}
