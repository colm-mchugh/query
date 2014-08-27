//  Copyright (c) 2013 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package couchbase

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/couchbaselabs/clog"
	cb "github.com/couchbaselabs/go-couchbase"
	"github.com/couchbaselabs/query/datastore"
	"github.com/couchbaselabs/query/errors"
	"github.com/couchbaselabs/query/value"
)

const NETWORK_CHANNEL = "NETWORK"

const TYPE_NULL = 64
const TYPE_BOOLEAN = 96
const TYPE_NUMBER = 128
const TYPE_STRING = 160
const TYPE_ARRAY = 192
const TYPE_OBJECT = 224

var MIN_ID = cb.DocID("")
var MAX_ID = cb.DocID(strings.Repeat(string([]byte{0xff}), 251))

func ViewTotalRows(bucket *cb.Bucket, ddoc string, view string, options map[string]interface{}) (int64, errors.Error) {
	options["limit"] = 0

	logURL, err := bucket.ViewURL(ddoc, view, options)
	if err == nil {
		clog.To(NETWORK_CHANNEL, "Request View: %v", logURL)
	}
	vres, err := bucket.View(ddoc, view, options)
	if err != nil {
		return 0, errors.NewError(err, "Unable to access view")
	}

	return int64(vres.TotalRows), nil
}

func WalkViewInBatches(result chan cb.ViewRow, errs chan errors.Error, bucket *cb.Bucket,
	ddoc string, view string, options map[string]interface{}, batchSize int64, limit int64) {

	if limit != 0 && limit < batchSize {
		batchSize = limit
	}

	defer close(result)
	defer close(errs)

	defer func() {
		r := recover()
		if r != nil {
			clog.Error(fmt.Errorf("View Walking Panic: %v\n%s", r, debug.Stack()))
			errs <- errors.NewError(nil, "Panic In View Walking")
		}
	}()

	options["limit"] = batchSize + 1

	numRead := int64(0)
	ok := true
	for ok {

		logURL, err := bucket.ViewURL(ddoc, view, options)
		if err == nil {
			clog.To(NETWORK_CHANNEL, "Request View: %v", logURL)
		}
		vres, err := bucket.View(ddoc, view, options)
		if err != nil {
			errs <- errors.NewError(err, "Unable to access view")
			return
		}

		for i, row := range vres.Rows {
			if int64(i) < batchSize {
				// dont process the last row, its just used to see if we
				// need to continue processing
				result <- row
				numRead += 1
			}
		}

		if (int64(len(vres.Rows)) > batchSize) && (limit == 0 || (limit != 0 && numRead < limit)) {
			// prepare for next run
			skey := vres.Rows[batchSize].Key
			skeydocid := vres.Rows[batchSize].ID
			options["startkey"] = skey
			options["startkey_docid"] = cb.DocID(skeydocid)
		} else {
			// stop
			ok = false
		}
	}
}

func generateViewOptions(low value.Value, high value.Value, inclusion datastore.Inclusion) map[string]interface{} {
	viewOptions := map[string]interface{}{}

	if low != nil {
		viewOptions["startkey"] = encodeValue(low)
		if inclusion == datastore.NEITHER || inclusion == datastore.HIGH {
			viewOptions["startkey_docid"] = MAX_ID
		}
	}

	if high != nil {
		viewOptions["endkey"] = encodeValue(high)
		if inclusion == datastore.NEITHER || inclusion == datastore.LOW {
			viewOptions["endkey_docid"] = MIN_ID
		}
	}

	return viewOptions
}

func encodeValueAsMapKey(keys value.Values) interface{} {
	rv := make([]interface{}, len(keys))
	for i, lv := range keys {
		val := lv.Actual()
		rv[i] = encodeValue(val)
	}
	return rv
}

func encodeValue(val interface{}) interface{} {
	switch val := val.(type) {
	case nil:
		return []interface{}{TYPE_NULL}
	case bool:
		return []interface{}{TYPE_BOOLEAN, val}
	case float64:
		return []interface{}{TYPE_NUMBER, val}
	case string:
		return []interface{}{TYPE_STRING, encodeStringAsNumericArray(val)}
	case []interface{}:
		return []interface{}{TYPE_ARRAY, val}
	case map[string]interface{}:
		return []interface{}{TYPE_OBJECT, encodeObjectAsCompoundArray(val)}
	default:
		panic(fmt.Sprintf("Unable to encode type %T to map key", val))
	}
}

func encodeStringAsNumericArray(str string) []float64 {
	rv := make([]float64, len(str))
	for i, rune := range str {
		rv[i] = float64(rune)
	}
	return rv
}

func decodeNumericArrayAsString(na []interface{}) (string, error) {
	rv := ""
	for _, num := range na {
		switch num := num.(type) {
		case float64:
			rv = rv + string(rune(num))
		default:
			return "", fmt.Errorf("numeric array contained non-number")
		}
	}
	return rv, nil
}

func encodeObjectAsCompoundArray(obj map[string]interface{}) []interface{} {
	keys := make([]string, len(obj))
	counter := 0
	for k, _ := range obj {
		keys[counter] = k
		counter++
	}
	sort.Strings(keys)
	vals := make([]interface{}, len(obj))
	for i, key := range keys {
		vals[i] = encodeValue(obj[key])
	}
	return []interface{}{keys, vals}
}

func decodeCompoundArrayAsObject(ca []interface{}) (map[string]interface{}, error) {
	rv := map[string]interface{}{}

	if len(ca)%2 != 0 {
		return nil, fmt.Errorf("compound object array must be even length")
	}
	midpoint := len(ca) / 2
	for i := 0; i < midpoint; i++ {
		key, ok := ca[i].(string)
		if !ok {
			return nil, fmt.Errorf("compound object array contains non-string in key position")
		}
		val := ca[i+midpoint]
		// recursively decode object contents
		decodedVal, err := convertCouchbaseViewKeyEntryToValue(val)
		if err != nil {
			return nil, err
		}
		rv[key] = decodedVal
	}
	return rv, nil
}

/*
func convertCouchbaseViewKeyToLookupValue(key interface{}) (value.Value, error) {

	switch key := key.(type) {
	case []interface{}:
		// top-level key MUST be an array
		rv := make(value.Value, len(key))
		for i, keyEntry := range key {
			val, err := convertCouchbaseViewKeyEntryToDparval(keyEntry)
			if err != nil {
				return nil, err
			}
			rv[i] = val
		}
		return rv, nil
	}
	return nil, fmt.Errorf("Couchbase view key top-level MUST be an array")
}
*/

func convertCouchbaseViewKeyEntryToValue(keyEntry interface{}) (value.Value, error) {
	switch keyEntry := keyEntry.(type) {
	case []interface{}:
		// key-entries MUST also be arrays at the top-level
		if len(keyEntry) != 2 {
			return nil, fmt.Errorf("Key entries array must have length 2")
		}
		keyEntryType, ok := keyEntry[0].(float64)
		if !ok {
			return nil, fmt.Errorf("Key entry type must be number")
		}
		switch keyEntryType {
		case TYPE_NULL:
			return value.NewValue(nil), nil
		case TYPE_BOOLEAN, TYPE_NUMBER, TYPE_ARRAY:
			return value.NewValue(keyEntry[1]), nil
		case TYPE_STRING:
			keyStringValue, ok := keyEntry[1].([]interface{})
			if !ok {
				return nil, fmt.Errorf("key entry type string value must be array")
			}
			decodedString, err := decodeNumericArrayAsString(keyStringValue)
			if err != nil {
				return nil, err
			}
			return value.NewValue(decodedString), nil
		case TYPE_OBJECT:
			keyObjectValue, ok := keyEntry[1].([]interface{})
			if !ok {
				return nil, fmt.Errorf("key entry type object value must be array")
			}
			decodedObject, err := decodeCompoundArrayAsObject(keyObjectValue)
			if err != nil {
				return nil, err
			}
			return value.NewValue(decodedObject), nil
		}
		return nil, fmt.Errorf("Unkown type of key entry")
	}
	return nil, fmt.Errorf("Key entries top-level MUST be an array")
}