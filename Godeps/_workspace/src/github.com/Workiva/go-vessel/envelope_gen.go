package vessel

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *PollResponse) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "PartitionOffsets":
			var msz uint32
			msz, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.PartitionOffsets == nil && msz > 0 {
				z.PartitionOffsets = make(map[string]int64, msz)
			} else if len(z.PartitionOffsets) > 0 {
				for key, _ := range z.PartitionOffsets {
					delete(z.PartitionOffsets, key)
				}
			}
			for msz > 0 {
				msz--
				var xvk string
				var bzg int64
				xvk, err = dc.ReadString()
				if err != nil {
					return
				}
				bzg, err = dc.ReadInt64()
				if err != nil {
					return
				}
				z.PartitionOffsets[xvk] = bzg
			}
		case "Envelopes":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Envelopes) >= int(xsz) {
				z.Envelopes = z.Envelopes[:xsz]
			} else {
				z.Envelopes = make([]*Envelope, xsz)
			}
			for bai := range z.Envelopes {
				if dc.IsNil() {
					err = dc.ReadNil()
					if err != nil {
						return
					}
					z.Envelopes[bai] = nil
				} else {
					if z.Envelopes[bai] == nil {
						z.Envelopes[bai] = new(Envelope)
					}
					err = z.Envelopes[bai].DecodeMsg(dc)
					if err != nil {
						return
					}
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *PollResponse) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(2)
	if err != nil {
		return
	}
	err = en.WriteString("PartitionOffsets")
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.PartitionOffsets)))
	if err != nil {
		return
	}
	for xvk, bzg := range z.PartitionOffsets {
		err = en.WriteString(xvk)
		if err != nil {
			return
		}
		err = en.WriteInt64(bzg)
		if err != nil {
			return
		}
	}
	err = en.WriteString("Envelopes")
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Envelopes)))
	if err != nil {
		return
	}
	for bai := range z.Envelopes {
		if z.Envelopes[bai] == nil {
			err = en.WriteNil()
			if err != nil {
				return
			}
		} else {
			err = z.Envelopes[bai].EncodeMsg(en)
			if err != nil {
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *PollResponse) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 2)
	o = msgp.AppendString(o, "PartitionOffsets")
	o = msgp.AppendMapHeader(o, uint32(len(z.PartitionOffsets)))
	for xvk, bzg := range z.PartitionOffsets {
		o = msgp.AppendString(o, xvk)
		o = msgp.AppendInt64(o, bzg)
	}
	o = msgp.AppendString(o, "Envelopes")
	o = msgp.AppendArrayHeader(o, uint32(len(z.Envelopes)))
	for bai := range z.Envelopes {
		if z.Envelopes[bai] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z.Envelopes[bai].MarshalMsg(o)
			if err != nil {
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *PollResponse) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "PartitionOffsets":
			var msz uint32
			msz, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.PartitionOffsets == nil && msz > 0 {
				z.PartitionOffsets = make(map[string]int64, msz)
			} else if len(z.PartitionOffsets) > 0 {
				for key, _ := range z.PartitionOffsets {
					delete(z.PartitionOffsets, key)
				}
			}
			for msz > 0 {
				var xvk string
				var bzg int64
				msz--
				xvk, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				bzg, bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					return
				}
				z.PartitionOffsets[xvk] = bzg
			}
		case "Envelopes":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Envelopes) >= int(xsz) {
				z.Envelopes = z.Envelopes[:xsz]
			} else {
				z.Envelopes = make([]*Envelope, xsz)
			}
			for bai := range z.Envelopes {
				if msgp.IsNil(bts) {
					bts, err = msgp.ReadNilBytes(bts)
					if err != nil {
						return
					}
					z.Envelopes[bai] = nil
				} else {
					if z.Envelopes[bai] == nil {
						z.Envelopes[bai] = new(Envelope)
					}
					bts, err = z.Envelopes[bai].UnmarshalMsg(bts)
					if err != nil {
						return
					}
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *PollResponse) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 16 + msgp.MapHeaderSize
	if z.PartitionOffsets != nil {
		for xvk, bzg := range z.PartitionOffsets {
			_ = bzg
			s += msgp.StringPrefixSize + len(xvk) + msgp.Int64Size
		}
	}
	s += msgp.StringPrefixSize + 9 + msgp.ArrayHeaderSize
	for bai := range z.Envelopes {
		if z.Envelopes[bai] == nil {
			s += msgp.NilSize
		} else {
			s += z.Envelopes[bai].Msgsize()
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *MessageType) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var tmp uint8
		tmp, err = dc.ReadUint8()
		(*z) = MessageType(tmp)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z MessageType) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteUint8(uint8(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z MessageType) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendUint8(o, uint8(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MessageType) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var tmp uint8
		tmp, bts, err = msgp.ReadUint8Bytes(bts)
		(*z) = MessageType(tmp)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

func (z MessageType) Msgsize() (s int) {
	s = msgp.Uint8Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *QoS) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var tmp int8
		tmp, err = dc.ReadInt8()
		(*z) = QoS(tmp)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z QoS) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt8(int8(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z QoS) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt8(o, int8(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *QoS) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var tmp int8
		tmp, bts, err = msgp.ReadInt8Bytes(bts)
		(*z) = QoS(tmp)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

func (z QoS) Msgsize() (s int) {
	s = msgp.Int8Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Envelopes) DecodeMsg(dc *msgp.Reader) (err error) {
	var xsz uint32
	xsz, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap((*z)) >= int(xsz) {
		(*z) = (*z)[:xsz]
	} else {
		(*z) = make(Envelopes, xsz)
	}
	for ajw := range *z {
		if dc.IsNil() {
			err = dc.ReadNil()
			if err != nil {
				return
			}
			(*z)[ajw] = nil
		} else {
			if (*z)[ajw] == nil {
				(*z)[ajw] = new(Envelope)
			}
			err = (*z)[ajw].DecodeMsg(dc)
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Envelopes) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(uint32(len(z)))
	if err != nil {
		return
	}
	for wht := range z {
		if z[wht] == nil {
			err = en.WriteNil()
			if err != nil {
				return
			}
		} else {
			err = z[wht].EncodeMsg(en)
			if err != nil {
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Envelopes) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, uint32(len(z)))
	for wht := range z {
		if z[wht] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z[wht].MarshalMsg(o)
			if err != nil {
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Envelopes) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var xsz uint32
	xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap((*z)) >= int(xsz) {
		(*z) = (*z)[:xsz]
	} else {
		(*z) = make(Envelopes, xsz)
	}
	for hct := range *z {
		if msgp.IsNil(bts) {
			bts, err = msgp.ReadNilBytes(bts)
			if err != nil {
				return
			}
			(*z)[hct] = nil
		} else {
			if (*z)[hct] == nil {
				(*z)[hct] = new(Envelope)
			}
			bts, err = (*z)[hct].UnmarshalMsg(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z Envelopes) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for cua := range z {
		if z[cua] == nil {
			s += msgp.NilSize
		} else {
			s += z[cua].Msgsize()
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Envelope) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Type":
			{
				var tmp uint8
				tmp, err = dc.ReadUint8()
				z.Type = MessageType(tmp)
			}
			if err != nil {
				return
			}
		case "ID":
			z.ID, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Vector":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Vector) >= int(xsz) {
				z.Vector = z.Vector[:xsz]
			} else {
				z.Vector = make([]string, xsz)
			}
			for xhx := range z.Vector {
				z.Vector[xhx], err = dc.ReadString()
				if err != nil {
					return
				}
			}
		case "Channel":
			z.Channel, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Timestamp":
			z.Timestamp, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "QoS":
			{
				var tmp int8
				tmp, err = dc.ReadInt8()
				z.QoS = QoS(tmp)
			}
			if err != nil {
				return
			}
		case "TTL":
			z.TTL, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "Receivers":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Receivers) >= int(xsz) {
				z.Receivers = z.Receivers[:xsz]
			} else {
				z.Receivers = make([]string, xsz)
			}
			for lqf := range z.Receivers {
				z.Receivers[lqf], err = dc.ReadString()
				if err != nil {
					return
				}
			}
		case "Body":
			z.Body, err = dc.ReadBytes(z.Body)
			if err != nil {
				return
			}
		case "Headers":
			z.Headers, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Partition":
			z.Partition, err = dc.ReadInt32()
			if err != nil {
				return
			}
		case "Offset":
			z.Offset, err = dc.ReadInt64()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Envelope) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(12)
	if err != nil {
		return
	}
	err = en.WriteString("Type")
	if err != nil {
		return
	}
	err = en.WriteUint8(uint8(z.Type))
	if err != nil {
		return
	}
	err = en.WriteString("ID")
	if err != nil {
		return
	}
	err = en.WriteString(z.ID)
	if err != nil {
		return
	}
	err = en.WriteString("Vector")
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Vector)))
	if err != nil {
		return
	}
	for xhx := range z.Vector {
		err = en.WriteString(z.Vector[xhx])
		if err != nil {
			return
		}
	}
	err = en.WriteString("Channel")
	if err != nil {
		return
	}
	err = en.WriteString(z.Channel)
	if err != nil {
		return
	}
	err = en.WriteString("Timestamp")
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Timestamp)
	if err != nil {
		return
	}
	err = en.WriteString("QoS")
	if err != nil {
		return
	}
	err = en.WriteInt8(int8(z.QoS))
	if err != nil {
		return
	}
	err = en.WriteString("TTL")
	if err != nil {
		return
	}
	err = en.WriteInt64(z.TTL)
	if err != nil {
		return
	}
	err = en.WriteString("Receivers")
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Receivers)))
	if err != nil {
		return
	}
	for lqf := range z.Receivers {
		err = en.WriteString(z.Receivers[lqf])
		if err != nil {
			return
		}
	}
	err = en.WriteString("Body")
	if err != nil {
		return
	}
	err = en.WriteBytes(z.Body)
	if err != nil {
		return
	}
	err = en.WriteString("Headers")
	if err != nil {
		return
	}
	err = en.WriteString(z.Headers)
	if err != nil {
		return
	}
	err = en.WriteString("Partition")
	if err != nil {
		return
	}
	err = en.WriteInt32(z.Partition)
	if err != nil {
		return
	}
	err = en.WriteString("Offset")
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Offset)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Envelope) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 12)
	o = msgp.AppendString(o, "Type")
	o = msgp.AppendUint8(o, uint8(z.Type))
	o = msgp.AppendString(o, "ID")
	o = msgp.AppendString(o, z.ID)
	o = msgp.AppendString(o, "Vector")
	o = msgp.AppendArrayHeader(o, uint32(len(z.Vector)))
	for xhx := range z.Vector {
		o = msgp.AppendString(o, z.Vector[xhx])
	}
	o = msgp.AppendString(o, "Channel")
	o = msgp.AppendString(o, z.Channel)
	o = msgp.AppendString(o, "Timestamp")
	o = msgp.AppendInt64(o, z.Timestamp)
	o = msgp.AppendString(o, "QoS")
	o = msgp.AppendInt8(o, int8(z.QoS))
	o = msgp.AppendString(o, "TTL")
	o = msgp.AppendInt64(o, z.TTL)
	o = msgp.AppendString(o, "Receivers")
	o = msgp.AppendArrayHeader(o, uint32(len(z.Receivers)))
	for lqf := range z.Receivers {
		o = msgp.AppendString(o, z.Receivers[lqf])
	}
	o = msgp.AppendString(o, "Body")
	o = msgp.AppendBytes(o, z.Body)
	o = msgp.AppendString(o, "Headers")
	o = msgp.AppendString(o, z.Headers)
	o = msgp.AppendString(o, "Partition")
	o = msgp.AppendInt32(o, z.Partition)
	o = msgp.AppendString(o, "Offset")
	o = msgp.AppendInt64(o, z.Offset)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Envelope) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Type":
			{
				var tmp uint8
				tmp, bts, err = msgp.ReadUint8Bytes(bts)
				z.Type = MessageType(tmp)
			}
			if err != nil {
				return
			}
		case "ID":
			z.ID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Vector":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Vector) >= int(xsz) {
				z.Vector = z.Vector[:xsz]
			} else {
				z.Vector = make([]string, xsz)
			}
			for xhx := range z.Vector {
				z.Vector[xhx], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
			}
		case "Channel":
			z.Channel, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Timestamp":
			z.Timestamp, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "QoS":
			{
				var tmp int8
				tmp, bts, err = msgp.ReadInt8Bytes(bts)
				z.QoS = QoS(tmp)
			}
			if err != nil {
				return
			}
		case "TTL":
			z.TTL, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "Receivers":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Receivers) >= int(xsz) {
				z.Receivers = z.Receivers[:xsz]
			} else {
				z.Receivers = make([]string, xsz)
			}
			for lqf := range z.Receivers {
				z.Receivers[lqf], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
			}
		case "Body":
			z.Body, bts, err = msgp.ReadBytesBytes(bts, z.Body)
			if err != nil {
				return
			}
		case "Headers":
			z.Headers, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Partition":
			z.Partition, bts, err = msgp.ReadInt32Bytes(bts)
			if err != nil {
				return
			}
		case "Offset":
			z.Offset, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *Envelope) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 4 + msgp.Uint8Size + msgp.StringPrefixSize + 2 + msgp.StringPrefixSize + len(z.ID) + msgp.StringPrefixSize + 6 + msgp.ArrayHeaderSize
	for xhx := range z.Vector {
		s += msgp.StringPrefixSize + len(z.Vector[xhx])
	}
	s += msgp.StringPrefixSize + 7 + msgp.StringPrefixSize + len(z.Channel) + msgp.StringPrefixSize + 9 + msgp.Int64Size + msgp.StringPrefixSize + 3 + msgp.Int8Size + msgp.StringPrefixSize + 3 + msgp.Int64Size + msgp.StringPrefixSize + 9 + msgp.ArrayHeaderSize
	for lqf := range z.Receivers {
		s += msgp.StringPrefixSize + len(z.Receivers[lqf])
	}
	s += msgp.StringPrefixSize + 4 + msgp.BytesPrefixSize + len(z.Body) + msgp.StringPrefixSize + 7 + msgp.StringPrefixSize + len(z.Headers) + msgp.StringPrefixSize + 9 + msgp.Int32Size + msgp.StringPrefixSize + 6 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *PollRequest) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "PartitionOffsets":
			var msz uint32
			msz, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.PartitionOffsets == nil && msz > 0 {
				z.PartitionOffsets = make(map[string]int64, msz)
			} else if len(z.PartitionOffsets) > 0 {
				for key, _ := range z.PartitionOffsets {
					delete(z.PartitionOffsets, key)
				}
			}
			for msz > 0 {
				msz--
				var daf string
				var pks int64
				daf, err = dc.ReadString()
				if err != nil {
					return
				}
				pks, err = dc.ReadInt64()
				if err != nil {
					return
				}
				z.PartitionOffsets[daf] = pks
			}
		case "Count":
			z.Count, err = dc.ReadUint32()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *PollRequest) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteMapHeader(2)
	if err != nil {
		return
	}
	err = en.WriteString("PartitionOffsets")
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.PartitionOffsets)))
	if err != nil {
		return
	}
	for daf, pks := range z.PartitionOffsets {
		err = en.WriteString(daf)
		if err != nil {
			return
		}
		err = en.WriteInt64(pks)
		if err != nil {
			return
		}
	}
	err = en.WriteString("Count")
	if err != nil {
		return
	}
	err = en.WriteUint32(z.Count)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *PollRequest) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, 2)
	o = msgp.AppendString(o, "PartitionOffsets")
	o = msgp.AppendMapHeader(o, uint32(len(z.PartitionOffsets)))
	for daf, pks := range z.PartitionOffsets {
		o = msgp.AppendString(o, daf)
		o = msgp.AppendInt64(o, pks)
	}
	o = msgp.AppendString(o, "Count")
	o = msgp.AppendUint32(o, z.Count)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *PollRequest) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "PartitionOffsets":
			var msz uint32
			msz, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.PartitionOffsets == nil && msz > 0 {
				z.PartitionOffsets = make(map[string]int64, msz)
			} else if len(z.PartitionOffsets) > 0 {
				for key, _ := range z.PartitionOffsets {
					delete(z.PartitionOffsets, key)
				}
			}
			for msz > 0 {
				var daf string
				var pks int64
				msz--
				daf, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				pks, bts, err = msgp.ReadInt64Bytes(bts)
				if err != nil {
					return
				}
				z.PartitionOffsets[daf] = pks
			}
		case "Count":
			z.Count, bts, err = msgp.ReadUint32Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *PollRequest) Msgsize() (s int) {
	s = msgp.MapHeaderSize + msgp.StringPrefixSize + 16 + msgp.MapHeaderSize
	if z.PartitionOffsets != nil {
		for daf, pks := range z.PartitionOffsets {
			_ = pks
			s += msgp.StringPrefixSize + len(daf) + msgp.Int64Size
		}
	}
	s += msgp.StringPrefixSize + 5 + msgp.Uint32Size
	return
}
