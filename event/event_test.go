package event

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/db/dbtest"
	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func Test(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (s *S) SetUpSuite(c *check.C) {
	config.Set("database:url", "127.0.0.1:27017")
	config.Set("database:name", "tsuru_events_tests")
}

func (s *S) SetUpTest(c *check.C) {
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	err = dbtest.ClearAllCollections(conn.Events().Database)
	c.Assert(err, check.IsNil)
}

func (s *S) TestNewEventDone(c *check.C) {
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	c.Assert(evt.StartTime.IsZero(), check.Equals, false)
	c.Assert(evt.LockUpdateTime.IsZero(), check.Equals, false)
	expected := &Event{eventData: eventData{
		ID:             eventId{target: EventTarget{Name: "app", Value: "myapp"}},
		Target:         EventTarget{Name: "app", Value: "myapp"},
		Kind:           "env-set",
		Owner:          "me@me.com",
		Running:        true,
		StartTime:      evt.StartTime,
		LockUpdateTime: evt.LockUpdateTime,
	}}
	c.Assert(evt, check.DeepEquals, expected)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].StartTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].LockUpdateTime.IsZero(), check.Equals, false)
	evts[0].StartTime = expected.StartTime
	evts[0].LockUpdateTime = expected.LockUpdateTime
	c.Assert(evts, check.DeepEquals, []Event{*expected})
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	evts, err = AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].StartTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].LockUpdateTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].EndTime.IsZero(), check.Equals, false)
	evts[0].EndTime = time.Time{}
	evts[0].StartTime = expected.StartTime
	evts[0].LockUpdateTime = expected.LockUpdateTime
	expected.Running = false
	expected.ID = eventId{objId: evts[0].ID.objId}
	c.Assert(evts, check.DeepEquals, []Event{*expected})
}

func (s *S) TestNewEventCustomDataDone(c *check.C) {
	customData := struct{ A string }{A: "value"}
	evt, err := NewEventCustomData(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com", customData)
	c.Assert(err, check.IsNil)
	c.Assert(evt.StartTime.IsZero(), check.Equals, false)
	c.Assert(evt.LockUpdateTime.IsZero(), check.Equals, false)
	expected := &Event{eventData: eventData{
		ID:              eventId{target: EventTarget{Name: "app", Value: "myapp"}},
		Target:          EventTarget{Name: "app", Value: "myapp"},
		Kind:            "env-set",
		Owner:           "me@me.com",
		Running:         true,
		StartTime:       evt.StartTime,
		LockUpdateTime:  evt.LockUpdateTime,
		StartCustomData: customData,
	}}
	c.Assert(evt, check.DeepEquals, expected)
	customData = struct{ A string }{A: "other"}
	err = evt.DoneCustomData(nil, customData)
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].StartTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].LockUpdateTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].EndTime.IsZero(), check.Equals, false)
	evts[0].EndTime = time.Time{}
	evts[0].StartTime = expected.StartTime
	evts[0].LockUpdateTime = expected.LockUpdateTime
	expected.Running = false
	expected.ID = eventId{objId: evts[0].ID.objId}
	expected.StartCustomData = bson.M{"a": "value"}
	expected.EndCustomData = bson.M{"a": "other"}
	c.Assert(evts, check.DeepEquals, []Event{*expected})
}

func (s *S) TestNewEventLocks(c *check.C) {
	_, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	_, err = NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-unset", "other@other.com")
	c.Assert(err, check.ErrorMatches, `event locked: app\(myapp\) running "env-set" start by me@me.com at .+`)
}

func (s *S) TestNewEventLockExpired(c *check.C) {
	oldLockExpire := lockExpireTimeout
	lockExpireTimeout = time.Millisecond
	defer func() {
		lockExpireTimeout = oldLockExpire
	}()
	_, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	updater.stop()
	time.Sleep(100 * time.Millisecond)
	_, err = NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-unset", "other@other.com")
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 2)
	c.Assert(evts[0].Kind, check.Equals, "env-set")
	c.Assert(evts[1].Kind, check.Equals, "env-unset")
	c.Assert(evts[0].Running, check.Equals, false)
	c.Assert(evts[1].Running, check.Equals, true)
	c.Assert(evts[0].Error, check.Matches, `event expired, no update for [\d.]+\w+`)
	c.Assert(evts[1].Error, check.Equals, "")
}

func (s *S) TestUpdaterUpdatesAndStopsUpdating(c *check.C) {
	updater.stop()
	oldUpdateInterval := lockUpdateInterval
	lockUpdateInterval = time.Millisecond
	defer func() {
		lockUpdateInterval = oldUpdateInterval
	}()
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	t0 := evts[0].LockUpdateTime
	time.Sleep(100 * time.Millisecond)
	evts, err = AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	t1 := evts[0].LockUpdateTime
	c.Assert(t0.Before(t1), check.Equals, true)
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	time.Sleep(100 * time.Millisecond)
	evts, err = AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	t0 = evts[0].LockUpdateTime
	time.Sleep(100 * time.Millisecond)
	evts, err = AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	t1 = evts[0].LockUpdateTime
	c.Assert(t0, check.DeepEquals, t1)
}

func (s *S) TestEventAbort(c *check.C) {
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	err = evt.Abort()
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 0)
}

func (s *S) TestEventDoneError(c *check.C) {
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	err = evt.Done(errors.New("myerr"))
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].StartTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].LockUpdateTime.IsZero(), check.Equals, false)
	c.Assert(evts[0].EndTime.IsZero(), check.Equals, false)
	expected := &Event{eventData: eventData{
		ID:             eventId{objId: evts[0].ID.objId},
		Target:         EventTarget{Name: "app", Value: "myapp"},
		Kind:           "env-set",
		Owner:          "me@me.com",
		StartTime:      evts[0].StartTime,
		LockUpdateTime: evts[0].LockUpdateTime,
		EndTime:        evts[0].EndTime,
		Error:          "myerr",
	}}
	c.Assert(evts, check.DeepEquals, []Event{*expected})
}

func (s *S) TestEventLogf(c *check.C) {
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	evt.Logf("%s %d", "hey", 42)
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].Log, check.Equals, "hey 42\n")
}

func (s *S) TestEventLogfWithWriter(c *check.C) {
	evt, err := NewEvent(EventTarget{Name: "app", Value: "myapp"}, "env-set", "me@me.com")
	c.Assert(err, check.IsNil)
	buf := bytes.Buffer{}
	evt.SetLogWriter(&buf)
	evt.Logf("%s %d", "hey", 42)
	c.Assert(buf.String(), check.Equals, "hey 42\n")
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	evts, err := AllEvents()
	c.Assert(err, check.IsNil)
	c.Assert(evts, check.HasLen, 1)
	c.Assert(evts[0].Log, check.Equals, "hey 42\n")
}
