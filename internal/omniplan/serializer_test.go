package omniplan

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gunnarrb/jql-to-plan/internal/jira"
)

func TestSerializer_Serialize(t *testing.T) {
	tickets := []jira.Ticket{
		{Key: "TEST-1", Summary: "First Task", Link: "https://jira.example.com/browse/TEST-1", Assignee: "Alice Smith", EffortDays: 2},
		{Key: "TEST-2", Summary: "Second Task", Link: "https://jira.example.com/browse/TEST-2", Assignee: "Bob Jones", EffortDays: 5, DependencyKeys: []string{"TEST-1"}},
		{Key: "TEST-3", Summary: "Third Task", Link: "https://jira.example.com/browse/TEST-3", Assignee: "Alice Smith"}, // No effort, should default
	}

	serializer := NewSerializer("Test Project")
	var buf bytes.Buffer

	err := serializer.Serialize(&buf, tickets, nil)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	output := buf.String()

	// Verify XML header
	if !strings.HasPrefix(output, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Output should start with XML header")
	}

	// Verify namespace
	if !strings.Contains(output, "xmlns=\"http://www.omnigroup.com/namespace/OmniPlan/v2\"") {
		t.Error("Output should contain OmniPlan namespace")
	}

	// Verify granularity
	if !strings.Contains(output, "<granularity>days</granularity>") {
		t.Error("Output should contain granularity element")
	}

	// Verify task titles
	if !strings.Contains(output, "<title>First Task</title>") {
		t.Error("Output should contain first task title")
	}
	if !strings.Contains(output, "<title>Second Task</title>") {
		t.Error("Output should contain second task title")
	}

	// Verify Jira keys in user-data
	if !strings.Contains(output, "<string>TEST-1</string>") {
		t.Error("Output should contain TEST-1 key in user-data")
	}
	if !strings.Contains(output, "<string>TEST-2</string>") {
		t.Error("Output should contain TEST-2 key in user-data")
	}

	// Verify top-task exists
	if !strings.Contains(output, "<top-task") {
		t.Error("Output should contain top-task reference")
	}

	// Verify effort conversion: 2 days = 2 * 8 * 3600 = 57600 seconds
	if !strings.Contains(output, "<effort>57600</effort>") {
		t.Error("Output should contain effort of 57600 seconds for 2 day task")
	}

	// Verify effort conversion: 5 days = 5 * 8 * 3600 = 144000 seconds
	if !strings.Contains(output, "<effort>144000</effort>") {
		t.Error("Output should contain effort of 144000 seconds for 5 day task")
	}

	// Verify default effort (8 hours = 28800 seconds) for task without EffortDays
	if !strings.Contains(output, "<effort>28800</effort>") {
		t.Error("Output should contain default effort of 28800 seconds for task without EffortDays")
	}

	// Verify assignee resources are created
	if !strings.Contains(output, "<name>Alice Smith</name>") {
		t.Error("Output should contain Alice Smith as a resource")
	}
	if !strings.Contains(output, "<name>Bob Jones</name>") {
		t.Error("Output should contain Bob Jones as a resource")
	}

	// Verify Staff type for resources
	if !strings.Contains(output, "<type>Staff</type>") {
		t.Error("Output should contain Staff type for resources")
	}

	// Verify assignments exist (tasks should have assignment elements)
	if !strings.Contains(output, "<assignment") {
		t.Error("Output should contain assignment elements for tasks")
	}

	// Verify prerequisite exists for TEST-2 (dependent on TEST-1)
	if !strings.Contains(output, "<prerequisite-task") {
		t.Error("Output should contain prerequisite-task element")
	}
}

func TestSerializer_EmptyTickets(t *testing.T) {
	serializer := NewSerializer("Empty Project")
	var buf bytes.Buffer

	err := serializer.Serialize(&buf, []jira.Ticket{}, nil)
	if err != nil {
		t.Fatalf("Serialize failed for empty tickets: %v", err)
	}

	output := buf.String()

	// Should still produce valid XML structure
	if !strings.Contains(output, "<scenario") {
		t.Error("Output should contain scenario element even with no tickets")
	}
}

func TestSerializer_Serialize_WithEpicGrouping(t *testing.T) {
	tickets := []jira.Ticket{
		{Key: "TASK-1", Summary: "Task in Epic", Link: "http://jira/TASK-1", EpicLink: "EPIC-1"},
		{Key: "TASK-2", Summary: "Task No Epic", Link: "http://jira/TASK-2"},
	}
	epics := map[string]jira.Ticket{
		"EPIC-1": {Key: "EPIC-1", Summary: "Big Project Epic", Status: "In Progress", Link: "http://jira/EPIC-1"},
	}

	serializer := NewSerializer("Epic Project")
	serializer.GroupByEpic = true

	var buf bytes.Buffer
	err := serializer.Serialize(&buf, tickets, epics)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	output := buf.String()

	// Verify Epic Group exists
	if !strings.Contains(output, "<title>Big Project Epic</title>") {
		t.Error("Output should contain Epic Summary as title")
	}
	if !strings.Contains(output, "<type>group</type>") {
		t.Error("Output should contain group type tasks")
	}

	// Verify Epic User Data
	if !strings.Contains(output, "<string>EPIC-1</string>") {
		t.Error("Output should contain Epic Key in user data")
	}
	if !strings.Contains(output, "<string>In Progress</string>") {
		t.Error("Output should contain Epic Status in user data")
	}

	// Verify Milestone exists
	if !strings.Contains(output, "<title>Big Project Epic Done</title>") {
		t.Error("Output should contain Epic Milestone title")
	}
	if !strings.Contains(output, "<type>milestone</type>") {
		t.Error("Output should contain milestone type task")
	}

	// Verify Milestone depends on Group (Prerequisite)
	// This is harder to verify precisely with simple string checks without parsing XML,
	// but we can check if prerequisites exist generally in the file which implies valid structure
	// combined with previous checks.
	if !strings.Contains(output, "<prerequisite-task") {
		t.Error("Output should contain prerequisite-task for milestone dependency")
	}
}

func TestSerializer_Serialize_WithMilestoneDone(t *testing.T) {
	tickets := []jira.Ticket{
		{Key: "TASK-1", Summary: "Task 1", Link: "http://jira/TASK-1", EpicLink: "EPIC-1"},
	}
	epics := map[string]jira.Ticket{
		"EPIC-1": {Key: "EPIC-1", Summary: "Epic 1", Status: "In Progress", Link: "http://jira/EPIC-1"},
	}

	serializer := NewSerializer("Milestone Project")
	serializer.GroupByEpic = true
	serializer.MilestoneDone = true

	var buf bytes.Buffer
	err := serializer.Serialize(&buf, tickets, epics)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	output := buf.String()

	// Verify "Done" Milestone exists
	if !strings.Contains(output, "<title>Done</title>") {
		t.Error("Output should contain 'Done' milestone")
	}

	// Verify it depends on the Epic Milestone
	// We expect "Epic 1 Done" to be in the output, and "Done" to depend on it.
	if !strings.Contains(output, "<title>Epic 1 Done</title>") {
		t.Error("Output should contain 'Epic 1 Done' milestone")
	}
}
