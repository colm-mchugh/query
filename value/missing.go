//  Copieright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package value

import (
	"fmt"
)

// Missing value
type missingValue string

var MISSING_VALUE Value = missingValue("")

func NewMissingValue() Value {
	return MISSING_VALUE
}

// Ideally, we should never marshal a MISSING value.
func (this missingValue) MarshalJSON() ([]byte, error) {
	return _NULL_BYTES, nil
}

// Description of which property or index was undefined (if known).
func (this missingValue) Error() string {
	if string(this) != "" {
		return fmt.Sprintf("Missing field or index %s.", string(this))
	} else {
		return "Missing field or index."
	}
}

func (this missingValue) Type() Type { return MISSING }

func (this missingValue) Actual() interface{} {
	return nil
}

func (this missingValue) Equals(other Value) bool {
	return other.Type() == MISSING
}

func (this missingValue) Collate(other Value) int {
	return int(MISSING - other.Type())
}

func (this missingValue) Truth() bool {
	return false
}

func (this missingValue) Copy() Value {
	return this
}

func (this missingValue) CopyForUpdate() Value {
	return this
}

var _MISSING_BYTES = []byte{}

func (this missingValue) Bytes() []byte {
	return _MISSING_BYTES
}

func (this missingValue) Field(field string) (Value, bool) {
	return missingField(field), false
}

func (this missingValue) SetField(field string, val interface{}) error {
	return Unsettable(field)
}

func (this missingValue) UnsetField(field string) error {
	return Unsettable(field)
}

func (this missingValue) Index(index int) (Value, bool) {
	return missingIndex(index), false
}

func (this missingValue) SetIndex(index int, val interface{}) error {
	return Unsettable(index)
}

func (this missingValue) Slice(start, end int) (Value, bool) {
	return MISSING_VALUE, false
}

func (this missingValue) SliceTail(start int) (Value, bool) {
	return MISSING_VALUE, false
}

func (this missingValue) Descendants(buffer []interface{}) []interface{} {
	return buffer
}

func (this missingValue) Fields() map[string]interface{} {
	return nil
}

func missingField(field string) missingValue {
	return missingValue(field)
}

func missingIndex(index int) missingValue {
	return missingValue(string(index))
}
