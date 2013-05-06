// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"github.com/globocom/tsuru/app/bind"
	"github.com/globocom/tsuru/provision"
	"launchpad.net/gocheck"
	"sort"
)

func (s *S) TestUnitGetName(c *gocheck.C) {
	u := Unit{Name: "abcdef", app: &App{Name: "2112"}}
	c.Assert(u.GetName(), gocheck.Equals, "abcdef")
}

func (s *S) TestUnitGetMachine(c *gocheck.C) {
	u := Unit{Machine: 10}
	c.Assert(u.GetMachine(), gocheck.Equals, u.Machine)
}

func (s *S) TestUnitGetStatus(c *gocheck.C) {
	var tests = []struct {
		input    string
		expected provision.Status
	}{
		{"started", provision.StatusStarted},
		{"pending", provision.StatusPending},
		{"creating", provision.StatusCreating},
		{"down", provision.StatusDown},
		{"error", provision.StatusError},
		{"installing", provision.StatusInstalling},
		{"creating", provision.StatusCreating},
	}
	for _, test := range tests {
		u := Unit{State: test.input}
		got := u.GetStatus()
		if got != test.expected {
			c.Errorf("u.GetStatus(): want %q, got %q.", test.expected, got)
		}
	}
}

func (s *S) TestUnitShouldBeABinderUnit(c *gocheck.C) {
	var _ bind.Unit = &Unit{}
}

func (s *S) TestUnitSliceLen(c *gocheck.C) {
	units := UnitSlice{Unit{}, Unit{}}
	c.Assert(units.Len(), gocheck.Equals, 2)
}

func (s *S) TestUnitSliceLess(c *gocheck.C) {
	units := UnitSlice{
		Unit{Name: "a", State: string(provision.StatusError)},
		Unit{Name: "b", State: string(provision.StatusDown)},
		Unit{Name: "c", State: string(provision.StatusPending)},
		Unit{Name: "d", State: string(provision.StatusCreating)},
		Unit{Name: "e", State: string(provision.StatusInstalling)},
		Unit{Name: "f", State: string(provision.StatusStarted)},
	}
	c.Assert(units.Less(0, 1), gocheck.Equals, true)
	c.Assert(units.Less(1, 2), gocheck.Equals, true)
	c.Assert(units.Less(2, 3), gocheck.Equals, true)
	c.Assert(units.Less(4, 5), gocheck.Equals, true)
	c.Assert(units.Less(5, 0), gocheck.Equals, false)
}

func (s *S) TestUnitSliceSwap(c *gocheck.C) {
	units := UnitSlice{
		Unit{Name: "b", State: string(provision.StatusDown)},
		Unit{Name: "c", State: string(provision.StatusPending)},
		Unit{Name: "a", State: string(provision.StatusError)},
		Unit{Name: "d", State: string(provision.StatusCreating)},
		Unit{Name: "e", State: string(provision.StatusInstalling)},
		Unit{Name: "f", State: string(provision.StatusStarted)},
	}
	units.Swap(0, 2)
	c.Assert(units.Less(0, 2), gocheck.Equals, true)
}

func (s *S) TestUnitSliceSort(c *gocheck.C) {
	units := UnitSlice{
		Unit{Name: "b", State: string(provision.StatusDown)},
		Unit{Name: "c", State: string(provision.StatusPending)},
		Unit{Name: "a", State: string(provision.StatusError)},
		Unit{Name: "d", State: string(provision.StatusCreating)},
		Unit{Name: "e", State: string(provision.StatusInstalling)},
		Unit{Name: "f", State: string(provision.StatusStarted)},
	}
	c.Assert(sort.IsSorted(units), gocheck.Equals, false)
	sort.Sort(units)
	c.Assert(sort.IsSorted(units), gocheck.Equals, true)
}

func (s *S) TestGenerateUnitQuotaItem(c *gocheck.C) {
	var tests = []struct {
		app  *App
		want string
	}{
		{&App{Name: "black"}, "black-0"},
		{&App{Name: "black", Units: []Unit{{QuotaItem: "black-1"}, {QuotaItem: "black-5"}}}, "black-6"},
		{&App{Name: "white", Units: []Unit{{QuotaItem: "white-9"}}}, "white-10"},
		{&App{}, "-0"},
		{&App{Name: "white", Units: []Unit{{Name: "white/0"}}}, "white-0"},
		{&App{Name: "white", Units: []Unit{{QuotaItem: "white-w"}}}, "white-0"},
	}
	for _, t := range tests {
		got := generateUnitQuotaItem(t.app)
		c.Check(got, gocheck.Equals, t.want)
	}
}
