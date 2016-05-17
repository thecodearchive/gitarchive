package github

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// MarshalMsg implements msgp.Marshaler
func (z Repo) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Stars"
	o = append(o, 0x83, 0xa5, 0x53, 0x74, 0x61, 0x72, 0x73)
	o = msgp.AppendInt(o, z.Stars)
	// string "Parent"
	o = append(o, 0xa6, 0x50, 0x61, 0x72, 0x65, 0x6e, 0x74)
	o = msgp.AppendString(o, z.Parent)
	// string "LastUpdated"
	o = append(o, 0xab, 0x4c, 0x61, 0x73, 0x74, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64)
	o = msgp.AppendTime(o, z.LastUpdated)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Repo) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var xvk uint32
	xvk, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for xvk > 0 {
		xvk--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Stars":
			z.Stars, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Parent":
			z.Parent, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "LastUpdated":
			z.LastUpdated, bts, err = msgp.ReadTimeBytes(bts)
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

func (z Repo) Msgsize() (s int) {
	s = 1 + 6 + msgp.IntSize + 7 + msgp.StringPrefixSize + len(z.Parent) + 12 + msgp.TimeSize
	return
}
