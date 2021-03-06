//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package execution

import (
	"github.com/couchbase/query/value"
)

// Distincting of input data.
type Distinct struct {
	base
	set     *value.Set
	collect bool
}

const _DISTINCT_CAP = 1024

func NewDistinct(collect bool) *Distinct {
	rv := &Distinct{
		base:    newBase(),
		set:     value.NewSet(_DISTINCT_CAP),
		collect: collect,
	}

	rv.output = rv
	return rv
}

func (this *Distinct) Accept(visitor Visitor) (interface{}, error) {
	return visitor.VisitDistinct(this)
}

func (this *Distinct) Copy() Operator {
	return &Distinct{
		base: this.base.copy(),
		set:  value.NewSet(_DISTINCT_CAP),
	}
}

func (this *Distinct) RunOnce(context *Context, parent value.Value) {
	this.runConsumer(this, context, parent)
}

func (this *Distinct) processItem(item value.AnnotatedValue, context *Context) bool {
	p := item.GetAttachment("projection")
	if p == nil {
		p = item
	}

	this.set.Put(p.(value.Value), item)
	return true
}

func (this *Distinct) afterItems(context *Context) {
	if this.collect {
		return
	}

	for _, av := range this.set.Values() {
		if !this.sendItem(value.NewAnnotatedValue(av)) {
			return
		}
	}

	this.set = nil
}

func (this *Distinct) Set() *value.Set {
	return this.set
}
