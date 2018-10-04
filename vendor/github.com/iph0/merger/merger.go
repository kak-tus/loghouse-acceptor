// Copyright (c) 2018, Eugene Ponizovsky, <ponizovsky@gmail.com>. All rights
// reserved. Use of this source code is governed by a MIT License that can
// be found in the LICENSE file.

// Package merger performs recursive merge of maps or structures into new one.
// Non-zero values from the right side has higher precedence. Slices do not
// merging, because main use case of this package is merging configuration
// parameters, and in this case merging of slices is unacceptable. Slices from
// the right side has higher precedence.
package merger

import "reflect"

var globalMerger = New(Config{})

// Merger type represents merger.
type Merger struct {
	config *Config
}

// Config type represents configuration parameters for merger.
type Config struct {
	// MergeHook is a hook that is called on every merge pair.
	MergeHook func(m *Merger, left, right reflect.Value) reflect.Value
}

// New method creates new merger instance.
func New(config Config) *Merger {
	return &Merger{
		config: &config,
	}
}

// Merge method performs recursive merge of two values into new one using global
// merger.
func Merge(left, right interface{}) interface{} {
	return globalMerger.Merge(left, right)
}

// Merge method performs recursive merge of two values into new one.
func (m *Merger) Merge(left, right interface{}) interface{} {
	res := m.merge(
		reflect.ValueOf(left),
		reflect.ValueOf(right),
	)

	if !res.IsValid() {
		return nil
	}

	return res.Interface()
}

// MergeValues performs recursive merge of two reflect.Values into new one. Method
// must be used only from MergeHook.
func (m *Merger) MergeValues(left, right reflect.Value) reflect.Value {
	return m.mergeValues(left, right)
}

func (m *Merger) merge(left, right reflect.Value) reflect.Value {
	left = reveal(left)
	right = reveal(right)

	if !left.IsValid() {
		return right
	}
	if !right.IsValid() {
		return left
	}

	if m.config.MergeHook != nil {
		return m.config.MergeHook(m, left, right)
	}

	return m.mergeValues(left, right)
}

func (m *Merger) mergeValues(left, right reflect.Value) reflect.Value {
	leftKind := left.Kind()
	rightKind := right.Kind()

	if leftKind == reflect.Ptr &&
		rightKind == reflect.Ptr {

		left := left.Elem()
		right := right.Elem()
		left = reveal(left)
		right = reveal(right)
		leftKind := left.Kind()
		rightKind := right.Kind()

		if leftKind == reflect.Map &&
			rightKind == reflect.Map {

			return m.mergeMap(left, right).Addr()
		} else if leftKind == reflect.Struct &&
			rightKind == reflect.Struct {

			return m.mergeStruct(left, right).Addr()
		}
	} else if leftKind == reflect.Map &&
		rightKind == reflect.Map {

		return m.mergeMap(left, right)
	} else if leftKind == reflect.Struct &&
		rightKind == reflect.Struct {

		return m.mergeStruct(left, right)
	}

	if isZero(right) {
		return left
	}

	return right
}

func (m *Merger) mergeMap(left, right reflect.Value) reflect.Value {
	rightType := right.Type()
	result := reflect.MakeMap(rightType)

	for _, key := range left.MapKeys() {
		result.SetMapIndex(key, left.MapIndex(key))
	}

	for _, key := range right.MapKeys() {
		value := m.merge(result.MapIndex(key), right.MapIndex(key))
		result.SetMapIndex(key, value)
	}

	return result
}

func (m *Merger) mergeStruct(left, right reflect.Value) reflect.Value {
	leftType := left.Type()
	rightType := right.Type()

	if leftType != rightType {
		return right
	}

	result := reflect.New(rightType).Elem()

	for i := 0; i < rightType.NumField(); i++ {
		leftField := left.Field(i)
		rightField := right.Field(i)
		resField := result.Field(i)

		if resField.Kind() == reflect.Interface &&
			isZero(leftField) && isZero(rightField) {

			continue
		}

		if resField.CanSet() {
			res := m.merge(leftField, rightField)
			resField.Set(res)
		}
	}

	return result
}

func reveal(value reflect.Value) reflect.Value {
	kind := value.Kind()

	if kind == reflect.Interface {
		return value.Elem()
	}

	return value
}

func isZero(value reflect.Value) bool {
	zero := reflect.Zero(value.Type())
	return reflect.DeepEqual(zero.Interface(), value.Interface())
}
