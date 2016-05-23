package vessel

// Predicate is a function which takes a Message and returns a bool indicating
// if the Message matches the predicate and if it should be filtered. A nil
// Predicate matches every Message. If take is true, the Message will be placed
// on the Filter channel. If filter is true, the Message will not be
// propagated.
type Predicate func(*Message) (take, filter bool)

// Filter is used to multiplex Messages into a channel. Each Message received
// by a Vessel client passes through a series of Filters. If the Message
// matches the Filter predicate, a copy of it is placed on the channel. A
// Filter with a nil predicate matches every Message.
type Filter struct {
	// Predicate is used to match a Message to a Filter. A nil Predicate
	// matches every message.
	Predicate Predicate

	// ExcludePublishes determines if publish messages should not be matched.
	ExcludePublishes bool

	// Channel is the channel on which matched messages are placed.
	Channel chan<- *Message

	// Block determines if the Filter should block putting on the channel if
	// it's full or simply skip the Filter.
	Block bool
}

func (f *Filter) apply(message *Message) bool {

	var take, filter bool
	if f.Predicate == nil {
		take = true
	} else {
		take, filter = f.Predicate(message)
	}

	if !take || f.Channel == nil || (f.ExcludePublishes && message.Channel != "") {
		return filter
	}

	msg := message.Copy()
	if f.Block {
		f.Channel <- msg
	} else {
		select {
		case f.Channel <- msg:
		default:
		}
	}

	return filter
}
