package bus

const (
	MSG_TYPE_CONT uint32 = 0
	MSG_TYPE_ERR  uint32 = 1
	MSG_TYPE_INFO uint32 = 2
)

type BusMsg struct {
	MsgType  uint32
	Contents string
}

type Bus struct {
	Channel chan BusMsg
}
