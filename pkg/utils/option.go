package utils

import (
	"bytes"
	"encoding/json"
)

// An Option can contain a value or nil (null) if set.
// This is incredibly useful for partial update API requests:
//   - For example: you cannot just use a pointer in all cases
//   - Take domain.Location, its 'address' field is optional
//   - So setting nil should be allowed, and to differentiate setting
//     a nil value from whether you wish to update the value is difficult.
//
// This is why both isSet and isNull exist here; there is no need
// for pointer types thanks to isNull.
type Option[T any] struct {
	value  T
	isSet  bool
	isNull bool
}

// Some returns a set Option object filled with a value (i.e., is not null)
func Some[T any](v T) Option[T] {
	return Option[T]{value: v, isSet: true, isNull: false}
}

// None returns an unset Option object
func None[T any]() Option[T] {
	return Option[T]{isSet: false}
}

// Null returns a set Option object that has no value (i.e., is null)
func Null[T any]() Option[T] {
	return Option[T]{isSet: true, isNull: true}
}

// IsSet reports whether the Option was explicitly given a value (Some or
// Null) rather than left absent (None).
func (o Option[T]) IsSet() bool { return o.isSet }

// IsNull reports whether the Option was explicitly set to null (Null).
// Meaningless if IsSet is false.
func (o Option[T]) IsNull() bool { return o.isNull }

// GetValue 'unwraps' the Option and extracts the value and valid status boolean.
// This method expects isSet = true and isNull = false.
// If either the Option is unset or null, it returns a zero value with a false
// (invalid) status.
// If it is set and not null, then the value is returned with a true (valid) status.
func (o Option[T]) GetValue() (T, bool) {
	if !o.isSet || o.isNull {
		var zeroVal T
		return zeroVal, false
	}
	return o.value, true
}

// ValueOr attempts to Get the value and if it fails (is invalid) it returns a fallback value.
func (o Option[T]) ValueOr(fallback T) T {
	if v, ok := o.GetValue(); ok {
		return v
	}
	return fallback
}

// Custom marshal of Option
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.isSet || o.isNull {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// Custom unmarshal of Option
func (o *Option[T]) UnmarshalJSON(data []byte) error {
	o.isSet = true

	if bytes.Equal(data, []byte("null")) {
		o.isNull = true
		return nil
	}

	o.isNull = false
	return json.Unmarshal(data, &o.value)
}
