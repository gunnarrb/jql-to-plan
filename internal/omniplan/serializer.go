package omniplan

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/gunnarrb/jql-to-plan/internal/jira"
)

// Serializer converts Jira tickets to OmniPlan XML
type Serializer struct {
	ProjectName   string
	GroupByEpic   bool
	MilestoneDone bool
}

// NewSerializer creates a new OmniPlan serializer
func NewSerializer(projectName string) *Serializer {
	return &Serializer{
		ProjectName: projectName,
	}
}

// Serialize writes Jira tickets as OmniPlan XML to the given writer
func (s *Serializer) Serialize(w io.Writer, tickets []jira.Ticket, epics map[string]jira.Ticket) error {
	scenario := s.buildScenario(tickets, epics)

	// Write XML header
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("writing XML header: %w", err)
	}

	// Encode the scenario
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(scenario); err != nil {
		return fmt.Errorf("encoding scenario: %w", err)
	}

	// Ensure we end with a newline
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("writing final newline: %w", err)
	}

	return nil
}

// buildScenario constructs the full OmniPlan scenario from tickets
func (s *Serializer) buildScenario(tickets []jira.Ticket, epics map[string]jira.Ticket) *Scenario {
	scenarioID := GenerateID()
	topResourceID := "r-1"
	topTaskID := "t-1"

	// Collect unique assignees and create resource IDs for them
	assigneeToResourceID := make(map[string]string)
	var staffResources []Resource
	var childResourceRefs []Reference

	for _, ticket := range tickets {
		if ticket.Assignee != "" {
			if _, exists := assigneeToResourceID[ticket.Assignee]; !exists {
				resourceID := fmt.Sprintf("r%d", idCounter.Add(1))
				assigneeToResourceID[ticket.Assignee] = resourceID
				staffResources = append(staffResources, Resource{
					ID:   resourceID,
					Name: ticket.Assignee,
					Type: "Staff",
				})
				childResourceRefs = append(childResourceRefs, Reference{IDRef: resourceID})
			}
		}
	}

	// Build tasks from tickets, passing the assignee map
	tasks, childTaskRefs := s.buildTasksFromTickets(tickets, assigneeToResourceID, epics)

	// Create the top-level group task that contains all child tasks
	topTask := Task{
		ID:          topTaskID,
		Type:        "group",
		Recalculate: "duration",
		StaticCost:  0,
		ChildTasks:  childTaskRefs,
	}

	// Prepend the top task to the task list
	allTasks := append([]Task{topTask}, tasks...)

	// Create the project resource with staff as children
	projectResource := Resource{
		ID:             topResourceID,
		Name:           s.ProjectName,
		Type:           "Project",
		ChildResources: childResourceRefs,
	}

	// Combine project resource with staff resources
	allResources := append([]Resource{projectResource}, staffResources...)

	return &Scenario{
		XMLNS:       Namespace,
		OPNS:        Namespace,
		ID:          scenarioID,
		Granularity: "days",
		TopResource: Reference{IDRef: topResourceID},
		Resources:   allResources,
		TopTask:     Reference{IDRef: topTaskID},
		Tasks:       allTasks,
		CriticalPaths: []CriticalPath{
			{
				Root:      "-1",
				Enabled:   "false",
				Resources: "false",
				Color: &Color{
					Space: "srgb",
					R:     1,
					G:     0.5,
					B:     0.5,
				},
			},
		},
	}
}

// buildTasksFromTickets converts Jira tickets to OmniPlan tasks
func (s *Serializer) buildTasksFromTickets(tickets []jira.Ticket, assigneeToResourceID map[string]string, epics map[string]jira.Ticket) ([]Task, []Reference) {
	var tasks []Task
	var refs []Reference

	// First pass: Create tasks and build a map of Jira Key -> Task ID
	jiraKeyToTaskID := make(map[string]string)
	var taskPtrs []*Task

	// If GroupByEpic is enabled, we need to organize tasks by Epic
	// We'll use a map of EpicKey -> List of TaskIDs (child refs)
	epicToChildRefs := make(map[string][]Reference)
	// And a map to keep track of created Epic Group Tasks
	epicTasks := make(map[string]*Task)
	epicMilestones := make(map[string]*Task)

	for _, ticket := range tickets {
		taskID := fmt.Sprintf("t%d", idCounter.Add(1))
		jiraKeyToTaskID[ticket.Key] = taskID

		// Calculate effort in seconds: days * 8 hours/day * 3600 seconds/hour
		// Default to 8 hours (1 day) if no effort specified
		effort := int64(28800) // Default 8 hours
		if ticket.EffortDays > 0 {
			effort = int64(ticket.EffortDays * 8 * 3600)
		}

		task := &Task{
			ID:          taskID,
			Title:       ticket.Summary,
			Effort:      effort,
			Recalculate: "duration",
			StaticCost:  0,
			UserData: &UserData{
				Items: []UserDataItem{
					{Key: "Jira Key", Value: ticket.Key},
					{Key: "Jira Link", Value: ticket.Link},
					{Key: "Jira Status", Value: ticket.Status},
				},
			},
		}

		// Assign the task to the resource if there's an assignee
		if ticket.Assignee != "" {
			if resourceID, exists := assigneeToResourceID[ticket.Assignee]; exists {
				task.Assignments = []Reference{{IDRef: resourceID}}
			}
		}

		taskPtrs = append(taskPtrs, task)

		// If GroupByEpic is on and ticket has an epic link, add to epic group
		if s.GroupByEpic && ticket.EpicLink != "" {
			epicToChildRefs[ticket.EpicLink] = append(epicToChildRefs[ticket.EpicLink], Reference{IDRef: taskID})
		} else {
			// Otherwise add to top level
			refs = append(refs, Reference{IDRef: taskID})
		}
	}

	// Create Epic Groups and Milestones if needed
	if s.GroupByEpic {
		for epicKey, children := range epicToChildRefs {
			// Retrieve epic details
			epicSummary := epicKey
			epicLink := ""
			epicStatus := ""
			if epicTicket, ok := epics[epicKey]; ok {
				epicSummary = epicTicket.Summary
				epicLink = epicTicket.Link
				epicStatus = epicTicket.Status
			}

			// Create Group Task for Epic
			groupID := fmt.Sprintf("t%d", idCounter.Add(1))
			groupTask := &Task{
				ID:          groupID,
				Title:       epicSummary,
				Type:        "group",
				Recalculate: "duration",
				StaticCost:  0,
				ChildTasks:  children,
				UserData: &UserData{
					Items: []UserDataItem{
						{Key: "Jira Key", Value: epicKey},
						{Key: "Jira Link", Value: epicLink},
						{Key: "Jira Status", Value: epicStatus},
					},
				},
			}
			epicTasks[epicKey] = groupTask
			tasks = append(tasks, *groupTask)
			refs = append(refs, Reference{IDRef: groupID})

			// Create Milestone for Epic
			milestoneID := fmt.Sprintf("t%d", idCounter.Add(1))
			milestoneTask := &Task{
				ID:          milestoneID,
				Title:       fmt.Sprintf("%s Done", epicSummary),
				Type:        "milestone",
				Recalculate: "duration",
				StaticCost:  0,
				Prerequisites: []PrerequisiteTask{
					{IDRef: groupID},
				},
			}
			epicMilestones[epicKey] = milestoneTask
			tasks = append(tasks, *milestoneTask)
			refs = append(refs, Reference{IDRef: milestoneID})
		}
	}

	// Create "Done" Milestone if requested
	if s.MilestoneDone {
		// Collect prerequisites for the Done milestone
		var donePrereqs []PrerequisiteTask

		// If GroupByEpic is active, depend on Epic milestones
		if s.GroupByEpic {
			for _, milestoneTask := range epicMilestones {
				donePrereqs = append(donePrereqs, PrerequisiteTask{
					IDRef: milestoneTask.ID,
				})
			}
		} else {
			// If not grouping by epic, maybe it should depend on all top level tasks?
			// For now, let's just make it depend on any other milestones we find, or all tasks if no milestones.
			// But the requirement says "has all other milestones in the plan as prerequisites".
			// Let's iterate over ALL tasks and find milestones.
			for _, t := range tasks {
				if t.Type == "milestone" {
					donePrereqs = append(donePrereqs, PrerequisiteTask{
						IDRef: t.ID,
					})
				}
			}
		}

		if len(donePrereqs) > 0 {
			milestoneID := fmt.Sprintf("t%d", idCounter.Add(1))
			doneTask := &Task{
				ID:            milestoneID,
				Title:         "Done",
				Type:          "milestone",
				Recalculate:   "duration",
				StaticCost:    0,
				Prerequisites: donePrereqs,
			}
			tasks = append(tasks, *doneTask)
			refs = append(refs, Reference{IDRef: milestoneID})
		}
	}

	// Second pass: Resolve dependencies
	for i, ticket := range tickets {
		if len(ticket.DependencyKeys) == 0 {
			continue
		}

		for _, depKey := range ticket.DependencyKeys {
			if depID, ok := jiraKeyToTaskID[depKey]; ok {
				taskPtrs[i].Prerequisites = append(taskPtrs[i].Prerequisites, PrerequisiteTask{
					IDRef: depID,
				})
			} else {
				fmt.Printf("Warning: Ticket %s depends on %s, but %s was not found in the JQL result set.\n", ticket.Key, depKey, depKey)
			}
		}
	}

	// Convert *Task back to Task struct values
	for _, t := range taskPtrs {
		tasks = append(tasks, *t)
	}

	return tasks, refs
}

// SerializeToString is a convenience method that serializes to a string
func (s *Serializer) SerializeToString(tickets []jira.Ticket) (string, error) {
	var buf []byte
	w := &byteSliceWriter{buf: &buf}

	if err := s.Serialize(w, tickets, nil); err != nil {
		return "", err
	}

	return string(buf), nil
}

// byteSliceWriter is a simple io.Writer that writes to a byte slice
type byteSliceWriter struct {
	buf *[]byte
}

func (w *byteSliceWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
