package captainslog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SyslogMsg holds an Unmarshaled rfc3164 message.
type SyslogMsg struct {
	Pri                 Priority
	Time                time.Time
	Host                string
	Tag                 Tag
	Cee                 string
	IsJSON              bool
	IsCee               bool
	optionDontParseJSON bool
	Content             string
	timeFormat          string
	JSONValues          map[string]interface{}
	mutex               *sync.Mutex
}

// Content holds the Content of a syslog message,
// including the Content as a string, and a struct of
// the JSONValues of appropriate.
type Content struct {
	Content    string
	JSONValues map[string]interface{}
}

// Time holds both the time derviced from a
// syslog message along with the time format string
// used to parse it.
type Time struct {
	Time       time.Time
	TimeFormat string
}

// Tag holds the data derviced from a
// syslog message's tag, including the full
// tag, the program name and the pid.
type Tag struct {
	Program  string
	Pid      string
	HasColon bool
}

func NewTag() *Tag {
	return &Tag{
		HasColon: true,
	}
}

// String converts the specified Tag into a string.
func (t Tag) String() string {
	colon := ""
	if t.HasColon {
		colon = ":"
	}

	if len(t.Pid) == 0 {
		return t.Program + colon
	}

	return fmt.Sprintf("%s[%s]%s", t.Program, t.Pid, colon)
}

// NewSyslogMsg creates a new empty SyslogMsg.
func NewSyslogMsg() SyslogMsg {
	return SyslogMsg{
		JSONValues: make(map[string]interface{}),
		mutex:      &sync.Mutex{},
	}
}

// NewSyslogMsgFromBytes accepts a []byte containing an RFC3164
// message and returns a SyslogMsg. If the original RFC3164
// message is a CEE enhanced message, the JSON will be
// parsed into the JSONValues map[string]inferface{}
func NewSyslogMsgFromBytes(b []byte, options ...func(*Parser)) (SyslogMsg, error) {
	p := NewParser(options...)
	msg, err := p.ParseBytes(b)
	return msg, err
}

// SetFacility accepts a captainslog.Facility to
// set the facility of the SyslogMsg
func (s *SyslogMsg) SetFacility(f Facility) error {
	return s.Pri.SetFacility(f)
}

// SetSeverity accepts a captainslog.Severity to set the
// severity of the SyslogMsg
func (s *SyslogMsg) SetSeverity(sv Severity) error {
	return s.Pri.SetSeverity(sv)
}

// SetTime accepts a time.Time to set the time
// of the SyslogMsg
func (s *SyslogMsg) SetTime(t time.Time) {
	s.Time = t
}

// SetProgram accepts a string to set the programname
// of the SyslogMsg
func (s *SyslogMsg) SetProgram(p string) {
	s.Tag.Program = p
}

// SetPid accepts a string to set the pid of
// the SyslogMsg
func (s *SyslogMsg) SetPid(p string) {
	s.Tag.Pid = p
}

// SetHost accepts a string to set the host
// of the SyslogMsg
func (s *SyslogMsg) SetHost(h string) {
	s.Host = h
}

// SetContent accepts a string to set the  content
// of the SyslogMsg. It will check to see if the
// string is JSON and try to parse it if so.
func (s *SyslogMsg) SetContent(c string) error {
	_, content, err := ParseContent([]byte(c), ContentOptionParseJSON)
	s.Content = content.Content
	s.JSONValues = content.JSONValues
	return err
}

// AddTagArray adds a tag to an array of tags at the key. If the key
// does not already exist, it will create the key and initially it
// to a []interface{}.
func (s *SyslogMsg) AddTagArray(key string, value interface{}) error {
	if _, ok := s.JSONValues[key]; !ok {
		s.JSONValues[key] = make([]interface{}, 0)
	}

	switch val := s.JSONValues[key].(type) {
	case []interface{}:
		s.JSONValues[key] = append(val, value)
		if !s.IsCee {
			s.IsCee = true
			s.Cee = " @cee:"
			s.JSONValues["msg"] = s.Content[1:]
		}
		return nil
	default:
		return fmt.Errorf("tags key in message was not an array")
	}
}

// AddTag adds a tag to the value at key. If the key exists,
// the value currently at the key will be overwritten.
func (s *SyslogMsg) AddTag(key string, value interface{}) {
	s.JSONValues[key] = value
}

// String returns the SyslogMsg as an RFC3164 string.
func (s *SyslogMsg) String() string {
	var content string
	if s.IsJSON && !s.optionDontParseJSON {
		b, err := json.Marshal(s.JSONValues)
		if err != nil {
			panic(err)
		}
		content = string(b)
	} else {
		if len(s.JSONValues) > 0 {
			s.JSONValues["msg"] = strings.TrimLeft(s.Content, " ")
			s.IsCee = true
			s.Cee = " @cee:"
			b, err := json.Marshal(s.JSONValues)
			if err != nil {
				panic(err)
			}
			content = string(b)
		} else {
			content = s.Content
		}
	}
	return fmt.Sprintf("<%s>%s %s %s%s%s\n", s.Pri, s.Time.Format(s.timeFormat), s.Host, s.Tag, s.Cee, content)
}

// Bytes returns the SyslogMsg as RFC3164 []byte.
func (s *SyslogMsg) Bytes() []byte {
	return []byte(s.String())
}

// JSON returns a JSON representation of the message encoded in a []byte. Syslog fields are named with
// a "syslog_" prefix to avoid potential collision with fields from the message body.
func (s *SyslogMsg) JSON() ([]byte, error) {
	content := make(map[string]interface{})
	if s.optionDontParseJSON && s.IsCee {
		decoder := json.NewDecoder(bytes.NewBuffer([]byte(s.Content)))
		decoder.UseNumber()
		err := decoder.Decode(&content)
		if err != nil {
			return []byte(""), err
		}

	}
	for key, value := range s.JSONValues {
		content[key] = value
	}

	content["syslog_time"] = s.Time
	content["syslog_host"] = s.Host
	content["syslog_tag"] = s.Tag.String()
	content["syslog_programname"] = s.Tag.Program
	content["syslog_pid"] = s.Tag.Pid
	content["syslog_facilitytext"] = s.Pri.Facility.String()
	content["syslog_severitytext"] = s.Pri.Severity.String()

	if !s.IsCee {
		content["syslog_content"] = s.Content
	}

	b, err := json.Marshal(content)
	return b, err
}
