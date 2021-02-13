package bombardier

import (
	"strconv"
	"time"
)

const (
	nilStr = "nil"
)

type NullableUint64 struct {
	val *uint64
}

func (n *NullableUint64) String() string {
	if n.val == nil {
		return nilStr
	}
	return strconv.FormatUint(*n.val, 10)
}

func (n *NullableUint64) Set(value string) error {
	res, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	n.val = new(uint64)
	*n.val = res
	return nil
}

type NullableDuration struct {
	val *time.Duration
}

func (n *NullableDuration) String() string {
	if n.val == nil {
		return nilStr
	}
	return n.val.String()
}

func (n *NullableDuration) Set(value string) error {
	res, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	n.val = &res
	return nil
}

type NullableString struct {
	val *string
}

func (n *NullableString) String() string {
	if n.val == nil {
		return nilStr
	}
	return *n.val
}

func (n *NullableString) Set(value string) error {
	n.val = new(string)
	*n.val = value
	return nil
}
