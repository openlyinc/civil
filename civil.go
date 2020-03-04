// Copyright 2016 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package civil implements types for civil time, a time-zone-independent
// representation of time that follows the rules of the proleptic
// Gregorian calendar with exactly 24-hour days, 60-minute hours, and 60-second
// minutes.
//
// Because they lack location information, these types do not represent unique
// moments or intervals of time. Use time.Time for that purpose.
package civil

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// A Date represents a date (year, month, day).
//
// This type does not include location information, and therefore does not
// describe a unique 24-hour timespan.
type Date struct {
	Year  int        // Year (e.g., 2014).
	Month time.Month // Month of the year (January = 1, ...).
	Day   int        // Day of the month, starting at 1.
}

// DateOf returns the Date in which a time occurs in that time's location.
func DateOf(t time.Time) Date {
	var d Date
	d.Year, d.Month, d.Day = t.Date()
	return d
}

// RFC3339Date is the civil date format of RFC3339
const RFC3339Date = "2006-01-02"

// ParseDate parses a string in RFC3339 full-date format and returns the date value it represents.
func ParseDate(s string) (Date, error) {
	const dateZero = "0000-00-00"

	if s == dateZero {
		return Date{}, nil
	}
	t, err := time.Parse(RFC3339Date, s)
	if err != nil {
		return Date{}, err
	}
	return DateOf(t), nil
}

// String returns the date in RFC3339 full-date format.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// IsValid reports whether the date is valid.
func (d Date) IsValid() bool {
	return DateOf(d.In(time.UTC)) == d
}

// In returns the time corresponding to time 00:00:00 of the date in the location.
//
// In is always consistent with time.Date, even when time.Date returns a time
// on a different day. For example, if loc is America/Indiana/Vincennes, then both
//     time.Date(1955, time.May, 1, 0, 0, 0, 0, loc)
// and
//     civil.Date{Year: 1955, Month: time.May, Day: 1}.In(loc)
// return 23:00:00 on April 30, 1955.
//
// In panics if loc is nil.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// AddDays returns the date that is n days in the future.
// n can also be negative to go into the past.
func (d Date) AddDays(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(0, 0, n))
}

// AddMonths returns the date that is n months in the future.
// n can also be negative to go into the past.
func (d Date) AddMonths(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(0, n, 0))
}

// AddYears returns the date that is n years in the future.
// n can also be negative to go into the past.
func (d Date) AddYears(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(n, 0, 0))
}

// DaysSince returns the signed number of days between the date and s, not including the end day.
// This is the inverse operation to AddDays.
func (d Date) DaysSince(s Date) (days int) {
	// We convert to Unix time so we do not have to worry about leap seconds:
	// Unix time increases by exactly 86400 seconds per day.
	deltaUnix := d.In(time.UTC).Unix() - s.In(time.UTC).Unix()
	return int(deltaUnix / 86400)
}

// Before reports whether d1 occurs before d2.
func (d1 Date) Before(d2 Date) bool {
	if d1.Year != d2.Year {
		return d1.Year < d2.Year
	}
	if d1.Month != d2.Month {
		return d1.Month < d2.Month
	}
	return d1.Day < d2.Day
}

// After reports whether d1 occurs after d2.
func (d1 Date) After(d2 Date) bool {
	return d2.Before(d1)
}

// MarshalText implements the encoding.TextMarshaler interface.
// The output is the result of d.String().
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The date is expected to be a string in a format accepted by ParseDate.
func (d *Date) UnmarshalText(data []byte) error {
	var err error
	*d, err = ParseDate(string(data))
	return err
}

// UnmarshalJSON implements encoding/json Unmarshaler interface
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("date should be a string, got %s", data)
	}
	val, err := ParseDate(s)
	if err != nil {
		return fmt.Errorf("invalid date, data: %s, err: %v", s, err)
	}
	*d = val
	return nil
}

// MarshalJSON implements encoding/json Marshaler interface
func (d *Date) MarshalJSON() ([]byte, error) {
	if y := d.Year; y < 0 || y >= 10000 {
		// RFC 3339 is clear that years are 4 digits exactly.
		// See golang.org/issue/4556#c15 for more discussion.
		return nil, fmt.Errorf("Date.MarshalJSON: year '%v' outside of range [0,9999]", y)
	}

	b := make([]byte, 0, len(RFC3339Date)+2)
	b = append(b, '"')
	b = append(b, d.String()...)
	b = append(b, '"')
	return b, nil
}

// Value implements the database/sql/driver valuer interface.
func (d Date) Value() (driver.Value, error) {
	return d.String(), nil
}

// Scan implements the database/sql scanner interface.
func (d *Date) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		t, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("'%s' could not be converted into a valid type", str)
		}

		val := DateOf(t)
		*d = val
	} else {
		val, err := ParseDate(str)
		if err != nil {
			return err
		}
		*d = val
	}

	return nil
}

// A Time represents a time with nanosecond precision.
//
// This type does not include location information, and therefore does not
// describe a unique moment in time.
//
// This type exists to represent the TIME type in storage-based APIs like BigQuery.
// Most operations on Times are unlikely to be meaningful. Prefer the DateTime type.
type Time struct {
	Hour       int // The hour of the day in 24-hour format; range [0-23]
	Minute     int // The minute of the hour; range [0-59]
	Second     int // The second of the minute; range [0-59]
	Nanosecond int // The nanosecond of the second; range [0-999999999]
}

// TimeOf returns the Time representing the time of day in which a time occurs
// in that time's location. It ignores the date.
func TimeOf(t time.Time) Time {
	var tm Time
	tm.Hour, tm.Minute, tm.Second = t.Clock()
	tm.Nanosecond = t.Nanosecond()
	return tm
}

// RFC3339Time is the civil time format of RFC3339
const RFC3339Time = "15:04:05.999999999"

// ParseTime parses a string and returns the time value it represents.
// ParseTime accepts an extended form of the RFC3339 partial-time format. After
// the HH:MM:SS part of the string, an optional fractional part may appear,
// consisting of a decimal point followed by one to nine decimal digits.
// (RFC3339 admits only one digit after the decimal point).
func ParseTime(s string) (Time, error) {
	t, err := time.Parse(RFC3339Time, s)
	if err != nil {
		return Time{}, err
	}
	return TimeOf(t), nil
}

// String returns the date in the format described in ParseTime. If Nanoseconds
// is zero, no fractional part will be generated. Otherwise, the result will
// end with a fractional part consisting of a decimal point and nine digits.
func (t Time) String() string {
	s := fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	if t.Nanosecond == 0 {
		return s
	}
	return s + fmt.Sprintf(".%09d", t.Nanosecond)
}

// IsValid reports whether the time is valid.
func (t Time) IsValid() bool {
	// Construct a non-zero time.
	tm := time.Date(2, 2, 2, t.Hour, t.Minute, t.Second, t.Nanosecond, time.UTC)
	return TimeOf(tm) == t
}

// MarshalText implements the encoding.TextMarshaler interface.
// The output is the result of t.String().
func (t Time) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The time is expected to be a string in a format accepted by ParseTime.
func (t *Time) UnmarshalText(data []byte) error {
	var err error
	*t, err = ParseTime(string(data))
	return err
}

// UnmarshalJSON implements encoding/json Unmarshaler interface
func (t *Time) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("time should be a string, got %s", data)
	}
	val, err := ParseTime(s)
	if err != nil {
		return fmt.Errorf("invalid time: %v", err)
	}
	*t = val
	return nil
}

// MarshalJSON implements encoding/json Marshaler interface
func (t *Time) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(RFC3339Time)+2)
	b = append(b, '"')
	b = append(b, t.String()...)
	b = append(b, '"')
	return b, nil
}

// Value implements the database/sql/driver valuer interface.
func (t Time) Value() (driver.Value, error) {
	return t.String(), nil
}

// Scan implements the database/sql scanner interface.
func (t *Time) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		tm, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("'%s' could not be converted into a valid type", str)
		}

		val := TimeOf(tm)
		*t = val
	} else {
		val, err := ParseTime(str)
		if err != nil {
			return err
		}
		*t = val
	}

	return nil
}

// A DateTime represents a date and time.
//
// This type does not include location information, and therefore does not
// describe a unique moment in time.
type DateTime struct {
	Date Date
	Time Time
}

// Note: We deliberately do not embed Date into DateTime, to avoid promoting AddDays and Sub.

// DateTimeOf returns the DateTime in which a time occurs in that time's location.
func DateTimeOf(t time.Time) DateTime {
	return DateTime{
		Date: DateOf(t),
		Time: TimeOf(t),
	}
}

// RFC3339Time is the civil datetime format of RFC3339
const RFC3339DateTime = "2006-01-02T15:04:05.999999999"

// ParseDateTime parses a string and returns the DateTime it represents.
// ParseDateTime accepts a variant of the RFC3339 date-time format that omits
// the time offset but includes an optional fractional time, as described in
// ParseTime. Informally, the accepted format is
//     YYYY-MM-DDTHH:MM:SS[.FFFFFFFFF]
// where the 'T' may be a lower-case 't'.
func ParseDateTime(s string) (DateTime, error) {
	t, err := time.Parse(RFC3339DateTime, s)
	if err != nil {
		t, err = time.Parse("2006-01-02t15:04:05.999999999", s)
		if err != nil {
			return DateTime{}, err
		}
	}
	return DateTimeOf(t), nil
}

// String returns the date in the format described in ParseDate.
func (dt DateTime) String() string {
	return dt.Date.String() + "T" + dt.Time.String()
}

// IsValid reports whether the datetime is valid.
func (dt DateTime) IsValid() bool {
	return dt.Date.IsValid() && dt.Time.IsValid()
}

// In returns the time corresponding to the DateTime in the given location.
//
// If the time is missing or ambiguous at the location, In returns the same
// result as time.Date. For example, if loc is America/Indiana/Vincennes, then
// both
//     time.Date(1955, time.May, 1, 0, 30, 0, 0, loc)
// and
//     civil.DateTime{
//         civil.Date{Year: 1955, Month: time.May, Day: 1}},
//         civil.Time{Minute: 30}}.In(loc)
// return 23:30:00 on April 30, 1955.
//
// In panics if loc is nil.
func (dt DateTime) In(loc *time.Location) time.Time {
	return time.Date(dt.Date.Year, dt.Date.Month, dt.Date.Day, dt.Time.Hour, dt.Time.Minute, dt.Time.Second, dt.Time.Nanosecond, loc)
}

// Before reports whether dt1 occurs before dt2.
func (dt1 DateTime) Before(dt2 DateTime) bool {
	return dt1.In(time.UTC).Before(dt2.In(time.UTC))
}

// After reports whether dt1 occurs after dt2.
func (dt1 DateTime) After(dt2 DateTime) bool {
	return dt2.Before(dt1)
}

// MarshalText implements the encoding.TextMarshaler interface.
// The output is the result of dt.String().
func (dt DateTime) MarshalText() ([]byte, error) {
	return []byte(dt.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The datetime is expected to be a string in a format accepted by ParseDateTime
func (dt *DateTime) UnmarshalText(data []byte) error {
	var err error
	*dt, err = ParseDateTime(string(data))
	return err
}

// UnmarshalJSON implements encoding/json Unmarshaler interface
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	if dt == nil {
		return errors.New("nil receiver")
	}
	if tIdx := bytes.IndexAny(data, "Tt"); tIdx > 0 {
		dataDate := data[:tIdx+1] // `"` to the `T`
		dataDate[tIdx] = byte('"')
		dataTime := data[tIdx:] // `T` to the end `"`
		dataTime[0] = byte('"')

		if err := dt.Date.UnmarshalJSON(dataDate); err != nil {
			return errors.Wrapf(err, "date prefix (%s) in '%s' could not be converted", dataDate, data)
		}
		if err := dt.Time.UnmarshalJSON(dataTime); err != nil {
			return errors.Wrapf(err, "time suffix (%s) in '%s' could not be converted", dataTime, data)
		}
		return nil
	}

	// If here, let the original code try to decode
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("datetime should be a string, got %s", data)
	}
	val, err := ParseDateTime(s)
	if err != nil {
		return fmt.Errorf("invalid datetime: %v", err)
	}
	*dt = val
	return nil
}

// MarshalJSON implements encoding/json Marshaler interface
func (dt *DateTime) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(RFC3339DateTime)+2)
	b = append(b, '"')
	b = append(b, dt.String()...)
	b = append(b, '"')
	return b, nil
}

// Value implements the database/sql/driver valuer interface.
func (dt DateTime) Value() (driver.Value, error) {
	return dt.String(), nil
}

// Scan implements the database/sql scanner interface.
func (dt *DateTime) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		t, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("'%s' could not be converted into a valid type", str)
		}

		val := DateTimeOf(t)
		*dt = val
	} else {
		val, err := ParseDateTime(str)
		if err != nil {
			return err
		}
		*dt = val
	}

	return nil
}
