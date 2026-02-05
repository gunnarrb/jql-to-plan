// Package omniplan provides types and utilities for serializing project plans
// to OmniPlan 4 XML format.
package omniplan

import (
	"encoding/xml"
	"fmt"
	"sync/atomic"
)

// Namespace is the OmniPlan v2 XML namespace
const Namespace = "http://www.omnigroup.com/namespace/OmniPlan/v2"

// Scenario is the root element of an OmniPlan document
type Scenario struct {
	XMLName       xml.Name       `xml:"scenario"`
	XMLNS         string         `xml:"xmlns,attr"`
	OPNS          string         `xml:"xmlns:opns,attr"`
	ID            string         `xml:"id,attr"`
	Granularity   string         `xml:"granularity"`
	TopResource   Reference      `xml:"top-resource"`
	Resources     []Resource     `xml:"resource"`
	TopTask       Reference      `xml:"top-task"`
	Tasks         []Task         `xml:"task"`
	CriticalPaths []CriticalPath `xml:"critical-path"`
}

// Reference holds an idref attribute for linking elements
type Reference struct {
	IDRef string `xml:"idref,attr"`
}

// Resource represents a project resource (staff, equipment, etc.)
type Resource struct {
	ID             string      `xml:"id,attr"`
	Name           string      `xml:"name,omitempty"`
	Type           string      `xml:"type,omitempty"`
	ChildResources []Reference `xml:"child-resource,omitempty"`
}

// Task represents a project task
type Task struct {
	ID            string             `xml:"id,attr"`
	Title         string             `xml:"title,omitempty"`
	Type          string             `xml:"type,omitempty"`
	LeveledStart  string             `xml:"leveled-start,omitempty"`
	Effort        int64              `xml:"effort,omitempty"`
	Recalculate   string             `xml:"recalculate,omitempty"`
	StaticCost    int                `xml:"static-cost"`
	ChildTasks    []Reference        `xml:"child-task,omitempty"`
	UserData      *UserData          `xml:"user-data,omitempty"`
	Prerequisites []PrerequisiteTask `xml:"prerequisite-task,omitempty"`
	Assignments   []Reference        `xml:"assignment,omitempty"`
	Note          *Note              `xml:"note,omitempty"`
}

// PrerequisiteTask represents a task dependency
type PrerequisiteTask struct {
	IDRef string `xml:"idref,attr"`
	Kind  string `xml:"kind,attr,omitempty"`
}

// UserData holds custom key-value data for a task
type UserData struct {
	Items []UserDataItem
}

// UserDataItem is a key-value pair in user data
type UserDataItem struct {
	Key   string
	Value string
}

// MarshalXML implements custom marshaling for UserData to match OmniPlan format
func (u UserData) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if len(u.Items) == 0 {
		return nil
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, item := range u.Items {
		if err := e.EncodeElement(item.Key, xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
			return err
		}
		if err := e.EncodeElement(item.Value, xml.StartElement{Name: xml.Name{Local: "string"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// Note represents a rich text note on a task
type Note struct {
	Text NoteText `xml:"text"`
}

// NoteText contains the text content of a note
type NoteText struct {
	Paragraphs []NoteParagraph `xml:"p"`
}

// NoteParagraph represents a paragraph in a note
type NoteParagraph struct {
	Run NoteRun `xml:"run"`
}

// NoteRun contains the literal text
type NoteRun struct {
	Literal string `xml:"lit"`
}

// CriticalPath represents a critical path configuration
type CriticalPath struct {
	Root      string `xml:"root,attr"`
	Enabled   string `xml:"enabled,attr"`
	Resources string `xml:"resources,attr"`
	Color     *Color `xml:"color,omitempty"`
}

// Color represents an sRGB color
type Color struct {
	Space string  `xml:"space,attr"`
	R     float64 `xml:"r,attr,omitempty"`
	G     float64 `xml:"g,attr,omitempty"`
	B     float64 `xml:"b,attr,omitempty"`
	H     float64 `xml:"h,attr,omitempty"`
	S     float64 `xml:"s,attr,omitempty"`
	V     float64 `xml:"v,attr,omitempty"`
}

// NewNote creates a Note with the given text
func NewNote(text string) *Note {
	return &Note{
		Text: NoteText{
			Paragraphs: []NoteParagraph{
				{Run: NoteRun{Literal: text}},
			},
		},
	}
}

// Package level counter for generating unique IDs
var idCounter atomic.Int64

// GenerateID creates a unique ID for OmniPlan elements
func GenerateID() string {
	// OmniPlan uses alphanumeric IDs like "fimc6xeFjk1" or "dFRdv9nCq3E"
	// For simplicity, we'll use a counter-based approach with a prefix
	return fmt.Sprintf("gen%d", idCounter.Add(1))
}
